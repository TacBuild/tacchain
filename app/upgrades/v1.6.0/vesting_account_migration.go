package v160

import (
	"fmt"
	"time"

	"cosmossdk.io/math"

	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// migrateVestingAccount performs the full migration:
//  1. Load and validate old PeriodicVestingAccount
//  2. Withdraw delegation rewards from old account
//     2a. Migrate unbonding delegations (store-level rewrite to new address)
//     2b. Migrate redelegations (store-level rewrite to new address)
//     2c. Migrate tokenize share record ownership related to the old account
//  3. Move delegations from old to new address (store-level rewrite)
//  4. Clean up old account (replace with BaseAccount to make all coins spendable)
//  5. Move all remaining balances from old to new address
//  6. Create identical PeriodicVestingAccount at new address
func migrateVestingAccount(ctx sdk.Context, ak *upgrades.AppKeepers, oldAddress string, newAddress string) error {
	logger := ctx.Logger()

	oldAddr, err := sdk.AccAddressFromBech32(oldAddress)
	if err != nil {
		return fmt.Errorf("invalid old address: %w", err)
	}
	newAddr, err := sdk.AccAddressFromBech32(newAddress)
	if err != nil {
		return fmt.Errorf("invalid new address: %w", err)
	}

	// ──────────────────────────────────────────────────────────────
	// 1. Load and validate old account
	// ──────────────────────────────────────────────────────────────
	oldAcc := ak.AccountKeeper.GetAccount(ctx, oldAddr)
	if oldAcc == nil {
		return fmt.Errorf("old account %s not found", oldAddress)
	}

	oldVestingAcc, ok := oldAcc.(*vestingtypes.PeriodicVestingAccount)
	if !ok {
		return fmt.Errorf("old account is not a PeriodicVestingAccount, got %T", oldAcc)
	}

	logger.Info("Old vesting account loaded",
		"address", oldAddress,
		"original_vesting", oldVestingAcc.OriginalVesting.String(),
		"start_time", oldVestingAcc.StartTime,
		"end_time", oldVestingAcc.EndTime,
		"periods", len(oldVestingAcc.VestingPeriods),
	)

	// ──────────────────────────────────────────────────────────────
	// 2. Withdraw all staking rewards from old account
	// ──────────────────────────────────────────────────────────────
	// A leaked old key can set a custom withdraw address before the upgrade.
	// Force rewards directly to the new account before withdrawing them.
	if err := ak.DistrKeeper.SetDelegatorWithdrawAddr(ctx, oldAddr, newAddr); err != nil {
		return fmt.Errorf("failed to redirect withdraw address to new account: %w", err)
	}

	delegations, err := snapshotDelegatorDelegations(ctx, ak, oldAddr)
	if err != nil {
		return fmt.Errorf("failed to get delegations: %w", err)
	}

	for _, del := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if err != nil {
			return fmt.Errorf("invalid validator address %s: %w", del.ValidatorAddress, err)
		}

		rewards, err := ak.DistrKeeper.WithdrawDelegationRewards(ctx, oldAddr, valAddr)
		if err != nil {
			logger.Error("failed to withdraw rewards", "validator", del.ValidatorAddress, "error", err)
			// Continue — rewards may be zero
		} else {
			logger.Info("Withdrawn delegation rewards",
				"validator", del.ValidatorAddress,
				"rewards", rewards.String(),
			)
		}
	}
	if err := ak.DistrKeeper.DeleteDelegatorWithdrawAddr(ctx, oldAddr, newAddr); err != nil {
		return fmt.Errorf("failed to clear old account withdraw address: %w", err)
	}

	// ──────────────────────────────────────────────────────────────
	// 2a. Migrate unbonding delegations (store-level rewrite)
	//     We rewrite UBD records from old→new delegator address so that
	//     when they mature, tokens go to the new address.
	//     This does NOT touch validator power or token pools.
	// ──────────────────────────────────────────────────────────────
	if err := migrateUnbondingDelegations(ctx, ak, oldAddr, oldAddress, newAddress); err != nil {
		return fmt.Errorf("failed to migrate unbonding delegations: %w", err)
	}

	// ──────────────────────────────────────────────────────────────
	// 2b. Migrate redelegations (store-level rewrite)
	//     Redelegation records are bookkeeping entries that prevent
	//     double-redelegation. The actual delegation is already at the
	//     destination validator. We rewrite them to the new address.
	// ──────────────────────────────────────────────────────────────
	if err := migrateRedelegations(ctx, ak, oldAddr, oldAddress, newAddress); err != nil {
		return fmt.Errorf("failed to migrate redelegations: %w", err)
	}

	// ──────────────────────────────────────────────────────────────
	// 2c. Migrate tokenize share record ownership (store-level rewrite)
	//     Tokenized shares keep their reward/transfer owner in staking state.
	//     If the old vesting address owns a record, or still holds that record's
	//     share-token balance, rewrite ownership to the rescued address so a
	//     custom pre-upgrade TokenizedShareOwner cannot keep control.
	// ──────────────────────────────────────────────────────────────
	if err := migrateTokenizeShareRecordOwners(ctx, ak, oldAddr, newAddr); err != nil {
		return fmt.Errorf("failed to migrate tokenize share record owners: %w", err)
	}

	// ──────────────────────────────────────────────────────────────
	// 3. Move delegations from old address to new address
	//    We do a store-level rewrite: remove the old delegation record
	//    and create a new one with the same shares. This does NOT touch
	//    validator tokens/power, avoiding the "duplicate validator set
	//    entry" error that Unbond+Delegate causes within a single block.
	//    Distribution hooks are called to keep reward tracking correct.
	// ──────────────────────────────────────────────────────────────
	for _, del := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if err != nil {
			return fmt.Errorf("invalid validator address: %w", err)
		}

		validator, err := ak.StakingKeeper.GetValidator(ctx, valAddr)
		if err != nil {
			return fmt.Errorf("validator %s not found: %w", del.ValidatorAddress, err)
		}

		tokens := validator.TokensFromShares(del.Shares).TruncateInt()

		logger.Info("Migrating delegation (store-level rewrite)",
			"validator", del.ValidatorAddress,
			"shares", del.Shares.String(),
			"tokens", tokens.String(),
		)

		// 3a. Call BeforeDelegationSharesModified for old addr to finalize
		//     distribution rewards tracking (already withdrawn above, but
		//     the hook also increments the validator period).
		if err := ak.StakingKeeper.Hooks().BeforeDelegationSharesModified(ctx, oldAddr, valAddr); err != nil {
			return fmt.Errorf("BeforeDelegationSharesModified hook failed: %w", err)
		}

		// 3b. Remove old delegation record from store
		if err := ak.StakingKeeper.RemoveDelegation(ctx, del); err != nil {
			return fmt.Errorf("failed to remove old delegation: %w", err)
		}

		// 3c. Call BeforeDelegationCreated for new addr (increments validator period)
		if err := ak.StakingKeeper.Hooks().BeforeDelegationCreated(ctx, newAddr, valAddr); err != nil {
			return fmt.Errorf("BeforeDelegationCreated hook failed: %w", err)
		}

		// 3d. Create new delegation record with the same data, preserving
		//     ValidatorBond and any future delegation fields.
		newDelegation := del
		newDelegation.DelegatorAddress = newAddress
		if err := ak.StakingKeeper.SetDelegation(ctx, newDelegation); err != nil {
			return fmt.Errorf("failed to set new delegation: %w", err)
		}

		// 3e. Call AfterDelegationModified to initialize distribution tracking
		if err := ak.StakingKeeper.Hooks().AfterDelegationModified(ctx, newAddr, valAddr); err != nil {
			return fmt.Errorf("AfterDelegationModified hook failed: %w", err)
		}

		logger.Info("Delegation migrated (store-level)",
			"validator", del.ValidatorAddress,
			"shares", del.Shares.String(),
			"tokens", tokens.String(),
		)
	}

	// ──────────────────────────────────────────────────────────────
	// 4. Clean up old account: replace PeriodicVestingAccount with
	//    a plain BaseAccount. This makes ALL coins spendable because
	//    BaseAccount has no vesting lock.
	// ──────────────────────────────────────────────────────────────
	tombstoneAcc := authtypes.NewBaseAccountWithAddress(oldAddr)
	tombstoneAcc.AccountNumber = oldVestingAcc.GetAccountNumber()
	tombstoneAcc.Sequence = oldVestingAcc.GetSequence() + 1 // bump sequence to invalidate pending txs
	ak.AccountKeeper.SetAccount(ctx, tombstoneAcc)

	logger.Info("Old account cleaned up (converted to BaseAccount)",
		"address", oldAddress,
		"new_sequence", tombstoneAcc.Sequence,
	)

	// ──────────────────────────────────────────────────────────────
	// 5. Move all remaining balances from old to new address
	//    Now that old account is a BaseAccount, all coins are spendable.
	// ──────────────────────────────────────────────────────────────
	oldBalances := ak.BankKeeper.GetAllBalances(ctx, oldAddr)
	if oldBalances.IsAllPositive() {
		if err := ak.BankKeeper.SendCoins(ctx, oldAddr, newAddr, oldBalances); err != nil {
			return fmt.Errorf("failed to transfer balances: %w", err)
		}
		logger.Info("Transferred remaining balances",
			"amount", oldBalances.String(),
		)
	}

	// ──────────────────────────────────────────────────────────────
	// 6. Create new PeriodicVestingAccount with identical schedule
	//    We create it AFTER transferring coins so the new account
	//    already holds the correct balance.
	// ──────────────────────────────────────────────────────────────
	existingNewAcc := ak.AccountKeeper.GetAccount(ctx, newAddr)

	if existingNewAcc != nil {
		logger.Info("Rescue destination account exists",
			"address", newAddress,
			"type", fmt.Sprintf("%T", existingNewAcc),
			"sequence", existingNewAcc.GetSequence(),
		)
	}

	// The new address may now have a BaseAccount, or a front-run vesting
	// account created without the destination key. Convert cleanup-safe
	// destination state into the BaseAccount used by the rescued vesting account.
	newBaseAcc, err := rescueDestinationBaseAccount(ctx, ak, newAddr)
	if err != nil {
		return fmt.Errorf("new address %s cannot be used as rescue destination: %w", newAddress, err)
	}

	newVestingAcc, err := vestingtypes.NewPeriodicVestingAccount(
		newBaseAcc,
		oldVestingAcc.OriginalVesting,
		oldVestingAcc.StartTime,
		oldVestingAcc.VestingPeriods,
	)
	if err != nil {
		return fmt.Errorf("failed to create new vesting account: %w", err)
	}

	// Copy DelegatedVesting / DelegatedFree from old account
	newVestingAcc.DelegatedVesting = oldVestingAcc.DelegatedVesting
	newVestingAcc.DelegatedFree = oldVestingAcc.DelegatedFree

	ak.AccountKeeper.SetAccount(ctx, newVestingAcc)

	logger.Info("New vesting account created",
		"address", newAddress,
		"original_vesting", newVestingAcc.OriginalVesting.String(),
		"start_time", newVestingAcc.StartTime,
		"end_time", newVestingAcc.EndTime,
	)

	return nil
}

func snapshotDelegatorDelegations(ctx sdk.Context, ak *upgrades.AppKeepers, delegator sdk.AccAddress) ([]stakingtypes.Delegation, error) {
	var delegations []stakingtypes.Delegation
	err := ak.StakingKeeper.IterateDelegatorDelegations(ctx, delegator, func(delegation stakingtypes.Delegation) bool {
		delegations = append(delegations, delegation)
		return false
	})
	if err != nil {
		return nil, err
	}

	return delegations, nil
}

func snapshotDelegatorUnbondingDelegations(ctx sdk.Context, ak *upgrades.AppKeepers, delegator sdk.AccAddress) ([]stakingtypes.UnbondingDelegation, error) {
	var ubds []stakingtypes.UnbondingDelegation
	err := ak.StakingKeeper.IterateDelegatorUnbondingDelegations(ctx, delegator, func(ubd stakingtypes.UnbondingDelegation) bool {
		ubds = append(ubds, ubd)
		return false
	})
	if err != nil {
		return nil, err
	}

	return ubds, nil
}

func snapshotDelegatorRedelegations(ctx sdk.Context, ak *upgrades.AppKeepers, delegator sdk.AccAddress) ([]stakingtypes.Redelegation, error) {
	var reds []stakingtypes.Redelegation
	err := ak.StakingKeeper.IterateDelegatorRedelegations(ctx, delegator, func(red stakingtypes.Redelegation) bool {
		reds = append(reds, red)
		return false
	})
	if err != nil {
		return nil, err
	}

	return reds, nil
}

// migrateTokenizeShareRecordOwners rewrites TokenizeShareRecord.Owner to newAddr
// for records controlled by, or economically tied to, oldAddr. The share-token
// balance check covers a malicious pre-upgrade tokenization where the old key
// sets TokenizedShareOwner to a third-party address while leaving minted share
// tokens on the rescued account.
func migrateTokenizeShareRecordOwners(ctx sdk.Context, ak *upgrades.AppKeepers, oldAddr, newAddr sdk.AccAddress) error {
	records := ak.StakingKeeper.GetAllTokenizeShareRecords(ctx)
	if len(records) == 0 {
		return nil
	}

	for _, record := range records {
		shareTokenBalance := ak.BankKeeper.GetBalance(ctx, oldAddr, record.GetShareTokenDenom())
		if record.Owner != oldAddr.String() && !shareTokenBalance.IsPositive() {
			continue
		}
		if record.Owner == newAddr.String() {
			continue
		}

		oldOwner := record.Owner
		if err := ak.StakingKeeper.DeleteTokenizeShareRecord(ctx, record.Id); err != nil {
			return fmt.Errorf("delete tokenize share record %d: %w", record.Id, err)
		}

		record.Owner = newAddr.String()
		if err := ak.StakingKeeper.AddTokenizeShareRecord(ctx, record); err != nil {
			return fmt.Errorf("add tokenize share record %d with new owner: %w", record.Id, err)
		}

		ctx.Logger().Info(
			"Migrated tokenize share record owner",
			"record_id", record.Id,
			"old_owner", oldOwner,
			"new_owner", newAddr.String(),
			"validator", record.Validator,
			"old_share_token_balance", shareTokenBalance.String(),
		)
	}

	return nil
}

// migrateUnbondingDelegations rewrites all unbonding delegation records from
// oldAddr to newAddr at the store level. When the unbonding period completes,
// the tokens will be sent to newAddr instead of oldAddr.
//
// For each UBD we must update:
//   - The UBD record itself (keyed by delegator+validator)
//   - The by-validator index (keyed by validator+delegator)
//   - The unbonding queue time-slice entries (DVPair contains DelegatorAddress)
//   - The UnbondingByID index (maps unbonding ID → UBD for IBC callbacks)
func migrateUnbondingDelegations(
	ctx sdk.Context, ak *upgrades.AppKeepers,
	oldAddr sdk.AccAddress,
	oldAddress, newAddress string,
) error {
	logger := ctx.Logger()

	ubds, err := snapshotDelegatorUnbondingDelegations(ctx, ak, oldAddr)
	if err != nil {
		return fmt.Errorf("failed to get unbonding delegations: %w", err)
	}

	if len(ubds) == 0 {
		logger.Info("No unbonding delegations to migrate")
		return nil
	}

	for _, ubd := range ubds {
		if _, err := sdk.ValAddressFromBech32(ubd.ValidatorAddress); err != nil {
			return fmt.Errorf("invalid validator address %s: %w", ubd.ValidatorAddress, err)
		}

		totalBalance := math.ZeroInt()
		for _, entry := range ubd.Entries {
			totalBalance = totalBalance.Add(entry.Balance)
		}

		logger.Info("Migrating unbonding delegation",
			"validator", ubd.ValidatorAddress,
			"entries", len(ubd.Entries),
			"total_balance", totalBalance.String(),
		)

		// 1. Remove old UBD record (deletes store keys + by-val index)
		if err := ak.StakingKeeper.RemoveUnbondingDelegation(ctx, ubd); err != nil {
			return fmt.Errorf("failed to remove old unbonding delegation: %w", err)
		}

		// 2. Delete the UnbondingByID index entries for old UBD
		for _, entry := range ubd.Entries {
			if err := ak.StakingKeeper.DeleteUnbondingIndex(ctx, entry.UnbondingId); err != nil {
				return fmt.Errorf("failed to delete unbonding index %d: %w", entry.UnbondingId, err)
			}
		}

		// 3. Create new UBD with the same data but new delegator address
		newUbd := stakingtypes.UnbondingDelegation{
			DelegatorAddress: newAddress,
			ValidatorAddress: ubd.ValidatorAddress,
			Entries:          ubd.Entries,
		}

		// 4. Store new UBD record
		if err := ak.StakingKeeper.SetUnbondingDelegation(ctx, newUbd); err != nil {
			return fmt.Errorf("failed to set new unbonding delegation: %w", err)
		}

		// 5. Re-create UnbondingByID index entries for new UBD
		for _, entry := range newUbd.Entries {
			if err := ak.StakingKeeper.SetUnbondingDelegationByUnbondingID(ctx, newUbd, entry.UnbondingId); err != nil {
				return fmt.Errorf("failed to set unbonding index %d: %w", entry.UnbondingId, err)
			}
		}

		// 6. Update the unbonding queue: replace DVPair in all time-slice entries
		for _, entry := range ubd.Entries {
			if err := replaceUBDQueueEntry(ctx, ak, entry.CompletionTime, ubd.ValidatorAddress, oldAddress, newAddress); err != nil {
				return fmt.Errorf("failed to update unbonding queue for completion time %s: %w", entry.CompletionTime, err)
			}
		}

		logger.Info("Unbonding delegation migrated",
			"validator", ubd.ValidatorAddress,
			"entries", len(ubd.Entries),
			"old_delegator", oldAddress,
			"new_delegator", newAddress,
		)
	}

	return nil
}

// replaceUBDQueueEntry replaces the delegator address in a specific unbonding
// queue time-slice. The queue stores DVPair{DelegatorAddress, ValidatorAddress}
// entries grouped by CompletionTime.
func replaceUBDQueueEntry(
	ctx sdk.Context, ak *upgrades.AppKeepers,
	completionTime time.Time,
	validatorAddress string,
	oldAddress, newAddress string,
) error {
	timeSlice, err := ak.StakingKeeper.GetUBDQueueTimeSlice(ctx, completionTime)
	if err != nil {
		return err
	}

	found := false
	alreadyRewritten := false
	for i, dvPair := range timeSlice {
		if dvPair.DelegatorAddress == oldAddress && dvPair.ValidatorAddress == validatorAddress {
			timeSlice[i].DelegatorAddress = newAddress
			found = true
			// Don't break — there could be multiple entries for the same pair
		} else if dvPair.DelegatorAddress == newAddress && dvPair.ValidatorAddress == validatorAddress {
			alreadyRewritten = true
		}
	}

	if !found {
		if alreadyRewritten {
			return nil
		}
		// Queue entry may already have been processed or not exist;
		// log a warning but don't fail the upgrade.
		ctx.Logger().Warn("UBD queue entry not found",
			"completion_time", completionTime,
			"validator", validatorAddress,
			"delegator", oldAddress,
		)
		return nil
	}

	return ak.StakingKeeper.SetUBDQueueTimeSlice(ctx, completionTime, timeSlice)
}

// migrateRedelegations rewrites all redelegation records from oldAddr to
// newAddr at the store level. Redelegation records are bookkeeping entries
// that prevent double-redelegation within the unbonding period. The actual
// delegation is already at the destination validator and was migrated in step 3.
//
// For each RED we must update:
//   - The RED record itself (keyed by delegator+srcVal+dstVal)
//   - The by-src-validator index
//   - The by-dst-validator index
//   - The redelegation queue time-slice entries (DVVTriplet contains DelegatorAddress)
//   - The UnbondingByID index
func migrateRedelegations(
	ctx sdk.Context, ak *upgrades.AppKeepers,
	oldAddr sdk.AccAddress,
	oldAddress, newAddress string,
) error {
	logger := ctx.Logger()

	reds, err := snapshotDelegatorRedelegations(ctx, ak, oldAddr)
	if err != nil {
		return fmt.Errorf("failed to get redelegations: %w", err)
	}

	if len(reds) == 0 {
		logger.Info("No redelegations to migrate")
		return nil
	}

	for _, red := range reds {
		logger.Info("Migrating redelegation",
			"src_validator", red.ValidatorSrcAddress,
			"dst_validator", red.ValidatorDstAddress,
			"entries", len(red.Entries),
		)

		// 1. Remove old RED record (deletes store keys + both val indices)
		if err := ak.StakingKeeper.RemoveRedelegation(ctx, red); err != nil {
			return fmt.Errorf("failed to remove old redelegation: %w", err)
		}

		// 2. Delete the UnbondingByID index entries for old RED
		for _, entry := range red.Entries {
			if err := ak.StakingKeeper.DeleteUnbondingIndex(ctx, entry.UnbondingId); err != nil {
				return fmt.Errorf("failed to delete redelegation unbonding index %d: %w", entry.UnbondingId, err)
			}
		}

		// 3. Create new RED with the same data but new delegator address
		newRed := stakingtypes.Redelegation{
			DelegatorAddress:    newAddress,
			ValidatorSrcAddress: red.ValidatorSrcAddress,
			ValidatorDstAddress: red.ValidatorDstAddress,
			Entries:             red.Entries,
		}

		// 4. Store new RED record
		if err := ak.StakingKeeper.SetRedelegation(ctx, newRed); err != nil {
			return fmt.Errorf("failed to set new redelegation: %w", err)
		}

		// 5. Re-create UnbondingByID index entries for new RED
		for _, entry := range newRed.Entries {
			if err := ak.StakingKeeper.SetRedelegationByUnbondingID(ctx, newRed, entry.UnbondingId); err != nil {
				return fmt.Errorf("failed to set redelegation unbonding index %d: %w", entry.UnbondingId, err)
			}
		}

		// 6. Update the redelegation queue: replace DVVTriplet in all time-slice entries
		for _, entry := range red.Entries {
			if err := replaceREDQueueEntry(ctx, ak, entry.CompletionTime, red.ValidatorSrcAddress, red.ValidatorDstAddress, oldAddress, newAddress); err != nil {
				return fmt.Errorf("failed to update redelegation queue for completion time %s: %w", entry.CompletionTime, err)
			}
		}

		logger.Info("Redelegation migrated",
			"src_validator", red.ValidatorSrcAddress,
			"dst_validator", red.ValidatorDstAddress,
			"entries", len(red.Entries),
			"old_delegator", oldAddress,
			"new_delegator", newAddress,
		)
	}

	return nil
}

// replaceREDQueueEntry replaces the delegator address in a specific redelegation
// queue time-slice. The queue stores DVVTriplet{DelegatorAddress, ValidatorSrcAddress,
// ValidatorDstAddress} entries grouped by CompletionTime.
func replaceREDQueueEntry(
	ctx sdk.Context, ak *upgrades.AppKeepers,
	completionTime time.Time,
	valSrcAddress, valDstAddress string,
	oldAddress, newAddress string,
) error {
	timeSlice, err := ak.StakingKeeper.GetRedelegationQueueTimeSlice(ctx, completionTime)
	if err != nil {
		return err
	}

	found := false
	alreadyRewritten := false
	for i, triplet := range timeSlice {
		if triplet.DelegatorAddress == oldAddress &&
			triplet.ValidatorSrcAddress == valSrcAddress &&
			triplet.ValidatorDstAddress == valDstAddress {
			timeSlice[i].DelegatorAddress = newAddress
			found = true
		} else if triplet.DelegatorAddress == newAddress &&
			triplet.ValidatorSrcAddress == valSrcAddress &&
			triplet.ValidatorDstAddress == valDstAddress {
			alreadyRewritten = true
		}
	}

	if !found {
		if alreadyRewritten {
			return nil
		}
		ctx.Logger().Warn("RED queue entry not found",
			"completion_time", completionTime,
			"src_validator", valSrcAddress,
			"dst_validator", valDstAddress,
			"delegator", oldAddress,
		)
		return nil
	}

	return ak.StakingKeeper.SetRedelegationQueueTimeSlice(ctx, completionTime, timeSlice)
}
