package v166

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"

	appconfig "github.com/TacBuild/tacchain/app/config"
	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type validatorStakingPositions struct {
	delegation    *stakingtypes.Delegation
	unbonding     *stakingtypes.UnbondingDelegation
	redelegations []stakingtypes.Redelegation
}

func migrateAccountStaking(ctx sdk.Context, ak *upgrades.AppKeepers, source, destination sdk.AccAddress) error {
	positions, err := collectStakingPositions(ctx, ak.StakingKeeper, source)
	if err != nil {
		return err
	}

	var skipped int
	for _, validator := range slices.Sorted(maps.Keys(positions)) {
		cacheCtx, write := ctx.CacheContext()
		if err := transferStakingPositions(cacheCtx, ak, source, destination, positions[validator]); err != nil {
			skipped++
			ctx.Logger().Error("Staking positions skipped", "validator", validator, "error", err)
			continue
		}
		write()
	}
	ctx.Logger().Info("Account staking migration finished", "transferred", len(positions)-skipped, "skipped", skipped)
	return nil
}

func collectStakingPositions(ctx sdk.Context, sk *stakingkeeper.Keeper, source sdk.AccAddress) (positions map[string]*validatorStakingPositions, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			positions = nil
			err = fmt.Errorf("read staking positions panic: %v", recovered)
		}
	}()

	denom, err := sk.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	if denom != appconfig.BaseDenom {
		return nil, fmt.Errorf("unexpected staking denomination %s", denom)
	}
	positions = make(map[string]*validatorStakingPositions)
	forValidator := func(address string) *validatorStakingPositions {
		validator := strings.ToLower(address)
		group := positions[validator]
		if group == nil {
			group = &validatorStakingPositions{}
			positions[validator] = group
		}
		return group
	}

	delegations, err := sk.GetAllDelegatorDelegations(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("read delegations: %w", err)
	}
	for _, delegation := range delegations {
		forValidator(delegation.ValidatorAddress).delegation = &delegation
	}
	if err := sk.IterateDelegatorUnbondingDelegations(ctx, source, func(record stakingtypes.UnbondingDelegation) bool {
		forValidator(record.ValidatorAddress).unbonding = &record
		return false
	}); err != nil {
		return nil, fmt.Errorf("read unbonding delegations: %w", err)
	}
	if err := sk.IterateDelegatorRedelegations(ctx, source, func(record stakingtypes.Redelegation) bool {
		group := forValidator(record.ValidatorDstAddress)
		group.redelegations = append(group.redelegations, record)
		return false
	}); err != nil {
		return nil, fmt.Errorf("read redelegations: %w", err)
	}
	return positions, nil
}

func transferStakingPositions(ctx sdk.Context, ak *upgrades.AppKeepers, source, destination sdk.AccAddress, positions *validatorStakingPositions) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("staking migration panic: %v", recovered)
		}
	}()

	var vestingAccount *vestingtypes.PeriodicVestingAccount
	switch account := ak.AccountKeeper.GetAccount(ctx, destination).(type) {
	case *authtypes.BaseAccount:
	case *vestingtypes.PeriodicVestingAccount:
		if account.BaseVestingAccount == nil || account.BaseAccount == nil {
			return fmt.Errorf("destination base account not found")
		}
		vestingAccount = account
	default:
		return fmt.Errorf("unexpected destination account type %T", account)
	}

	amount := sdkmath.ZeroInt()
	if positions.delegation != nil {
		delegated, err := transferDelegationWithRewards(ctx, ak, source, destination, *positions.delegation)
		if err != nil {
			return fmt.Errorf("transfer delegation: %w", err)
		}
		amount = amount.Add(delegated)
	}
	if positions.unbonding != nil {
		unbonding, err := transferUnbondingDelegation(ctx, ak.StakingKeeper, destination, *positions.unbonding)
		if err != nil {
			return fmt.Errorf("transfer unbonding delegation: %w", err)
		}
		amount = amount.Add(unbonding)
	}
	for _, redelegation := range positions.redelegations {
		if err := transferRedelegation(ctx, ak.StakingKeeper, destination, redelegation); err != nil {
			return fmt.Errorf("transfer redelegation: %w", err)
		}
	}

	if vestingAccount != nil && amount.IsPositive() {
		vestingAccount.DelegatedFree = vestingAccount.DelegatedFree.Add(sdk.NewCoin(appconfig.BaseDenom, amount))
		ak.AccountKeeper.SetAccount(ctx, vestingAccount)
	}
	if err := transferAccountBalance(ctx, ak.BankKeeper, source, destination); err != nil {
		return fmt.Errorf("transfer staking rewards: %w", err)
	}
	return nil
}

func transferDelegationWithRewards(ctx sdk.Context, ak *upgrades.AppKeepers, source, destination sdk.AccAddress, delegation stakingtypes.Delegation) (sdkmath.Int, error) {
	withdrawAddress, err := ak.DistrKeeper.GetDelegatorWithdrawAddr(ctx, source)
	if err != nil {
		return sdkmath.ZeroInt(), err
	}
	overrideWithdrawAddress := !withdrawAddress.Equals(source)
	if overrideWithdrawAddress {
		if err := ak.DistrKeeper.SetDelegatorWithdrawAddr(ctx, source, source); err != nil {
			return sdkmath.ZeroInt(), err
		}
	}

	amount, err := transferDelegation(ctx, ak, source, destination, delegation)
	if err != nil {
		return amount, err
	}
	if overrideWithdrawAddress {
		return amount, ak.DistrKeeper.SetDelegatorWithdrawAddr(ctx, source, withdrawAddress)
	}
	return amount, nil
}

func transferDelegation(ctx sdk.Context, ak *upgrades.AppKeepers, source, destination sdk.AccAddress, delegation stakingtypes.Delegation) (sdkmath.Int, error) {
	zero := sdkmath.ZeroInt()
	sk := ak.StakingKeeper
	validatorBytes, err := sk.ValidatorAddressCodec().StringToBytes(delegation.ValidatorAddress)
	if err != nil {
		return zero, err
	}
	validatorAddress := sdk.ValAddress(validatorBytes)
	if delegation.Shares.IsZero() {
		if err := settleDelegationRewards(ctx, ak, source, validatorAddress, delegation.Shares); err != nil {
			return zero, err
		}
		return zero, sk.RemoveDelegation(ctx, delegation)
	}
	validator, err := sk.GetValidator(ctx, validatorAddress)
	if err != nil {
		return zero, err
	}
	if source.Equals(validatorAddress) && !validator.Jailed && validator.MinSelfDelegation.IsPositive() {
		return zero, errors.New("cannot transfer required validator self-delegation")
	}

	existing, err := sk.GetDelegation(ctx, destination, validatorAddress)
	hasExisting := err == nil
	if err != nil && !errors.Is(err, stakingtypes.ErrNoDelegation) {
		return zero, err
	}
	updated := delegation
	updated.DelegatorAddress = destination.String()
	previousAmount := zero
	if hasExisting {
		if existing.ValidatorBond != delegation.ValidatorBond {
			return zero, fmt.Errorf("incompatible delegation bond flags")
		}
		updated = existing
		updated.Shares = existing.Shares.Add(delegation.Shares)
		if existing.Shares.IsPositive() {
			previousAmount = validator.TokensFromShares(existing.Shares).TruncateInt()
		}
	}

	hooks := sk.Hooks()
	if err := hooks.BeforeDelegationSharesModified(ctx, source, validatorAddress); err != nil {
		return zero, err
	}
	if hasExisting {
		err = settleDelegationRewards(ctx, ak, destination, validatorAddress, existing.Shares)
	} else {
		err = hooks.BeforeDelegationCreated(ctx, destination, validatorAddress)
	}
	if err != nil {
		return zero, err
	}
	if err := sk.RemoveDelegation(ctx, delegation); err != nil {
		return zero, err
	}
	if err := sk.SetDelegation(ctx, updated); err != nil {
		return zero, err
	}
	if err := hooks.AfterDelegationModified(ctx, destination, validatorAddress); err != nil {
		return zero, err
	}
	return validator.TokensFromShares(updated.Shares).TruncateInt().Sub(previousAmount), nil
}

func settleDelegationRewards(ctx sdk.Context, ak *upgrades.AppKeepers, delegator sdk.AccAddress, validator sdk.ValAddress, shares sdkmath.LegacyDec) error {
	if shares.IsZero() {
		initialized, err := ak.DistrKeeper.HasDelegatorStartingInfo(ctx, validator, delegator)
		if err != nil || !initialized {
			return err
		}
	}
	return ak.StakingKeeper.Hooks().BeforeDelegationSharesModified(ctx, delegator, validator)
}

func transferUnbondingDelegation(ctx sdk.Context, sk *stakingkeeper.Keeper, destination sdk.AccAddress, record stakingtypes.UnbondingDelegation) (sdkmath.Int, error) {
	amount := sdkmath.ZeroInt()
	validator, err := sk.ValidatorAddressCodec().StringToBytes(record.ValidatorAddress)
	if err != nil {
		return amount, err
	}
	updated := record
	updated.DelegatorAddress = destination.String()
	existing, err := sk.GetUnbondingDelegation(ctx, destination, validator)
	if err == nil {
		updated.Entries = append(existing.Entries, record.Entries...)
	} else if !errors.Is(err, stakingtypes.ErrNoUnbondingDelegation) {
		return amount, err
	}
	if err := sk.RemoveUnbondingDelegation(ctx, record); err != nil {
		return amount, err
	}
	if err := sk.SetUnbondingDelegation(ctx, updated); err != nil {
		return amount, err
	}

	oldPair := stakingtypes.DVPair{DelegatorAddress: record.DelegatorAddress, ValidatorAddress: record.ValidatorAddress}
	newPair := oldPair
	newPair.DelegatorAddress = updated.DelegatorAddress
	queued := make(map[time.Time]struct{})
	for _, entry := range record.Entries {
		amount = amount.Add(entry.Balance)
		if entry.UnbondingId != 0 {
			if err := sk.SetUnbondingDelegationByUnbondingID(ctx, updated, entry.UnbondingId); err != nil {
				return amount, err
			}
		}
		at := entry.CompletionTime.UTC()
		if _, exists := queued[at]; exists {
			continue
		}
		queued[at] = struct{}{}
		queue, err := sk.GetUBDQueueTimeSlice(ctx, at)
		if err != nil {
			return amount, err
		}
		if err := sk.SetUBDQueueTimeSlice(ctx, at, replaceQueueEntry(queue, oldPair, newPair)); err != nil {
			return amount, err
		}
	}
	return amount, nil
}

func transferRedelegation(ctx sdk.Context, sk *stakingkeeper.Keeper, destination sdk.AccAddress, record stakingtypes.Redelegation) error {
	codec := sk.ValidatorAddressCodec()
	fromValidator, err := codec.StringToBytes(record.ValidatorSrcAddress)
	if err != nil {
		return err
	}
	toValidator, err := codec.StringToBytes(record.ValidatorDstAddress)
	if err != nil {
		return err
	}
	updated := record
	updated.DelegatorAddress = destination.String()
	existing, err := sk.GetRedelegation(ctx, destination, fromValidator, toValidator)
	if err == nil {
		updated.Entries = append(existing.Entries, record.Entries...)
	} else if !errors.Is(err, stakingtypes.ErrNoRedelegation) {
		return err
	}
	if err := sk.RemoveRedelegation(ctx, record); err != nil {
		return err
	}
	if err := sk.SetRedelegation(ctx, updated); err != nil {
		return err
	}

	oldKey := stakingtypes.DVVTriplet{
		DelegatorAddress:    record.DelegatorAddress,
		ValidatorSrcAddress: record.ValidatorSrcAddress,
		ValidatorDstAddress: record.ValidatorDstAddress,
	}
	newKey := oldKey
	newKey.DelegatorAddress = updated.DelegatorAddress
	queued := make(map[time.Time]struct{})
	for _, entry := range record.Entries {
		if entry.UnbondingId != 0 {
			if err := sk.SetRedelegationByUnbondingID(ctx, updated, entry.UnbondingId); err != nil {
				return err
			}
		}
		at := entry.CompletionTime.UTC()
		if _, exists := queued[at]; exists {
			continue
		}
		queued[at] = struct{}{}
		queue, err := sk.GetRedelegationQueueTimeSlice(ctx, at)
		if err != nil {
			return err
		}
		if err := sk.SetRedelegationQueueTimeSlice(ctx, at, replaceQueueEntry(queue, oldKey, newKey)); err != nil {
			return err
		}
	}
	return nil
}

func replaceQueueEntry[T comparable](queue []T, oldEntry, newEntry T) []T {
	queue = slices.DeleteFunc(queue, func(entry T) bool { return entry == oldEntry })
	if !slices.Contains(queue, newEntry) {
		queue = append(queue, newEntry)
	}
	return queue
}
