package v166

import (
	"errors"
	"fmt"
	"slices"
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

func migrateAccountStaking(ctx sdk.Context, ak *upgrades.AppKeepers, source, destination sdk.AccAddress) error {
	cacheCtx, write := ctx.CacheContext()
	if err := transferAccountStaking(cacheCtx, ak, source, destination); err != nil {
		return err
	}
	write()
	return nil
}

func transferAccountStaking(ctx sdk.Context, ak *upgrades.AppKeepers, source, destination sdk.AccAddress) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("staking migration panic: %v", recovered)
		}
	}()

	denom, err := ak.StakingKeeper.BondDenom(ctx)
	if err != nil {
		return err
	}
	if denom != appconfig.BaseDenom {
		return fmt.Errorf("unexpected staking denomination %s", denom)
	}

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

	delegated, err := transferDelegations(ctx, ak, source, destination)
	if err != nil {
		return fmt.Errorf("transfer delegations: %w", err)
	}
	unbonding, err := transferUnbondingDelegations(ctx, ak.StakingKeeper, source, destination)
	if err != nil {
		return fmt.Errorf("transfer unbonding delegations: %w", err)
	}
	if err := transferRedelegations(ctx, ak.StakingKeeper, source, destination); err != nil {
		return fmt.Errorf("transfer redelegations: %w", err)
	}

	if amount := delegated.Add(unbonding); vestingAccount != nil && amount.IsPositive() {
		vestingAccount.DelegatedFree = vestingAccount.DelegatedFree.Add(sdk.NewCoin(denom, amount))
		ak.AccountKeeper.SetAccount(ctx, vestingAccount)
	}
	if err := transferAccountBalance(ctx, ak.BankKeeper, source, destination); err != nil {
		return fmt.Errorf("transfer staking rewards: %w", err)
	}
	return nil
}

func transferDelegations(ctx sdk.Context, ak *upgrades.AppKeepers, source, destination sdk.AccAddress) (sdkmath.Int, error) {
	amount := sdkmath.ZeroInt()
	delegations, err := ak.StakingKeeper.GetAllDelegatorDelegations(ctx, source)
	if err != nil || len(delegations) == 0 {
		return amount, err
	}

	withdrawAddress, err := ak.DistrKeeper.GetDelegatorWithdrawAddr(ctx, source)
	if err != nil {
		return amount, err
	}
	overrideWithdrawAddress := !withdrawAddress.Equals(source)
	if overrideWithdrawAddress {
		if err := ak.DistrKeeper.SetDelegatorWithdrawAddr(ctx, source, source); err != nil {
			return amount, err
		}
	}
	for _, delegation := range delegations {
		moved, err := transferDelegation(ctx, ak, source, destination, delegation)
		if err != nil {
			return amount, err
		}
		amount = amount.Add(moved)
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

func transferUnbondingDelegations(ctx sdk.Context, sk *stakingkeeper.Keeper, source, destination sdk.AccAddress) (sdkmath.Int, error) {
	amount := sdkmath.ZeroInt()
	var records []stakingtypes.UnbondingDelegation
	if err := sk.IterateDelegatorUnbondingDelegations(ctx, source, func(record stakingtypes.UnbondingDelegation) bool {
		records = append(records, record)
		return false
	}); err != nil {
		return amount, err
	}
	for _, record := range records {
		moved, err := transferUnbondingDelegation(ctx, sk, destination, record)
		if err != nil {
			return amount, err
		}
		amount = amount.Add(moved)
	}
	return amount, nil
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

func transferRedelegations(ctx sdk.Context, sk *stakingkeeper.Keeper, source, destination sdk.AccAddress) error {
	var records []stakingtypes.Redelegation
	if err := sk.IterateDelegatorRedelegations(ctx, source, func(record stakingtypes.Redelegation) bool {
		records = append(records, record)
		return false
	}); err != nil {
		return err
	}
	for _, record := range records {
		if err := transferRedelegation(ctx, sk, destination, record); err != nil {
			return err
		}
	}
	return nil
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
