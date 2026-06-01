package v160

import (
	"bytes"
	"testing"
	"time"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/TacBuild/tacchain/app/upgrades"
	"github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtestutil "github.com/cosmos/cosmos-sdk/x/staking/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestRebuildStakingLSMAccountingUsesSlashedExchangeRate(t *testing.T) {
	ctx, keeper := newLSMAccountingTestKeeper(t)
	valCodec := address.NewBech32Codec("tacvaloper")

	valAddr := bytes.Repeat([]byte{0x01}, 20)
	valAddrStr, err := valCodec.BytesToString(valAddr)
	require.NoError(t, err)

	validator := stakingtypes.Validator{
		OperatorAddress:     valAddrStr,
		Status:              stakingtypes.Bonded,
		Tokens:              math.NewInt(900),
		DelegatorShares:     math.LegacyNewDec(1000),
		LiquidShares:        math.LegacyNewDec(999),
		ValidatorBondShares: math.LegacyNewDec(888),
		Description:         stakingtypes.Description{Moniker: "slashed-validator"},
		MinSelfDelegation:   math.OneInt(),
	}
	require.NoError(t, keeper.SetValidator(ctx, validator))

	liquidDelegator := lsmTestAccAddr(t, 0x02, 32)
	normalDelegator := lsmTestAccAddr(t, 0x03, 20)
	validatorBondDelegator := lsmTestAccAddr(t, 0x04, 20)

	liquidShares := math.LegacyNewDec(100)
	require.NoError(t, keeper.SetDelegation(ctx, stakingtypes.NewDelegation(
		liquidDelegator,
		valAddrStr,
		liquidShares,
	)))
	require.NoError(t, keeper.SetDelegation(ctx, stakingtypes.NewDelegation(
		normalDelegator,
		valAddrStr,
		math.LegacyNewDec(300),
	)))
	validatorBondDelegation := stakingtypes.NewDelegation(
		validatorBondDelegator,
		valAddrStr,
		math.LegacyNewDec(70),
	)
	validatorBondDelegation.ValidatorBond = true
	require.NoError(t, keeper.SetDelegation(ctx, validatorBondDelegation))

	keeper.SetTotalLiquidStakedTokens(ctx, math.NewInt(123456789))

	err = rebuildStakingLSMAccounting(ctx, &upgrades.AppKeepers{StakingKeeper: keeper})
	require.NoError(t, err)

	updatedValidator, err := keeper.GetValidator(ctx, valAddr)
	require.NoError(t, err)

	require.True(t, updatedValidator.LiquidShares.Equal(liquidShares))
	require.True(t, updatedValidator.ValidatorBondShares.Equal(math.LegacyNewDec(70)))

	expectedLiquidTokens := validator.TokensFromShares(liquidShares).TruncateInt()
	require.Equal(t, math.NewInt(90), expectedLiquidTokens)
	require.Equal(t, expectedLiquidTokens, keeper.GetTotalLiquidStakedTokens(ctx))
}

func newLSMAccountingTestKeeper(t *testing.T) (sdk.Context, *stakingkeeper.Keeper) {
	t.Helper()

	key := storetypes.NewKVStoreKey(stakingtypes.StoreKey)
	storeService := runtime.NewKVStoreService(key)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx.WithBlockHeader(cmtproto.Header{Time: time.Now().UTC()})

	encCfg := moduletestutil.MakeTestEncodingConfig()
	ctrl := gomock.NewController(t)

	accountKeeper := stakingtestutil.NewMockAccountKeeper(ctrl)
	accountKeeper.EXPECT().AddressCodec().Return(address.NewBech32Codec("tac")).AnyTimes()
	accountKeeper.EXPECT().
		GetModuleAddress(stakingtypes.BondedPoolName).
		Return(authtypes.NewModuleAddress(stakingtypes.BondedPoolName)).
		AnyTimes()
	accountKeeper.EXPECT().
		GetModuleAddress(stakingtypes.NotBondedPoolName).
		Return(authtypes.NewModuleAddress(stakingtypes.NotBondedPoolName)).
		AnyTimes()

	bankKeeper := stakingtestutil.NewMockBankKeeper(ctrl)
	keeper := stakingkeeper.NewKeeper(
		encCfg.Codec,
		storeService,
		accountKeeper,
		bankKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		address.NewBech32Codec("tacvaloper"),
		address.NewBech32Codec("tacvalcons"),
	)
	require.NoError(t, keeper.SetParams(ctx, stakingtypes.DefaultParams()))

	return ctx, keeper
}

func lsmTestAccAddr(t *testing.T, fill byte, length int) string {
	t.Helper()

	accAddr, err := address.NewBech32Codec("tac").BytesToString(bytes.Repeat([]byte{fill}, length))
	require.NoError(t, err)
	return accAddr
}
