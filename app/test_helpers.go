package app

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	bam "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// SetupOptions defines arguments that are passed into `TacChainApp` constructor.
type SetupOptions struct {
	Logger  log.Logger
	DB      *dbm.MemDB
	AppOpts servertypes.AppOptions
}

// NewTacChainAppWithCustomOptions initializes a new TacChainApp with custom options.
func NewTacChainAppWithCustomOptions(t *testing.T, isCheckTx bool, invCheckPeriod uint, options SetupOptions) *TacChainApp {
	t.Helper()

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
		invCheckPeriod,
		options.AppOpts,
		DefaultEVMChainID,
		SetupEvmConfig,
		bam.SetChainID(DefaultChainID),
	)
	genesisState := app.DefaultGenesis()
	genesisState, err = simtestutil.GenesisStateWithValSet(app.AppCodec(), genesisState, valSet, []authtypes.GenesisAccount{acc}, balance)
	require.NoError(t, err)

	if !isCheckTx {
		// init chain must be called to stop deliverState from being nil
		stateBytes, err := cmtjson.MarshalIndent(genesisState, "", " ")
		require.NoError(t, err)

		// Initialize the chain
		_, err = app.InitChain(&abci.RequestInitChain{
			ChainId:         DefaultChainID,
			Validators:      []abci.ValidatorUpdate{},
			ConsensusParams: simtestutil.DefaultConsensusParams,
			AppStateBytes:   stateBytes,
		})
		require.NoError(t, err)
	}

	return app
}

// GetEVMChainID extracts the EVM chain ID from a given string.
// If the string is in format "tacchain_2391-1", it extracts the numeric part.
// If the string is a simple numeric value, it parses it directly.
func GetEVMChainID(chainID string) (uint64, error) {
	match := regexp.MustCompile(`^[a-zA-Z]+_(\d+)-\d+$`).FindStringSubmatch(chainID)

	// If the chain ID is in the format "tacchain_2391-1", we extract the numeric part.
	if len(match) == 2 {
		res, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid chain ID format: %s", chainID)
		}

		return res, nil
	}

	// Otherwise, we assume it's a simple numeric value.
	res, err := strconv.ParseUint(chainID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid chain ID format: %s", chainID)
	}

	return res, nil
}

func NoOpEvmAppOptions(_ uint64) error {
	return nil
}
