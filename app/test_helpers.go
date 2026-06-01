package app

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	appconfig "github.com/TacBuild/tacchain/app/config"
	bam "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	evmsrvflags "github.com/cosmos/evm/server/flags"
	"github.com/spf13/cast"
)

// SetupOptions defines arguments that are passed into `TacChainApp` constructor.
type SetupOptions struct {
	Logger  log.Logger
	DB      *dbm.MemDB
	AppOpts servertypes.AppOptions
}

// NewTacChainAppWithCustomOptions initializes a new TacChainApp with custom options.
func NewTacChainAppWithCustomOptions(t *testing.T, isCheckTx bool, options SetupOptions) *TacChainApp {
	t.Helper()
	appconfig.SetupSDKConfig()
	evmChainID := cast.ToUint64(options.AppOpts.Get(evmsrvflags.EVMChainID))
	resetEVMTestConfig(t, evmChainID)
	t.Cleanup(func() {
		resetEVMTestConfig(t, evmChainID)
	})

	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)
	// create validator set with single validator
	validator := cmttypes.NewValidator(pubKey, 1)
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{validator})

	// generate genesis account
	senderPrivKey := secp256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100000000000000))),
	}

	app := NewTacChainApp(
		options.Logger,
		options.DB,
		nil,
		true,
		options.AppOpts,
		bam.SetChainID(appconfig.DefaultChainID),
	)
	genesisState := app.DefaultGenesis()
	genesisState, err = simtestutil.GenesisStateWithValSet(app.AppCodec(), genesisState, valSet, []authtypes.GenesisAccount{acc}, balance)
	require.NoError(t, err)

	// GenesisStateWithValSet overwrites bank genesis without denom metadata.
	// Restore it so that InitEvmCoinInfo can find the EVM denom metadata.
	var bankGenState banktypes.GenesisState
	app.AppCodec().MustUnmarshalJSON(genesisState[banktypes.ModuleName], &bankGenState)
	bankGenState.DenomMetadata = append(bankGenState.DenomMetadata, appconfig.DefaultBankDenomMetadata()...)
	genesisState[banktypes.ModuleName] = app.AppCodec().MustMarshalJSON(&bankGenState)

	if !isCheckTx {
		resetEVMTestConfig(t, evmChainID)

		// init chain must be called to stop deliverState from being nil
		stateBytes, err := cmtjson.MarshalIndent(genesisState, "", " ")
		require.NoError(t, err)

		// Initialize the chain
		_, err = app.InitChain(&abci.RequestInitChain{
			ChainId:         appconfig.DefaultChainID,
			Validators:      []abci.ValidatorUpdate{},
			ConsensusParams: simtestutil.DefaultConsensusParams,
			AppStateBytes:   stateBytes,
		})
		require.NoError(t, err)
	}

	return app
}
