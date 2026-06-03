package app

import (
	"context"
	"math/big"
	"testing"

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

	calls int
	tx    cmttypes.Tx
}

func (r *broadcastRecorder) BroadcastTxSync(_ context.Context, tx cmttypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	r.calls++
	r.tx = append(r.tx[:0], tx...)
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
	require.Equal(t, 1, rpcClient.calls)
	require.NotEmpty(t, rpcClient.tx)

	decodedTx, err := tacApp.txConfig.TxDecoder()(rpcClient.tx)
	require.NoError(t, err)

	msgs := decodedTx.GetMsgs()
	require.Len(t, msgs, 1)

	msg, ok := msgs[0].(*evmtypes.MsgEthereumTx)
	require.True(t, ok)
	require.Equal(t, ethTx.Hash(), msg.Hash())
}
