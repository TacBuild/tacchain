package v160

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	storetypes "cosmossdk.io/store/types"

	"github.com/TacBuild/tacchain/app/upgrades"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttestutil "github.com/cosmos/cosmos-sdk/x/mint/testutil"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

func newMintTestKeeper(t *testing.T) (sdk.Context, *mintkeeper.Keeper) {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(minttypes.StoreKey)
	storeService := runtime.NewKVStoreService(key)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx

	ctrl := gomock.NewController(t)
	accountKeeper := minttestutil.NewMockAccountKeeper(ctrl)
	bankKeeper := minttestutil.NewMockBankKeeper(ctrl)
	stakingKeeper := minttestutil.NewMockStakingKeeper(ctrl)
	accountKeeper.EXPECT().GetModuleAddress(minttypes.ModuleName).
		Return(authtypes.NewModuleAddress(minttypes.ModuleName))

	k := mintkeeper.NewKeeper(
		encCfg.Codec,
		storeService,
		stakingKeeper,
		accountKeeper,
		bankKeeper,
		authtypes.FeeCollectorName,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	return ctx, &k
}

func TestMigrateMintBlocksPerYear(t *testing.T) {
	ctx, k := newMintTestKeeper(t)

	// Seed mainnet-like params with the legacy value that assumed a 2.0s block.
	params := minttypes.DefaultParams()
	params.MintDenom = "utac"
	params.BlocksPerYear = 15_768_000
	require.NoError(t, k.Params.Set(ctx, params))

	ak := &upgrades.AppKeepers{MintKeeper: k}
	require.NoError(t, migrateMintBlocksPerYear(ctx, ak))

	got, err := k.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(20_300_000), got.BlocksPerYear)
	require.Equal(t, TargetBlocksPerYear, got.BlocksPerYear)

	// Every other parameter must be left untouched (inflation curve preserved).
	require.Equal(t, params.MintDenom, got.MintDenom)
	require.True(t, params.InflationMax.Equal(got.InflationMax))
	require.True(t, params.InflationMin.Equal(got.InflationMin))
	require.True(t, params.InflationRateChange.Equal(got.InflationRateChange))
	require.True(t, params.GoalBonded.Equal(got.GoalBonded))

	// Idempotent: a second run is a no-op and stays at target.
	require.NoError(t, migrateMintBlocksPerYear(ctx, ak))
	got2, err := k.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, TargetBlocksPerYear, got2.BlocksPerYear)
}

func TestMigrateMintBlocksPerYearNilKeeper(t *testing.T) {
	err := migrateMintBlocksPerYear(sdk.Context{}, &upgrades.AppKeepers{})
	require.Error(t, err)
}
