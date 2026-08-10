package app

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"

	rpcmock "github.com/cometbft/cometbft/rpc/client/mock"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/client"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// signedEthTx builds a transaction the broadcast path can recover a sender from.
// An unsigned one is not usable here: MsgEthereumTx.ValidateBasic rejects a
// message without a sender, so the broadcast has to derive it from the signature.
func signedEthTx(t *testing.T) (*ethtypes.Transaction, ethcmn.Address) {
	t.Helper()

	key, err := ethcrypto.GenerateKey()
	require.NoError(t, err)

	to := ethcmn.Address{}
	tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSigner(evmtypes.GetEthChainConfig()), &ethtypes.LegacyTx{
		Nonce:    1,
		To:       &to,
		Value:    big.NewInt(0),
		Gas:      21_000,
		GasPrice: big.NewInt(1),
	})
	require.NoError(t, err)

	return tx, ethcrypto.PubkeyToAddress(key.PublicKey)
}

type broadcastRecorder struct {
	rpcmock.Client

	// code the node answers the broadcast with; zero means accepted
	code uint32

	mu    sync.Mutex
	calls int
	tx    cmttypes.Tx
}

func (r *broadcastRecorder) BroadcastTxSync(_ context.Context, tx cmttypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls++
	r.tx = append(r.tx[:0], tx...)
	return &coretypes.ResultBroadcastTx{Code: r.code}, nil
}

func (r *broadcastRecorder) setCode(code uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.code = code
	r.calls = 0
}

func (r *broadcastRecorder) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *broadcastRecorder) txBytes() cmttypes.Tx {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append(cmttypes.Tx(nil), r.tx...)
}

type blockingBroadcastClient struct {
	rpcmock.Client

	started chan struct{}
	release chan struct{}
	done    chan struct{}
}

func (c *blockingBroadcastClient) BroadcastTxSync(context.Context, cmttypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	close(c.started)
	<-c.release
	close(c.done)
	return &coretypes.ResultBroadcastTx{}, nil
}

func TestEVMMempoolBroadcastTxFnUsesUpdatedClientCtx(t *testing.T) {
	tacApp := NewTacChainAppWithCustomOptions(t, true, SetupOptions{
		Logger:  log.NewTestLogger(t),
		DB:      dbm.NewMemDB(),
		AppOpts: simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
	})

	txPool := tacApp.EVMMempool.GetTxPool()
	require.Len(t, txPool.Subpools, 1)

	legacyPool, ok := txPool.Subpools[0].(*legacypool.LegacyPool)
	require.True(t, ok)
	require.NotNil(t, legacyPool.BroadcastTxFn)

	rpcClient := &broadcastRecorder{}
	tacApp.RegisterTxService(client.Context{}.
		WithTxConfig(tacApp.txConfig).
		WithClient(rpcClient),
	)

	ethTx, sender := signedEthTx(t)

	// Before the explicit BroadCastTxFn override, this callback captured the
	// empty client.Context from app construction and returned "no RPC client is
	// defined in offline mode". RegisterTxService is where evmserver passes the
	// real clientCtx after local.New(bftNode), so it must refresh app.clientCtx.
	require.NoError(t, legacyPool.BroadcastTxFn([]*ethtypes.Transaction{ethTx}))
	require.Eventually(t, func() bool {
		return rpcClient.callCount() == 1
	}, time.Second, 10*time.Millisecond)

	txBytes := rpcClient.txBytes()
	require.NotEmpty(t, txBytes)

	decodedTx, err := tacApp.txConfig.TxDecoder()(txBytes)
	require.NoError(t, err)

	msgs := decodedTx.GetMsgs()
	require.Len(t, msgs, 1)

	msg, ok := msgs[0].(*evmtypes.MsgEthereumTx)
	require.True(t, ok)
	require.Equal(t, ethTx.Hash(), msg.Hash())

	// Without the sender the receiving mempool refuses the message with
	// "sender address is missing", so the transaction never reaches its peers.
	require.Equal(t, sender.Bytes(), []byte(msg.From),
		"broadcast message must carry the recovered sender")
	require.NoError(t, msg.ValidateBasic())

	// The ante handler routes a transaction to the EVM path by this extension
	// option and nothing else, so a broadcast without it is refused before the
	// message is even looked at.
	extTx, ok := decodedTx.(authante.HasExtensionOptionsTx)
	require.True(t, ok)
	opts := extTx.GetExtensionOptions()
	require.Len(t, opts, 1, "broadcast tx must carry the ethereum extension option")
	require.Equal(t, "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx", opts[0].GetTypeUrl())

	// Fee and gas live on the ethereum transaction and have to be carried over.
	feeTx, ok := decodedTx.(sdk.FeeTx)
	require.True(t, ok)
	require.Equal(t, ethTx.Gas(), feeTx.GetGas())
	require.False(t, feeTx.GetFee().IsZero(), "broadcast tx must carry a fee")
}

func TestEVMMempoolBroadcastTxFnDoesNotBlockOnBroadcast(t *testing.T) {
	tacApp := NewTacChainAppWithCustomOptions(t, true, SetupOptions{
		Logger:  log.NewTestLogger(t),
		DB:      dbm.NewMemDB(),
		AppOpts: simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
	})

	txPool := tacApp.EVMMempool.GetTxPool()
	require.Len(t, txPool.Subpools, 1)

	legacyPool, ok := txPool.Subpools[0].(*legacypool.LegacyPool)
	require.True(t, ok)
	require.NotNil(t, legacyPool.BroadcastTxFn)

	rpcClient := &blockingBroadcastClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}

	tacApp.RegisterTxService(client.Context{}.
		WithTxConfig(tacApp.txConfig).
		WithClient(rpcClient),
	)

	ethTx, _ := signedEthTx(t)

	done := make(chan error, 1)
	go func() {
		done <- legacyPool.BroadcastTxFn([]*ethtypes.Transaction{ethTx})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("BroadcastTxFn blocked on BroadcastTxSync")
	}

	select {
	case <-rpcClient.started:
	case <-time.After(time.Second):
		t.Fatal("BroadcastTxFn did not start background broadcast")
	}

	close(rpcClient.release)
	select {
	case <-rpcClient.done:
	case <-time.After(time.Second):
		t.Fatal("background broadcast goroutine did not exit")
	}
}

func TestBroadcastEVMTransactionsMempoolCodes(t *testing.T) {
	tacApp := NewTacChainAppWithCustomOptions(t, true, SetupOptions{
		Logger:  log.NewTestLogger(t),
		DB:      dbm.NewMemDB(),
		AppOpts: simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
	})

	ethTx, _ := signedEthTx(t)

	// RegisterTxService may only run once per app, so the same client is reused
	// and its answer changed per case.
	rpcClient := &broadcastRecorder{}
	tacApp.RegisterTxService(client.Context{}.
		WithTxConfig(tacApp.txConfig).
		WithClient(rpcClient),
	)

	for _, tc := range []struct {
		name    string
		code    uint32
		wantErr bool
	}{
		// The submitting path has already given this transaction to Comet, so
		// the node's own cache answers the peer broadcast as a duplicate. That
		// is the normal outcome, not something to report.
		{"duplicate in mempool cache", sdkerrors.ErrTxInMempoolCache.ABCICode(), false},
		{"accepted", 0, false},
		{"rejected", sdkerrors.ErrInvalidRequest.ABCICode(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rpcClient.setCode(tc.code)

			err := tacApp.broadcastEVMTransactions([]*ethtypes.Transaction{ethTx})
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, 1, rpcClient.callCount())
		})
	}
}
