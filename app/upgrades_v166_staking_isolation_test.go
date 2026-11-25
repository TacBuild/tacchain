package app

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	appconfig "github.com/TacBuild/tacchain/app/config"
	v166 "github.com/TacBuild/tacchain/app/upgrades/v1.6.6"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestV166AccountStakingIsolatesFailedValidator(t *testing.T) {
	for _, tc := range []struct {
		name          string
		bondConflict  bool
		rejectRewards func() error
	}{
		{name: "missing rewards metadata"},
		{name: "conflicting validator bond flags", bondConflict: true},
		{name: "rewards transfer error", rejectRewards: func() error { return errors.New("rewards transfer rejected") }},
		{name: "rewards transfer panic", rejectRewards: func() error { panic("rewards transfer rejected") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, ctx, source, destination := newV166StakingTestApp(t)
			const denom = appconfig.BaseDenom
			fundV166TestAccount(t, app, ctx, source, 101, sdkmath.NewInt(400), sdkmath.NewInt(400))
			fundV166TestAccount(t, app, ctx, destination, 102, sdkmath.NewInt(25), sdkmath.ZeroInt())

			validators := []sdk.ValAddress{
				bytes.Repeat([]byte{1}, 20),
				bytes.Repeat([]byte{2}, 20),
				bytes.Repeat([]byte{3}, 20),
			}
			slices.SortFunc(validators, func(a, b sdk.ValAddress) int {
				return strings.Compare(a.String(), b.String())
			})
			before, skipped, after := validators[0], validators[1], validators[2]
			for i, amount := range []int64{200, 50, 30} {
				newV166TestValidator(t, app, ctx, validators[i])
				delegateV166TestCoins(t, app, ctx, source, validators[i], sdkmath.NewInt(amount))
			}

			sk := app.StakingKeeper
			redTime, err := sk.BeginRedelegation(ctx, source, before, skipped, sdkmath.LegacyNewDec(40))
			require.NoError(t, err)
			unbondTime, _, err := sk.Undelegate(ctx, source, skipped, sdkmath.LegacyNewDec(10))
			require.NoError(t, err)
			originalUnbonding, err := sk.GetUnbondingDelegation(ctx, source, skipped)
			require.NoError(t, err)
			originalRedelegation, err := sk.GetRedelegation(ctx, source, before, skipped)
			require.NoError(t, err)
			originalDelegation, err := sk.GetDelegation(ctx, source, skipped)
			require.NoError(t, err)
			originalStartingInfo, err := app.DistrKeeper.GetDelegatorStartingInfo(ctx, skipped, source)
			require.NoError(t, err)
			var destinationDelegation stakingtypes.Delegation
			if tc.bondConflict {
				delegateV166TestCoins(t, app, ctx, destination, skipped, sdkmath.NewInt(10))
				_, err := stakingkeeper.NewMsgServerImpl(sk).ValidatorBond(ctx, &stakingtypes.MsgValidatorBond{
					DelegatorAddress: destination.String(),
					ValidatorAddress: skipped.String(),
				})
				require.NoError(t, err)
				destinationDelegation, err = sk.GetDelegation(ctx, destination, skipped)
				require.NoError(t, err)
				require.True(t, destinationDelegation.ValidatorBond)
			}

			ctx = withV166TestBlock(ctx, ctx.BlockHeight()+1, ctx.BlockTime().Add(time.Second))
			for _, validatorAddress := range []sdk.ValAddress{skipped, after} {
				validator, err := sk.GetValidator(ctx, validatorAddress)
				require.NoError(t, err)
				rewards := sdk.NewCoins(sdk.NewCoin(denom, validator.Tokens))
				require.NoError(t, app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, rewards))
				require.NoError(t, app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, distrtypes.ModuleName, rewards))
				require.NoError(t, app.DistrKeeper.AllocateTokensToValidator(ctx, validator, sdk.NewDecCoinsFromCoins(rewards...)))
			}
			previousWithdraw := sdk.AccAddress(bytes.Repeat([]byte{9}, 20))
			require.NoError(t, app.DistrKeeper.SetDelegatorWithdrawAddr(ctx, source, previousWithdraw))

			rejected := 0
			if tc.rejectRewards != nil {
				app.BankKeeper.AppendSendRestriction(func(_ context.Context, from, to sdk.AccAddress, coins sdk.Coins) (sdk.AccAddress, error) {
					if from.Equals(source) && to.Equals(destination) && coins.AmountOf(denom).Equal(sdkmath.NewInt(80)) {
						rejected++
						return nil, tc.rejectRewards()
					}
					return to, nil
				})
			} else if !tc.bondConflict {
				require.NoError(t, app.DistrKeeper.DeleteDelegatorStartingInfo(ctx, skipped, source))
			}
			sourceBalance := app.BankKeeper.GetBalance(ctx, source, denom)
			destinationBalance := app.BankKeeper.GetBalance(ctx, destination, denom)
			supply := app.BankKeeper.GetSupply(ctx, denom)
			bonded, notBonded := sk.GetBondedPool(ctx).GetAddress(), sk.GetNotBondedPool(ctx).GetAddress()
			bondedBalance := app.BankKeeper.GetBalance(ctx, bonded, denom)
			notBondedBalance := app.BankKeeper.GetBalance(ctx, notBonded, denom)
			outstanding, err := app.DistrKeeper.GetValidatorOutstandingRewards(ctx, skipped)
			require.NoError(t, err)

			require.NoError(t, app.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: v166.UpgradeName, Height: ctx.BlockHeight()}))
			doneHeight, err := app.UpgradeKeeper.GetDoneHeight(ctx, v166.UpgradeName)
			require.NoError(t, err)
			require.Equal(t, ctx.BlockHeight(), doneHeight)
			if tc.rejectRewards != nil {
				require.Equal(t, 1, rejected)
			}
			if tc.bondConflict || tc.rejectRewards != nil {
				startingInfo, err := app.DistrKeeper.GetDelegatorStartingInfo(ctx, skipped, source)
				require.NoError(t, err)
				require.Equal(t, originalStartingInfo, startingInfo)
			}
			withdrawAddress, err := app.DistrKeeper.GetDelegatorWithdrawAddr(ctx, source)
			require.NoError(t, err)
			require.Equal(t, previousWithdraw, withdrawAddress)
			require.True(t, app.BankKeeper.GetBalance(ctx, previousWithdraw, denom).IsZero())
			require.True(t, app.BankKeeper.GetBalance(ctx, source, denom).IsZero())
			require.Equal(t, destinationBalance.Add(sourceBalance).Add(sdk.NewInt64Coin(denom, 30)), app.BankKeeper.GetBalance(ctx, destination, denom))
			require.Equal(t, supply, app.BankKeeper.GetSupply(ctx, denom))
			require.Equal(t, bondedBalance, app.BankKeeper.GetBalance(ctx, bonded, denom))
			require.Equal(t, notBondedBalance, app.BankKeeper.GetBalance(ctx, notBonded, denom))
			afterOutstanding, err := app.DistrKeeper.GetValidatorOutstandingRewards(ctx, skipped)
			require.NoError(t, err)
			require.Equal(t, outstanding, afterOutstanding)

			sourceDelegations, err := sk.GetAllDelegatorDelegations(ctx, source)
			require.NoError(t, err)
			require.Equal(t, []stakingtypes.Delegation{originalDelegation}, sourceDelegations)
			for i, validator := range []sdk.ValAddress{before, after} {
				delegation, err := sk.GetDelegation(ctx, destination, validator)
				require.NoError(t, err)
				require.Equal(t, sdkmath.LegacyNewDec([]int64{160, 30}[i]), delegation.Shares)
			}
			delegation, err := sk.GetDelegation(ctx, destination, skipped)
			if tc.bondConflict {
				require.NoError(t, err)
				require.Equal(t, destinationDelegation, delegation)
			} else {
				require.ErrorIs(t, err, stakingtypes.ErrNoDelegation)
			}
			unbonding, err := sk.GetUnbondingDelegationByUnbondingID(ctx, originalUnbonding.Entries[0].UnbondingId)
			require.NoError(t, err)
			require.Equal(t, originalUnbonding, unbonding)
			redelegation, err := sk.GetRedelegationByUnbondingID(ctx, originalRedelegation.Entries[0].UnbondingId)
			require.NoError(t, err)
			require.Equal(t, originalRedelegation, redelegation)
			unbondingQueue, err := sk.GetUBDQueueTimeSlice(ctx, unbondTime)
			require.NoError(t, err)
			require.Equal(t, []stakingtypes.DVPair{{DelegatorAddress: source.String(), ValidatorAddress: skipped.String()}}, unbondingQueue)
			redelegationQueue, err := sk.GetRedelegationQueueTimeSlice(ctx, redTime)
			require.NoError(t, err)
			require.Equal(t, []stakingtypes.DVVTriplet{{
				DelegatorAddress:    source.String(),
				ValidatorSrcAddress: before.String(),
				ValidatorDstAddress: skipped.String(),
			}}, redelegationQueue)
		})
	}
}
