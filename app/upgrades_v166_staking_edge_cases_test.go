package app

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	appconfig "github.com/TacBuild/tacchain/app/config"
	v166 "github.com/TacBuild/tacchain/app/upgrades/v1.6.6"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestV166AccountStakingSlashing(t *testing.T) {
	app, ctx, source, destination := newV166StakingTestApp(t)
	const denom = appconfig.BaseDenom
	tokens := appconfig.PowerReduction.MulRaw
	fundV166TestAccount(t, app, ctx, source, 101, tokens(100), tokens(100))
	fundV166TestAccount(t, app, ctx, destination, 102, tokens(120), tokens(120))

	sk := app.StakingKeeper
	fromValidator := sdk.ValAddress(bytes.Repeat([]byte{1}, 20))
	toValidator := sdk.ValAddress(bytes.Repeat([]byte{2}, 20))
	newV166TestValidator(t, app, ctx, fromValidator)
	newV166TestValidator(t, app, ctx, toValidator)
	delegateV166TestCoins(t, app, ctx, source, fromValidator, tokens(100))
	delegateV166TestCoins(t, app, ctx, destination, fromValidator, tokens(100))
	from, err := sk.GetValidator(ctx, fromValidator)
	require.NoError(t, err)
	consensusAddress, err := from.GetConsAddr()
	require.NoError(t, err)
	infractionHeight := ctx.BlockHeight()
	power := from.GetConsensusPower(appconfig.PowerReduction)
	_, err = sk.BeginRedelegation(ctx, source, fromValidator, toValidator, sdkmath.LegacyNewDecFromInt(tokens(40)))
	require.NoError(t, err)
	unbondTime, _, err := sk.Undelegate(ctx, source, fromValidator, sdkmath.LegacyNewDecFromInt(tokens(20)))
	require.NoError(t, err)

	ctx = withV166TestBlock(ctx, infractionHeight+1, ctx.BlockTime().Add(time.Second))
	_, err = sk.Slash(ctx, sdk.ConsAddress(consensusAddress), infractionHeight, power, sdkmath.LegacyNewDecWithPrec(25, 2))
	require.NoError(t, err)
	sourceUnbonding, err := sk.GetUnbondingDelegation(ctx, source, fromValidator)
	require.NoError(t, err)
	require.Equal(t, tokens(15), sourceUnbonding.Entries[0].Balance)
	unbondingID := sourceUnbonding.Entries[0].UnbondingId
	beforeFrom, err := sk.GetValidator(ctx, fromValidator)
	require.NoError(t, err)
	beforeTo, err := sk.GetValidator(ctx, toValidator)
	require.NoError(t, err)
	require.Equal(t, tokens(105), beforeFrom.Tokens)
	require.Equal(t, tokens(30), beforeTo.Tokens)
	bondedPool, notBondedPool := sk.GetBondedPool(ctx).GetAddress(), sk.GetNotBondedPool(ctx).GetAddress()
	beforeBonded := app.BankKeeper.GetBalance(ctx, bondedPool, denom)
	beforeNotBonded := app.BankKeeper.GetBalance(ctx, notBondedPool, denom)
	beforeSupply := app.BankKeeper.GetSupply(ctx, denom)
	beforeDestination := app.BankKeeper.GetBalance(ctx, destination, denom)
	require.True(t, app.BankKeeper.GetBalance(ctx, source, denom).IsZero())

	ctx = withV166TestBlock(ctx, infractionHeight+2, ctx.BlockTime().Add(time.Second))
	require.NoError(t, app.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: v166.UpgradeName, Height: ctx.BlockHeight()}))
	afterFrom, err := sk.GetValidator(ctx, fromValidator)
	require.NoError(t, err)
	afterTo, err := sk.GetValidator(ctx, toValidator)
	require.NoError(t, err)
	require.Equal(t, beforeFrom, afterFrom)
	require.Equal(t, beforeTo, afterTo)
	require.Equal(t, beforeBonded, app.BankKeeper.GetBalance(ctx, bondedPool, denom))
	require.Equal(t, beforeNotBonded, app.BankKeeper.GetBalance(ctx, notBondedPool, denom))
	require.Equal(t, beforeSupply, app.BankKeeper.GetSupply(ctx, denom))
	delegation, err := sk.GetDelegation(ctx, destination, fromValidator)
	require.NoError(t, err)
	require.Equal(t, sdkmath.LegacyNewDecFromInt(tokens(140)), delegation.Shares)
	require.IsType(t, &authtypes.BaseAccount{}, app.AccountKeeper.GetAccount(ctx, destination))
	require.Equal(t, beforeDestination.Amount, app.BankKeeper.SpendableCoins(ctx, destination).AmountOf(denom))

	_, err = sk.Slash(ctx, sdk.ConsAddress(consensusAddress), infractionHeight, power, sdkmath.LegacyNewDecWithPrec(75, 3))
	require.NoError(t, err)
	afterFrom, err = sk.GetValidator(ctx, fromValidator)
	require.NoError(t, err)
	delegation, err = sk.GetDelegation(ctx, destination, fromValidator)
	require.NoError(t, err)
	require.Equal(t, sdkmath.LegacyNewDecFromInt(tokens(140)), delegation.Shares)
	stakeAfterSlash := tokens(189).QuoRaw(2)
	require.Equal(t, stakeAfterSlash, afterFrom.TokensFromShares(delegation.Shares).TruncateInt())
	redelegated, err := sk.GetDelegation(ctx, destination, toValidator)
	require.NoError(t, err)
	require.Equal(t, sdkmath.LegacyNewDecFromInt(tokens(27)), redelegated.Shares)
	afterUnbonding, err := sk.GetUnbondingDelegationByUnbondingID(ctx, unbondingID)
	require.NoError(t, err)
	require.Equal(t, destination.String(), afterUnbonding.DelegatorAddress)
	pendingAfterSlash := tokens(27).QuoRaw(2)
	require.Equal(t, pendingAfterSlash, afterUnbonding.Entries[0].Balance)
	require.Equal(t, beforeSupply.Sub(sdk.NewCoin(denom, tokens(15))), app.BankKeeper.GetSupply(ctx, denom))

	ctx = withV166TestBlock(ctx, ctx.BlockHeight()+1, ctx.BlockTime().Add(time.Second))
	rewards := sdk.NewCoins(sdk.NewCoin(denom, stakeAfterSlash))
	require.NoError(t, app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, rewards))
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, distrtypes.ModuleName, rewards))
	require.NoError(t, app.DistrKeeper.AllocateTokensToValidator(ctx, afterFrom, sdk.NewDecCoinsFromCoins(rewards...)))
	withdrawn, err := app.DistrKeeper.WithdrawDelegationRewards(ctx, destination, fromValidator)
	require.NoError(t, err)
	require.Equal(t, rewards, withdrawn)

	ctx = withV166TestBlock(ctx, ctx.BlockHeight()+1, unbondTime)
	returned, err := sk.CompleteUnbonding(ctx, destination, fromValidator)
	require.NoError(t, err)
	require.Equal(t, pendingAfterSlash, returned.AmountOf(denom))
	require.Equal(t, beforeDestination.Amount.Add(stakeAfterSlash).Add(pendingAfterSlash), app.BankKeeper.SpendableCoins(ctx, destination).AmountOf(denom))
}

func TestV166AccountStakingFractionalShares(t *testing.T) {
	app, ctx, source, destination := newV166StakingTestApp(t)
	const denom = appconfig.BaseDenom
	amount := sdkmath.NewInt(100)
	fundV166TestAccount(t, app, ctx, source, 101, amount, amount)
	fundV166TestAccount(t, app, ctx, destination, 102, amount, amount)
	validatorAddress := sdk.ValAddress(bytes.Repeat([]byte{1}, 20))
	newV166TestValidator(t, app, ctx, validatorAddress)
	delegateV166TestCoins(t, app, ctx, source, validatorAddress, amount)
	delegateV166TestCoins(t, app, ctx, destination, validatorAddress, amount)
	shares := sdkmath.LegacyNewDecWithPrec(1, 1)
	sk := app.StakingKeeper
	_, unbonded, err := sk.Undelegate(ctx, source, validatorAddress, sdkmath.LegacyNewDec(100).Sub(shares))
	require.NoError(t, err)
	require.Equal(t, sdkmath.NewInt(99), unbonded)
	validator, err := sk.GetValidator(ctx, validatorAddress)
	require.NoError(t, err)
	require.True(t, validator.TokensFromShares(shares).TruncateInt().IsZero())

	ctx = withV166TestBlock(ctx, ctx.BlockHeight()+1, ctx.BlockTime().Add(time.Second))
	require.NoError(t, app.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: v166.UpgradeName, Height: ctx.BlockHeight()}))
	_, err = sk.GetDelegation(ctx, source, validatorAddress)
	require.ErrorIs(t, err, stakingtypes.ErrNoDelegation)
	delegation, err := sk.GetDelegation(ctx, destination, validatorAddress)
	require.NoError(t, err)
	require.Equal(t, sdkmath.LegacyNewDec(100).Add(shares), delegation.Shares)
	require.IsType(t, &authtypes.BaseAccount{}, app.AccountKeeper.GetAccount(ctx, destination))
}

func TestV166AccountStakingHeldUnbondingOnly(t *testing.T) {
	app, ctx, source, destination := newV166StakingTestApp(t)
	const denom = appconfig.BaseDenom
	amount := sdkmath.NewInt(100)
	fundV166TestAccount(t, app, ctx, source, 101, amount, amount)
	fundV166TestAccount(t, app, ctx, destination, 102, sdkmath.ZeroInt(), sdkmath.ZeroInt())
	validatorAddress := sdk.ValAddress(bytes.Repeat([]byte{1}, 20))
	newV166TestValidator(t, app, ctx, validatorAddress)
	delegateV166TestCoins(t, app, ctx, source, validatorAddress, amount)
	sk := app.StakingKeeper
	completionTime, _, err := sk.Undelegate(ctx, source, validatorAddress, sdkmath.LegacyNewDecFromInt(amount))
	require.NoError(t, err)
	require.True(t, app.BankKeeper.GetBalance(ctx, source, denom).IsZero())
	unbonding, err := sk.GetUnbondingDelegation(ctx, source, validatorAddress)
	require.NoError(t, err)
	unbondingID := unbonding.Entries[0].UnbondingId
	require.NoError(t, sk.PutUnbondingOnHold(ctx, unbondingID))

	ctx = withV166TestBlock(ctx, ctx.BlockHeight()+1, completionTime)
	queue, err := sk.DequeueAllMatureUBDQueue(ctx, completionTime)
	require.NoError(t, err)
	require.Len(t, queue, 1)
	returned, err := sk.CompleteUnbonding(ctx, source, validatorAddress)
	require.NoError(t, err)
	require.Empty(t, returned)

	ctx = withV166TestBlock(ctx, ctx.BlockHeight()+1, completionTime.Add(time.Second))
	require.NoError(t, app.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: v166.UpgradeName, Height: ctx.BlockHeight()}))
	require.NoError(t, sk.UnbondingCanComplete(ctx, unbondingID))
	require.Equal(t, amount, app.BankKeeper.SpendableCoins(ctx, destination).AmountOf(denom))
	require.True(t, app.BankKeeper.GetBalance(ctx, source, denom).IsZero())
}

func withV166TestBlock(ctx sdk.Context, height int64, blockTime time.Time) sdk.Context {
	info := ctx.HeaderInfo()
	info.Height = height
	info.Time = blockTime
	return ctx.WithBlockHeight(height).WithBlockTime(blockTime).WithHeaderInfo(info)
}
