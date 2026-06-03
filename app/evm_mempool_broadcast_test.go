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
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type broadcastRecorder struct {
	rpcmock.Client

	mu    sync.Mutex
	calls int
	tx    cmttypes.Tx
}

func (r *broadcastRecorder) BroadcastTxSync(_ context.Context, tx cmttypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls++
	r.tx = append(r.tx[:0], tx...)
	return &coretypes.ResultBroadcastTx{}, nil
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

	to := ethcmn.Address{}
	ethTx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    1,
		To:       &to,
		Value:    big.NewInt(0),
		Gas:      21_000,
		GasPrice: big.NewInt(1),
	})

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

	to := ethcmn.Address{}
	ethTx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    1,
		To:       &to,
		Value:    big.NewInt(0),
		Gas:      21_000,
		GasPrice: big.NewInt(1),
	})

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
