package v163

import (
	"context"
	"fmt"
	"time"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/TacBuild/tacchain/app/upgrades"
)

// maxRetrieve bounds the per-account lookups below. The six compromised accounts
// hold a handful of records each; the bound only exists because the keeper API
// requires one.
const maxRetrieve = 65535

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        storetypes.StoreUpgrades{},
}

// CreateUpgradeHandler runs the Aug-2026 follow-up rescue: refund the old OFT
// escrow, redirect the compromised validators' self-unbonding stake, and release
// every remaining delegation on those validators back to its owner.
//
// THE HANDLER MUST NEVER RETURN AN ERROR FOR ANYTHING IT MIGRATES.
//
// x/upgrade turns a handler error into a panic in PreBlocker ("Returning an error
// will end up in a panic"), which halts every node at the upgrade height. This
// code is public and the accounts it touches are controlled by an adversary, so
// any abort-on-mismatch guard would be a chain-halt button costing them one utac
// (send dust to a watched address, fill an account's max_entries, ...). Instead
// every task and every individual item is isolated in a cache context and skipped
// on failure. A partial migration is recoverable in a follow-up upgrade; a halted
// chain is a coordinated restart across the whole validator set.
//
// See ai-review/163-upgrade-plan.md, section B.4.
func CreateUpgradeHandler(
	mm upgrades.ModuleManager,
	configurator module.Configurator,
	ak *upgrades.AppKeepers,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		logger := sdkCtx.Logger().With("upgrade", UpgradeName)

		if sdkCtx.ChainID() != ChainID {
			logger.Info("not the mainnet chain-id, running module migrations only",
				"chain_id", sdkCtx.ChainID(), "expected", ChainID)
			return mm.RunMigrations(ctx, configurator, fromVM)
		}

		// Order matters: B must precede C. Task C undelegates the dust shares that
		// the operator accounts still hold on TOE 3 / TOE 4, and Task B is what
		// clears those accounts in the first place.
		runTask(sdkCtx, logger, "A1-escrow-refunds", func(c sdk.Context) { taskA1(c, ak, logger) })
		runTask(sdkCtx, logger, "A2-new-oft-funding", func(c sdk.Context) { taskA2(c, ak, logger) })
		runTask(sdkCtx, logger, "B-redirect-self-stake", func(c sdk.Context) { taskB(c, ak, logger) })
		runTask(sdkCtx, logger, "C-release-delegations", func(c sdk.Context) { taskC(c, ak, logger) })

		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}

// ── Task A1 ────────────────────────────────────────────────────────────────────

// taskA1 repays the 19 in-flight-message recipients out of the old OFT escrow.
//
// The 19 amounts summed to the escrow balance exactly at audit time, but that is
// deliberately NOT asserted: anyone can send dust to the escrow and an equality
// check would become a halt trigger. Sufficiency is enough, and any residue is
// left in the escrow rather than swept somewhere this migration invented.
func taskA1(ctx sdk.Context, ak *upgrades.AppKeepers, logger log.Logger) {
	escrow := evmAddr(EscrowAddr)
	need, ok := parseInt(RefundTotal)
	if !ok {
		logger.Error("A1 skipped: bad RefundTotal constant")
		return
	}

	have := ak.BankKeeper.GetBalance(ctx, escrow, Denom).Amount
	if have.LT(need) {
		logger.Error("A1 skipped: escrow underfunded", "have", have, "need", need)
		return
	}

	paid, sent := math.ZeroInt(), 0
	for i, r := range Refunds {
		amt, ok := parseInt(r.Amount)
		if !ok {
			logger.Error("A1 item skipped: bad amount", "index", i, "to", r.To)
			continue
		}
		to := evmAddr(r.To)
		if step(ctx, logger, fmt.Sprintf("A1[%d]->%s", i, r.To), func(c sdk.Context) error {
			return ak.BankKeeper.SendCoins(c, escrow, to, sdk.NewCoins(sdk.NewCoin(Denom, amt)))
		}) {
			paid, sent = paid.Add(amt), sent+1
		}
	}

	logger.Info("A1 done", "recipients", sent, "of", len(Refunds), "paid", paid,
		"escrow_remaining", ak.BankKeeper.GetBalance(ctx, escrow, Denom).Amount)
}

// ── Task A2 ────────────────────────────────────────────────────────────────────

// taskA2 tops up the new OFT contract so it can back the ETH-side supply. The
// source is the Foundation treasury multisig, not the escrow: the escrow covers
// exactly the 19 refunds in Task A1 and nothing else.
func taskA2(ctx sdk.Context, ak *upgrades.AppKeepers, logger log.Logger) {
	amt, ok := parseInt(NewOFTFunding)
	if !ok {
		logger.Error("A2 skipped: bad NewOFTFunding constant")
		return
	}
	from, to := evmAddr(FoundationMultisigAddr), evmAddr(NewOFTAddr)

	have := ak.BankKeeper.GetBalance(ctx, from, Denom).Amount
	if have.LT(amt) {
		logger.Error("A2 skipped: source underfunded", "have", have, "need", amt)
		return
	}
	if step(ctx, logger, "A2->newOFT", func(c sdk.Context) error {
		return ak.BankKeeper.SendCoins(c, from, to, sdk.NewCoins(sdk.NewCoin(Denom, amt)))
	}) {
		logger.Info("A2 done", "amount", amt, "to", NewOFTAddr)
	}
}

// ── Task B ─────────────────────────────────────────────────────────────────────

// taskB moves every staking position held by the six compromised operator
// accounts to RedirectDest, so the self-unbonding stake cannot be claimed by
// whoever holds those keys when it matures.
//
// It sweeps the accounts rather than a fixed list of records, which is what makes
// it survive a cancelled unbonding: if the stake was pushed back into a bonded
// delegation, step 1 unbonds it again and writes the new entry straight under the
// destination. It equally covers a fresh delegation or a redelegation to some
// other validator - anything those accounts hold is swept, wherever it points.
func taskB(ctx sdk.Context, ak *upgrades.AppKeepers, logger log.Logger) {
	sk := ak.StakingKeeper
	dest := evmAddr(RedirectDest)

	unbondingTime, err := sk.UnbondingTime(ctx)
	if err != nil {
		logger.Error("B skipped: cannot read unbonding time", "err", err)
		return
	}
	completion := ctx.BlockTime().Add(unbondingTime)

	moved, rekeyed, recreated := math.ZeroInt(), 0, 0
	for _, v := range TOEValidators {
		op, err := sk.ValidatorAddressCodec().StringToBytes(v.Valoper)
		if err != nil {
			logger.Error("B: bad valoper constant", "moniker", v.Moniker, "err", err)
			continue
		}
		// The operator account is the validator address under the account prefix.
		opAcc := sdk.AccAddress(op)

		// 1. Anything still bonded: unbond it and land the entry under dest.
		dels, err := sk.GetDelegatorDelegations(ctx, opAcc, maxRetrieve)
		if err != nil {
			logger.Error("B: cannot list delegations", "moniker", v.Moniker, "err", err)
		}
		for _, d := range dels {
			var amt math.Int
			tag := fmt.Sprintf("B-unbond[%s->%s]", v.Moniker, d.ValidatorAddress)
			if step(ctx, logger, tag, func(c sdk.Context) error {
				var err error
				amt, err = unbondTo(c, ak, d, dest, completion)
				return err
			}) && !amt.IsNil() {
				moved, recreated = moved.Add(amt), recreated+1
			}
		}

		// 2. Re-key every unbonding delegation the account still owns.
		ubds, err := sk.GetUnbondingDelegations(ctx, opAcc, maxRetrieve)
		if err != nil {
			logger.Error("B: cannot list unbonding delegations", "moniker", v.Moniker, "err", err)
		}
		for _, ubd := range ubds {
			var amt math.Int
			tag := fmt.Sprintf("B-rekey[%s->%s]", v.Moniker, ubd.ValidatorAddress)
			if step(ctx, logger, tag, func(c sdk.Context) error {
				var err error
				amt, err = rekeyUBD(c, sk, ak.AccountKeeper.AddressCodec(), ubd, dest)
				return err
			}) && !amt.IsNil() {
				moved, rekeyed = moved.Add(amt), rekeyed+1
			}
		}

		// Post-condition, recorded only. A leftover means some item was skipped
		// above; it is reported, never fatal.
		if d, err := sk.GetDelegatorDelegations(ctx, opAcc, maxRetrieve); err == nil && len(d) > 0 {
			logger.Error("B: account still holds delegations", "moniker", v.Moniker, "count", len(d))
		}
		if u, err := sk.GetUnbondingDelegations(ctx, opAcc, maxRetrieve); err == nil && len(u) > 0 {
			logger.Error("B: account still holds unbonding delegations", "moniker", v.Moniker, "count", len(u))
		}
	}

	logger.Info("B done", "moved", moved, "expected_at_audit", SelfStakeExpected,
		"records_rekeyed", rekeyed, "entries_recreated", recreated, "dest", RedirectDest)
}

// ── Task C ─────────────────────────────────────────────────────────────────────

// taskC releases every remaining delegation on the six TOE validators: each one
// is unbonded in full, with the standard unbonding period, back to its own
// delegator. Nothing is redirected - these are third parties' funds.
//
// Unlike Task B this calls the stock StakingKeeper.Undelegate. These are real
// users' balances, so the well-travelled keeper path is preferable to the
// hand-built entry Task B needs, and Undelegate's max_entries check is not worth
// routing around here: a delegator can only fill their own slots, and a failure
// just leaves that one delegation in place for its owner to exit themselves.
//
// Vesting stays intact: locked coins are tracked at maturity by CompleteUnbonding
// (via UndelegateCoinsFromModuleToAccount -> TrackUndelegation), not here, so
// vesting-locked stake returns still locked on its original schedule.
func taskC(ctx sdk.Context, ak *upgrades.AppKeepers, logger log.Logger) {
	sk := ak.StakingKeeper

	unbondingTime, err := sk.UnbondingTime(ctx)
	if err != nil {
		logger.Error("C skipped: cannot read unbonding time", "err", err)
		return
	}
	completion := ctx.BlockTime().Add(unbondingTime)

	released, count := math.ZeroInt(), 0
	for _, v := range TOEValidators {
		valAddr, err := sk.ValidatorAddressCodec().StringToBytes(v.Valoper)
		if err != nil {
			logger.Error("C: bad valoper constant", "moniker", v.Moniker, "err", err)
			continue
		}
		// Store iteration order: deterministic across nodes.
		dels, err := sk.GetValidatorDelegations(ctx, valAddr)
		if err != nil {
			logger.Error("C: cannot list delegations", "moniker", v.Moniker, "err", err)
			continue
		}
		for _, d := range dels {
			owner, err := ak.AccountKeeper.AddressCodec().StringToBytes(d.DelegatorAddress)
			if err != nil {
				logger.Error("C: bad delegator address", "delegator", d.DelegatorAddress, "err", err)
				continue
			}
			var amt math.Int
			tag := fmt.Sprintf("C-unbond[%s<-%s]", v.Moniker, d.DelegatorAddress)
			if step(ctx, logger, tag, func(c sdk.Context) error {
				var err error
				_, amt, err = sk.Undelegate(c, owner, valAddr, d.Shares)
				return err
			}) && !amt.IsNil() {
				released, count = released.Add(amt), count+1
			}
		}
		if left, err := sk.GetValidatorDelegations(ctx, valAddr); err == nil && len(left) > 0 {
			logger.Error("C: validator still has delegations", "moniker", v.Moniker, "count", len(left))
		}
	}

	logger.Info("C done", "delegations_released", count, "amount", released,
		"matures", completion.UTC().Format(time.RFC3339))
}

// ── staking primitives ─────────────────────────────────────────────────────────

// unbondTo removes a delegation and writes the resulting unbonding entry under
// target - the rescue destination, not the delegator. That redirection is the
// whole point, and StakingKeeper.Undelegate cannot do it: it always writes the
// entry under the delegator.
//
// Using Unbond directly also sidesteps Undelegate's max_entries check. That
// matters here and only here: the seven entry slots belong to the compromised
// accounts, so whoever holds those keys can fill them for a handful of cheap
// transactions and block the rescue. Unbond has no such check, and
// SetUnbondingDelegationEntry does not enforce the cap either.
func unbondTo(
	ctx sdk.Context,
	ak *upgrades.AppKeepers,
	d stakingtypes.Delegation,
	target sdk.AccAddress,
	completion time.Time,
) (math.Int, error) {
	sk := ak.StakingKeeper

	valAddr, err := sk.ValidatorAddressCodec().StringToBytes(d.ValidatorAddress)
	if err != nil {
		return math.ZeroInt(), err
	}
	delAddr, err := ak.AccountKeeper.AddressCodec().StringToBytes(d.DelegatorAddress)
	if err != nil {
		return math.ZeroInt(), err
	}

	validator, err := sk.GetValidator(ctx, valAddr)
	if err != nil {
		return math.ZeroInt(), err
	}
	wasBonded := validator.IsBonded()

	// No liquid-staking bookkeeping here: LSM is off on mainnet, and all six TOE
	// validators carry liquid_shares = 0 and validator_bond_shares = 0. Note it
	// lives in the staking msgServer rather than the keeper, so keeper.Undelegate
	// would skip it too - this is not a consequence of using Unbond directly.
	amt, err := sk.Unbond(ctx, delAddr, valAddr, d.Shares)
	if err != nil {
		return math.ZeroInt(), err
	}

	// Mirrors Undelegate: a bonded validator's tokens live in the bonded pool and
	// must move across. The TOE validators are unbonding, so this is defensive.
	if wasBonded && amt.IsPositive() {
		coins := sdk.NewCoins(sdk.NewCoin(Denom, amt))
		if err := ak.BankKeeper.SendCoinsFromModuleToModule(
			ctx, stakingtypes.BondedPoolName, stakingtypes.NotBondedPoolName, coins,
		); err != nil {
			return math.ZeroInt(), err
		}
	}

	ubd, err := sk.SetUnbondingDelegationEntry(ctx, target, valAddr, ctx.BlockHeight(), completion, amt)
	if err != nil {
		return math.ZeroInt(), err
	}
	if err := sk.InsertUBDQueue(ctx, ubd, completion); err != nil {
		return math.ZeroInt(), err
	}
	return amt, nil
}

// rekeyUBD changes an unbonding delegation's owner without touching its
// completion times, so the stake still matures on its original schedule and only
// the recipient changes.
//
// The UBD key is derived from (delegator, validator), so this is a delete and
// re-insert across four structures. Miss any one of them and the payout still
// goes to the old owner, or the index dangles:
//
//  1. UnbondingDelegationKey            - the record itself
//  2. UnbondingDelegationByValIndexKey  - written by Set/RemoveUnbondingDelegation
//  3. UnbondingQueueKey                 - the completion-time slice, NOT touched
//     by the two calls above
//  4. UnbondingIndexKey                 - UnbondingId -> UBD key, embeds the owner
//
// Dependencies are narrowed to the staking keeper and an address codec so this
// can be exercised directly in unit tests - it is the only hand-written state
// surgery in the migration.
func rekeyUBD(
	ctx sdk.Context,
	sk *stakingkeeper.Keeper,
	accCodec address.Codec,
	ubd stakingtypes.UnbondingDelegation,
	dest sdk.AccAddress,
) (math.Int, error) {
	valAddr, err := sk.ValidatorAddressCodec().StringToBytes(ubd.ValidatorAddress)
	if err != nil {
		return math.ZeroInt(), err
	}
	destStr, err := accCodec.BytesToString(dest)
	if err != nil {
		return math.ZeroInt(), err
	}
	if destStr == ubd.DelegatorAddress {
		return math.ZeroInt(), nil // already ours, nothing to do
	}

	// 3. Queue slices. Keyed by nanosecond so repeated completion times across
	// entries are handled once; the surviving pairs keep their relative order, so
	// the serialised slice is identical on every node.
	done := make(map[int64]bool, len(ubd.Entries))
	for _, e := range ubd.Entries {
		key := e.CompletionTime.UnixNano()
		if done[key] {
			continue
		}
		done[key] = true

		slice, err := sk.GetUBDQueueTimeSlice(ctx, e.CompletionTime)
		if err != nil {
			return math.ZeroInt(), err
		}
		out := make([]stakingtypes.DVPair, 0, len(slice)+1)
		hasDest := false
		for _, p := range slice {
			if p.ValidatorAddress == ubd.ValidatorAddress {
				if p.DelegatorAddress == ubd.DelegatorAddress {
					continue // drop the old owner
				}
				if p.DelegatorAddress == destStr {
					hasDest = true
				}
			}
			out = append(out, p)
		}
		if !hasDest {
			out = append(out, stakingtypes.DVPair{
				DelegatorAddress: destStr,
				ValidatorAddress: ubd.ValidatorAddress,
			})
		}
		if err := sk.SetUBDQueueTimeSlice(ctx, e.CompletionTime, out); err != nil {
			return math.ZeroInt(), err
		}
	}

	// 1 + 2. Move the record. If the destination already holds one against this
	// validator, merge into it instead of overwriting - never abort.
	if err := sk.RemoveUnbondingDelegation(ctx, ubd); err != nil {
		return math.ZeroInt(), err
	}
	target := ubd
	target.DelegatorAddress = destStr
	if existing, err := sk.GetUnbondingDelegation(ctx, dest, valAddr); err == nil {
		existing.Entries = append(existing.Entries, ubd.Entries...)
		target = existing
	}
	if err := sk.SetUnbondingDelegation(ctx, target); err != nil {
		return math.ZeroInt(), err
	}

	// 4. Re-point every unbonding id at the new key.
	for _, e := range target.Entries {
		if err := sk.SetUnbondingDelegationByUnbondingID(ctx, target, e.UnbondingId); err != nil {
			return math.ZeroInt(), err
		}
	}

	moved := math.ZeroInt()
	for _, e := range ubd.Entries {
		moved = moved.Add(e.Balance)
	}
	return moved, nil
}

// ── isolation helpers ──────────────────────────────────────────────────────────

// runTask executes one task against a cache context and commits only if it
// completes. Any error or panic discards that task's writes and leaves the rest
// of the upgrade untouched.
func runTask(ctx sdk.Context, logger log.Logger, name string, fn func(sdk.Context)) {
	cached, write := ctx.CacheContext()
	if err := protect(func() { fn(cached) }); err != nil {
		logger.Error("task failed, all of its state discarded", "task", name, "err", err)
		return
	}
	write()
}

// step is runTask for a single item inside a task: a failing delegation or
// transfer is rolled back and skipped, and the loop moves on.
func step(ctx sdk.Context, logger log.Logger, name string, fn func(sdk.Context) error) bool {
	cached, write := ctx.CacheContext()
	var inner error
	if err := protect(func() { inner = fn(cached) }); err != nil {
		logger.Error("step panicked, skipped", "step", name, "err", err)
		return false
	}
	if inner != nil {
		logger.Error("step failed, skipped", "step", name, "err", inner)
		return false
	}
	write()
	return true
}

// protect turns a panic into an error. Every input here is chain state, so a
// panic occurs identically on every node and recovering from it is deterministic.
func protect(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	fn()
	return nil
}

// evmAddr converts a 0x address to its bech32-equivalent account address. TAC
// accounts are the same 20 bytes under a different encoding.
func evmAddr(hex string) sdk.AccAddress {
	return common.HexToAddress(hex).Bytes()
}

func parseInt(s string) (math.Int, bool) {
	return math.NewIntFromString(s)
}
