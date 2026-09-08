# TacChain v1.6.3 — Changelog

> **Status:** In development
> **Upgrade name:** `v1.6.3`
> **Release tag:** `v1.6.3` (to be cut from `main`)
> **Previous version:** v1.6.2
> **Mainnet upgrade path:** v1.6.2 → v1.6.3
> **Chains:** TacChain Mainnet — planned for **2026-09-14**

---

## Summary

Follow-up release to the Aug-2026 incident. Two independent parts:

1. A **rescue state migration** in the `v1.6.3` upgrade handler: it repays the TAC
   stranded on the old OFT escrow, funds the new OFT, redirects the self-unbonding
   stake of six compromised validators to a TAC-controlled account, and releases
   every remaining delegation on those validators back to its owner.
2. The remaining **`cosmos/evm` hardening** deferred from v1.6.2, shipped as
   `v0.6.0-tac.15`.

The migration is time-critical: the compromised validators' self-unbonding matures
on **2026-09-23 15:35 UTC**, after which the funds would be liquid in accounts an
unauthorised party can sign for.

---

## Breaking Changes

None. No parameter change, no `ConsensusVersion` bump.

---

## Rescue Migration — `v1.6.3` upgrade handler

`app/upgrades/v1.6.3/`

Gated on chain-id `tacchain_239-1`; on any other network the handler runs the
module migrations and returns. All addresses and amounts are compiled-in absolute
values, audited against mainnet state at height 24,993,121.

| Task | What it does |
|------|--------------|
| **A1** | Repays the **19 in-flight-message recipients** from the old OFT escrow `0x1219c409…` — **34,354,103.50003 TAC**, matching the escrow balance to the wei. |
| **A2** | Transfers **7,628,902.102546 TAC** from the Foundation treasury multisig to the new OFT `0x3B56716b…` to back the ETH-side supply. |
| **B** | Moves the compromised validators' self-unbonding stake — **19,979,613.401007161764310628 TAC**, 6 records / 7 entries — to `0xf610Ea93…`. **Completion times are unchanged**: the stake still matures on its original schedule, only the recipient changes. |
| **C** | Undelegates **all 995 delegations** on the six validators in full, with the normal 21-day period, back to **each delegator's own account**. Nothing is redirected. Vesting-locked stake returns still locked. |

Order is fixed: **A1 → A2 → B → C**. Task C undelegates dust the operator accounts
hold on two of the validators, and Task B is what clears those accounts first.

### The handler cannot fail

Unlike the v1.6.2 recovery migration, this one **never returns an error**.

`x/upgrade` turns a handler error into a panic in `PreBlocker`, halting every node
at the upgrade height. The recovery migration could afford abort-on-mismatch: it
ran on an already-halted chain with no adversary. This one runs on a live chain,
its code is public, and it touches accounts an adversary controls — so any
exact-match guard would be a chain-halt trigger costing them a single `utac` (send
dust to a watched address, fill an account's `max_entries`, …).

Instead:

- every task and every individual item runs in its own `CacheContext`, committed
  only on success, with panics recovered;
- balance checks are **sufficiency tests with a safe else-branch**, never equality;
- expected totals are emitted as log/event **observations**, not assertions.

A partial migration is recoverable in a follow-up upgrade; a halted chain is a
coordinated restart across the whole validator set. Verification moves off-chain,
to a post-upgrade check at M+1.

### Task B: redirect mechanics

`UnbondingDelegation` records are keyed by `(delegator, validator)`, so changing
the owner is a delete-and-reinsert across four structures — the record, the
by-validator index, the completion-time queue slice, and the `UnbondingId` index.
Missing any one of them would either pay the old owner or leave a dangling index.

Task B **sweeps the six compromised accounts** rather than a fixed list of records,
so it still works if the stake was moved before the upgrade: anything found still
bonded is unbonded and written straight under the destination, which also covers a
fresh delegation or a redelegation to another validator.

It uses `Unbond` + `SetUnbondingDelegationEntry` rather than `Undelegate`, because
`Undelegate` can only write the entry under the delegator, and its `max_entries`
check operates on slots the compromised accounts control. Task C, dealing with
third-party funds, uses the stock `Undelegate` and skips on error.

---

## Security Fix

### `cosmos/evm` hardening (deferred from v1.6.2)

Bumps the `cosmos/evm` fork from `v0.6.0-tac.14` to `v0.6.0-tac.15`:

| Change |
|--------|
| Bech32 address-length guard — skip non-20-byte addresses when mirroring balances, closing a double-credit path (TAC-original; reported upstream) |
| Atomic `StateDB.Commit` — stage through a `CacheContext` and write only on success |
| Precompile out-of-gas semantics — return raw `vm.ErrOutOfGas` and bind the recover closure after the new gas meter is installed (upstream #1049) |
| Feemarket `EndBlock` — clamp `gasWanted` / `gasUsed` to `MaxInt64` instead of failing the block (upstream #1259) |

---

## Upgrade Handler

Runs the rescue migration described above, then the standard module migrations.

- No `StoreUpgrades`, no parameter change.
- No `ConsensusVersion` bump in any module.

---

## Dependency Versions

| Dependency | v1.6.2 | v1.6.3 |
|------------|--------|--------|
| `cosmos/evm` | fork @ `v0.6.0-tac.14` | fork @ `v0.6.0-tac.15` |
| `cosmos/cosmos-sdk` | fork @ `v0.53.6-tac.3` | unchanged |
| `ethereum/go-ethereum` | `v1.16.2-cosmos-1` | unchanged |
| `cometbft/cometbft` | `v0.38.21` | unchanged |
| `cosmos/ibc-go/v10` | `v10.3.1` | unchanged |
| Go | 1.23.8 | unchanged |
