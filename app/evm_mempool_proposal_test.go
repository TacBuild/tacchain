package app

import (
	"math/big"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func TestEVMMempoolProcessProposal(t *testing.T) {
	tacApp := NewTacChainAppWithCustomOptions(t, false, SetupOptions{
		Logger:  log.NewNopLogger(),
		DB:      dbm.NewMemDB(),
		AppOpts: simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
	})
	t.Cleanup(func() {
		require.NoError(t, tacApp.EVMMempool.Close())
		require.NoError(t, tacApp.Close())
	})

	key, err := ethcrypto.GenerateKey()
	require.NoError(t, err)
	sender := sdk.AccAddress(ethcrypto.PubkeyToAddress(key.PublicKey).Bytes())
	ctx := tacApp.NewContext(false)
	coins := sdk.NewCoins(sdk.NewCoin(evmtypes.GetEVMCoinDenom(), sdkmath.NewInt(1e18)))
	require.NoError(t, tacApp.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins))
	require.NoError(t, tacApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, coins))

	blockTime := time.Unix(1, 0).UTC()
	_, err = tacApp.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 1, Time: blockTime})
	require.NoError(t, err)
	_, err = tacApp.Commit()
	require.NoError(t, err)

	ctx, err = tacApp.CreateQueryContext(1, false)
	require.NoError(t, err)
	gasPrice := big.NewInt(1)
	if baseFee := tacApp.EVMKeeper.GetBaseFee(ctx); baseFee != nil {
		gasPrice.Add(gasPrice, baseFee)
	}
	signer := ethtypes.LatestSigner(evmtypes.GetEthChainConfig())
	recipient := ethcrypto.PubkeyToAddress(key.PublicKey)
	encodeEthereumTx := func(nonce uint64) []byte {
		ethTx, err := ethtypes.SignNewTx(key, signer, &ethtypes.LegacyTx{
			Nonce:    nonce,
			To:       &recipient,
			Value:    big.NewInt(1000),
			Gas:      100_000,
			GasPrice: gasPrice,
		})
		require.NoError(t, err)
		msg := &evmtypes.MsgEthereumTx{}
		require.NoError(t, msg.FromSignedEthereumTx(ethTx, signer))
		tx, err := msg.BuildTx(tacApp.txConfig.NewTxBuilder(), evmtypes.GetEVMCoinDenom())
		require.NoError(t, err)
		txBytes, err := tacApp.txConfig.TxEncoder()(tx)
		require.NoError(t, err)
		return txBytes
	}

	testCases := []struct {
		name      string
		txs       [][]byte
		expStatus abci.ResponseProcessProposal_ProposalStatus
	}{
		{
			name:      "accept empty proposal",
			expStatus: abci.ResponseProcessProposal_ACCEPT,
		},
		{
			name:      "accept valid Ethereum transaction",
			txs:       [][]byte{encodeEthereumTx(0)},
			expStatus: abci.ResponseProcessProposal_ACCEPT,
		},
		{
			name:      "reject Ethereum nonce gap",
			txs:       [][]byte{encodeEthereumTx(1)},
			expStatus: abci.ResponseProcessProposal_REJECT,
		},
		{
			name:      "reject malformed transaction bytes",
			txs:       [][]byte{{0xff}},
			expStatus: abci.ResponseProcessProposal_REJECT,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := tacApp.ProcessProposal(&abci.RequestProcessProposal{
				Height: 2,
				Time:   blockTime.Add(time.Second),
				Txs:    tc.txs,
			})
			require.NoError(t, err)
			require.Equal(t, tc.expStatus, response.Status)
		})
	}
}
