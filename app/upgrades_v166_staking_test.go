package app

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	appconfig "github.com/TacBuild/tacchain/app/config"
	v166 "github.com/TacBuild/tacchain/app/upgrades/v1.6.6"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestV166AccountStakingMigration(t *testing.T) {
	for _, tc := range []struct {
		name          string
		merge         bool
		rejectRewards bool
		legacyEmpty   bool
		selfBond      bool
	}{
		{name: "new positions"},
		{name: "merge positions", merge: true},
		{name: "empty delegation records", legacyEmpty: true},
		{name: "validator self-delegation", selfBond: true},
		{name: "rewards transfer failure", merge: true, rejectRewards: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, ctx, source, destination := newV166StakingTestApp(t)
			const denom = appconfig.BaseDenom
			blockTime := ctx.BlockTime()
			previousWithdraw := sdk.AccAddress(bytes.Repeat([]byte{9}, 20))

			for i, address := range []sdk.AccAddress{source, destination} {
				fundV166TestAccount(t, app, ctx, address, 101+uint64(i), sdkmath.NewInt(500), sdkmath.NewInt(450))
			}

			sk := app.StakingKeeper
			fromValidator := sdk.ValAddress(bytes.Repeat([]byte{1}, 20))
			if tc.selfBond {
				fromValidator = sdk.ValAddress(source)
			}
			toValidator := sdk.ValAddress(bytes.Repeat([]byte{2}, 20))
			newV166TestValidator(t, app, ctx, fromValidator)
			newV166TestValidator(t, app, ctx, toValidator)
			delegate := func(owner sdk.AccAddress, address sdk.ValAddress, amount int64) {
				delegateV166TestCoins(t, app, ctx, owner, address, sdkmath.NewInt(amount))
			}
			delegate(source, fromValidator, 300)
			delegate(source, toValidator, 100)
			redTime, err := sk.BeginRedelegation(ctx, source, fromValidator, toValidator, sdkmath.LegacyNewDec(40))
			require.NoError(t, err)
			unbondTime, _, err := sk.Undelegate(ctx, source, fromValidator, sdkmath.LegacyNewDec(20))
			require.NoError(t, err)

			sourceUnbonding, err := sk.GetUnbondingDelegation(ctx, source, fromValidator)
			require.NoError(t, err)
			unbondingID := sourceUnbonding.Entries[0].UnbondingId
			require.NoError(t, sk.PutUnbondingOnHold(ctx, unbondingID))
			sourceUnbonding, err = sk.GetUnbondingDelegation(ctx, source, fromValidator)
			require.NoError(t, err)
			sourceRedelegation, err := sk.GetRedelegation(ctx, source, fromValidator, toValidator)
			require.NoError(t, err)
			redelegationID := sourceRedelegation.Entries[0].UnbondingId

			var destinationUnbonding stakingtypes.UnbondingDelegation
			var destinationRedelegation stakingtypes.Redelegation
			var destinationRewards int64
			if tc.merge {
				delegate(destination, fromValidator, 100)
				_, err := sk.BeginRedelegation(ctx, destination, fromValidator, toValidator, sdkmath.LegacyNewDec(15))
				require.NoError(t, err)
				_, _, err = sk.Undelegate(ctx, destination, fromValidator, sdkmath.LegacyNewDec(5))
				require.NoError(t, err)
				destinationUnbonding, err = sk.GetUnbondingDelegation(ctx, destination, fromValidator)
				require.NoError(t, err)
				destinationRedelegation, err = sk.GetRedelegation(ctx, destination, fromValidator, toValidator)
				require.NoError(t, err)
				destinationRewards = 80
			}
			if tc.legacyEmpty {
				require.NoError(t, sk.SetDelegation(ctx, stakingtypes.Delegation{
					DelegatorAddress: source.String(),
					ValidatorAddress: sdk.ValAddress(bytes.Repeat([]byte{3}, 20)).String(),
					Shares:           sdkmath.LegacyZeroDec(),
				}))
				require.NoError(t, sk.SetDelegation(ctx, stakingtypes.Delegation{
					DelegatorAddress: destination.String(),
					ValidatorAddress: toValidator.String(),
					Shares:           sdkmath.LegacyZeroDec(),
				}))
			}

			upgradeHeight := ctx.BlockHeight() + 1
			ctx = ctx.WithBlockHeight(upgradeHeight).
				WithHeaderInfo(header.Info{ChainID: v166TestChainID, Height: upgradeHeight, Time: blockTime})
			validator, err := sk.GetValidator(ctx, fromValidator)
			require.NoError(t, err)
			rewards := sdk.NewCoins(sdk.NewCoin(denom, validator.Tokens))
			require.NoError(t, app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, rewards))
			require.NoError(t, app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, distrtypes.ModuleName, rewards))
			require.NoError(t, app.DistrKeeper.AllocateTokensToValidator(ctx, validator, sdk.NewDecCoinsFromCoins(rewards...)))
			require.NoError(t, app.DistrKeeper.SetDelegatorWithdrawAddr(ctx, source, previousWithdraw))

			bondedPool := sk.GetBondedPool(ctx).GetAddress()
			notBondedPool := sk.GetNotBondedPool(ctx).GetAddress()
			beforeBonded := app.BankKeeper.GetBalance(ctx, bondedPool, denom)
			beforeNotBonded := app.BankKeeper.GetBalance(ctx, notBondedPool, denom)
			beforeSource := app.BankKeeper.GetBalance(ctx, source, denom)
			beforeDestination := app.BankKeeper.GetBalance(ctx, destination, denom)
			destinationBase := app.AccountKeeper.GetAccount(ctx, destination).(*vestingtypes.PeriodicVestingAccount).BaseAccount
			beforeSourceDelegations, err := sk.GetAllDelegatorDelegations(ctx, source)
			require.NoError(t, err)

			var sends int
			if tc.rejectRewards {
				app.BankKeeper.AppendSendRestriction(func(_ context.Context, from, to sdk.AccAddress, _ sdk.Coins) (sdk.AccAddress, error) {
					if from.Equals(source) && to.Equals(destination) {
						sends++
						if sends == 2 {
							return nil, errors.New("rewards transfer rejected")
						}
					}
					return to, nil
				})
			}

			require.NoError(t, app.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: v166.UpgradeName, Height: ctx.BlockHeight()}))
			doneHeight, err := app.UpgradeKeeper.GetDoneHeight(ctx, v166.UpgradeName)
			require.NoError(t, err)
			require.Equal(t, ctx.BlockHeight(), doneHeight)
			require.IsType(t, &authtypes.BaseAccount{}, app.AccountKeeper.GetAccount(ctx, source))
			require.Equal(t, destinationBase, app.AccountKeeper.GetAccount(ctx, destination))
			require.True(t, app.BankKeeper.GetBalance(ctx, source, denom).IsZero())
			require.True(t, app.BankKeeper.GetBalance(ctx, previousWithdraw, denom).IsZero())
			require.Equal(t, beforeBonded, app.BankKeeper.GetBalance(ctx, bondedPool, denom))
			require.Equal(t, beforeNotBonded, app.BankKeeper.GetBalance(ctx, notBondedPool, denom))
			afterWithdraw, err := app.DistrKeeper.GetDelegatorWithdrawAddr(ctx, source)
			require.NoError(t, err)
			require.Equal(t, previousWithdraw, afterWithdraw)
			afterSourceDelegations, err := sk.GetAllDelegatorDelegations(ctx, source)
			require.NoError(t, err)
			afterUnbonding, err := sk.GetUnbondingDelegationByUnbondingID(ctx, unbondingID)
			require.NoError(t, err)
			afterRedelegation, err := sk.GetRedelegationByUnbondingID(ctx, redelegationID)
			require.NoError(t, err)
			hasStartingInfo, err := app.DistrKeeper.HasDelegatorStartingInfo(ctx, fromValidator, source)
			require.NoError(t, err)

			if tc.rejectRewards || tc.selfBond {
				if tc.rejectRewards {
					require.Equal(t, 2, sends)
				}
				require.Equal(t, beforeDestination.Add(beforeSource), app.BankKeeper.GetBalance(ctx, destination, denom))
				require.Len(t, afterSourceDelegations, 1)
				require.Equal(t, fromValidator.String(), afterSourceDelegations[0].ValidatorAddress)
				require.Equal(t, sdkmath.LegacyNewDec(240), afterSourceDelegations[0].Shares)
				require.Equal(t, sourceUnbonding, afterUnbonding)
				require.Equal(t, destination.String(), afterRedelegation.DelegatorAddress)
				require.Equal(t, append(destinationRedelegation.Entries, sourceRedelegation.Entries...), afterRedelegation.Entries)
				moved, err := sk.GetDelegation(ctx, destination, toValidator)
				require.NoError(t, err)
				wantShares := sdkmath.LegacyNewDec(140)
				if tc.merge {
					wantShares = wantShares.Add(sdkmath.LegacyNewDec(15))
				}
				require.Equal(t, wantShares, moved.Shares)
				require.True(t, hasStartingInfo)
				return
			}

			require.Empty(t, afterSourceDelegations)
			require.False(t, hasStartingInfo)
			destinationDelegations, err := sk.GetAllDelegatorDelegations(ctx, destination)
			require.NoError(t, err)
			require.Len(t, destinationDelegations, 2)
			for _, original := range beforeSourceDelegations {
				if original.Shares.IsZero() {
					continue
				}
				address, err := sdk.ValAddressFromBech32(original.ValidatorAddress)
				require.NoError(t, err)
				delegation, err := sk.GetDelegation(ctx, destination, address)
				require.NoError(t, err)
				shares := original.Shares
				if tc.merge {
					if address.Equals(fromValidator) {
						shares = shares.Add(sdkmath.LegacyNewDec(80))
					} else {
						shares = shares.Add(sdkmath.LegacyNewDec(15))
					}
				}
				require.Equal(t, shares, delegation.Shares)
				hasStartingInfo, err := app.DistrKeeper.HasDelegatorStartingInfo(ctx, address, destination)
				require.NoError(t, err)
				require.True(t, hasStartingInfo)
			}

			_, err = sk.GetUnbondingDelegation(ctx, source, fromValidator)
			require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)
			_, err = sk.GetRedelegation(ctx, source, fromValidator, toValidator)
			require.ErrorIs(t, err, stakingtypes.ErrNoRedelegation)
			require.Equal(t, destination.String(), afterUnbonding.DelegatorAddress)
			require.Equal(t, append(destinationUnbonding.Entries, sourceUnbonding.Entries...), afterUnbonding.Entries)
			require.Equal(t, destination.String(), afterRedelegation.DelegatorAddress)
			require.Equal(t, append(destinationRedelegation.Entries, sourceRedelegation.Entries...), afterRedelegation.Entries)
			received := beforeSource.Amount.AddRaw(240 + destinationRewards)
			require.Equal(t, beforeDestination.Amount.Add(received), app.BankKeeper.SpendableCoins(ctx, destination).AmountOf(denom))

			ctx = ctx.WithBlockTime(unbondTime)
			unbondingQueue, err := sk.DequeueAllMatureUBDQueue(ctx, unbondTime)
			require.NoError(t, err)
			require.Equal(t, []stakingtypes.DVPair{{DelegatorAddress: destination.String(), ValidatorAddress: fromValidator.String()}}, unbondingQueue)
			unbonded, err := sk.CompleteUnbonding(ctx, destination, fromValidator)
			require.NoError(t, err)
			unbondedTotal := unbonded.AmountOf(denom).AddRaw(20)
			maturityBalance := app.BankKeeper.GetBalance(ctx, destination, denom)
			require.NoError(t, sk.UnbondingCanComplete(ctx, unbondingID))
			require.Equal(t, maturityBalance.Add(sdk.NewInt64Coin(denom, 20)), app.BankKeeper.GetBalance(ctx, destination, denom))
			require.Equal(t, beforeDestination.Amount.Add(received).Add(unbondedTotal), app.BankKeeper.SpendableCoins(ctx, destination).AmountOf(denom))
			_, err = sk.GetUnbondingDelegationByUnbondingID(ctx, unbondingID)
			require.ErrorIs(t, err, stakingtypes.ErrNoUnbondingDelegation)

			maturityHeader := ctx.HeaderInfo()
			maturityHeader.Time = redTime
			ctx = ctx.WithBlockTime(redTime).WithHeaderInfo(maturityHeader)
			redelegationQueue, err := sk.DequeueAllMatureRedelegationQueue(ctx, redTime)
			require.NoError(t, err)
			require.Equal(t, []stakingtypes.DVVTriplet{{
				DelegatorAddress:    destination.String(),
				ValidatorSrcAddress: fromValidator.String(),
				ValidatorDstAddress: toValidator.String(),
			}}, redelegationQueue)
			_, err = sk.CompleteRedelegation(ctx, destination, fromValidator, toValidator)
			require.NoError(t, err)
			_, err = sk.GetRedelegationByUnbondingID(ctx, redelegationID)
			require.ErrorIs(t, err, stakingtypes.ErrNoRedelegation)
		})
	}
}

func newV166StakingTestApp(t *testing.T) (*TacChainApp, sdk.Context, sdk.AccAddress, sdk.AccAddress) {
	t.Helper()
	app := NewTacChainAppWithCustomOptions(t, false, SetupOptions{
		Logger:  log.NewTestLogger(t),
		DB:      dbm.NewMemDB(),
		AppOpts: simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
	})
	blockTime := time.Unix(10_000, 0).UTC()
	ctx := app.NewContext(false).WithChainID(v166TestChainID).WithBlockHeight(10).WithBlockTime(blockTime).
		WithHeaderInfo(header.Info{ChainID: v166TestChainID, Height: 10, Time: blockTime})
	source, err := sdk.AccAddressFromBech32(v166TestSource)
	require.NoError(t, err)
	destination, err := sdk.AccAddressFromBech32(v166TestDestination)
	require.NoError(t, err)
	return app, ctx, source, destination
}

func newV166TestValidator(t *testing.T, app *TacChainApp, ctx sdk.Context, address sdk.ValAddress) {
	t.Helper()
	validator, err := stakingtypes.NewValidator(address.String(), ed25519.GenPrivKey().PubKey(), stakingtypes.Description{Moniker: "test"})
	require.NoError(t, err)
	validator.Status = stakingtypes.Bonded
	require.NoError(t, app.StakingKeeper.SetValidator(ctx, validator))
	require.NoError(t, app.StakingKeeper.SetValidatorByConsAddr(ctx, validator))
	require.NoError(t, app.StakingKeeper.Hooks().AfterValidatorCreated(ctx, address))
}

func fundV166TestAccount(t *testing.T, app *TacChainApp, ctx sdk.Context, address sdk.AccAddress, number uint64, amount, vesting sdkmath.Int) {
	t.Helper()
	base := authtypes.NewBaseAccount(address, nil, number, 7)
	var account sdk.AccountI = base
	if vesting.IsPositive() {
		locked := sdk.NewCoins(sdk.NewCoin(appconfig.BaseDenom, vesting))
		periodicAccount, err := vestingtypes.NewPeriodicVestingAccount(base, locked, ctx.BlockTime().Unix()-1,
			vestingtypes.Periods{{Length: 365 * 86400, Amount: locked}},
		)
		require.NoError(t, err)
		account = periodicAccount
	}
	app.AccountKeeper.SetAccount(ctx, account)
	if amount.IsPositive() {
		funds := sdk.NewCoins(sdk.NewCoin(appconfig.BaseDenom, amount))
		require.NoError(t, app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, funds))
		require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, address, funds))
	}
}

func delegateV166TestCoins(t *testing.T, app *TacChainApp, ctx sdk.Context, owner sdk.AccAddress, address sdk.ValAddress, amount sdkmath.Int) {
	t.Helper()
	validator, err := app.StakingKeeper.GetValidator(ctx, address)
	require.NoError(t, err)
	_, err = app.StakingKeeper.Delegate(ctx, owner, amount, stakingtypes.Unbonded, validator, true)
	require.NoError(t, err)
}
