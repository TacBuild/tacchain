# TacChain v1.6.2 — Changelog

> **Status:** Ready for release
> **Upgrade name:** `v1.6.2`
> **Release tag:** `v1.6.2` (to be cut from `main`)
> **Previous version:** v1.6.1 (code lineage)
> **Mainnet upgrade path:** v1.6.0 → v1.6.2 directly — v1.6.1 shipped to SPB
> testnet only and never reached mainnet
> **Chains:** TacChain Mainnet — planned

---

## Summary

Recovery release on top of v1.6.1. Ships the Aug-2026 incident state recovery and
the upstream `cosmos/evm` security fix that closes the exploited primitive. The
v1.6.2 upgrade handler is a no-op; the only state change is the one-shot recovery
migration, which is height- and chain-id-gated.

Because v1.6.1 only ran on SPB testnet, mainnet upgrades directly from v1.6.0. The
v1.6.2 mainnet binary therefore also carries every v1.6.1 change — see the
[v1.6.1 changelog](./CHANGELOG-v160-v161.md) for those.

---

## Breaking Changes

None. No parameter change, no `ConsensusVersion` bump. The recovery migration is a
one-shot, gated, supply-neutral state change (see below), not a consensus change.

---

## State Recovery — Aug-2026 incident

`app/hardforks/aug2026/`

The exploit drained the `bonded_tokens_pool` bank balance to ~0 while the bonded
validators still held their tokens, leaving the pool insolvent
(`bank(pool) != Σ bonded validator.tokens`). A one-shot `PreBlocker` migration
restores it:

- **Supply-neutral.** Mints the pool deficit and burns the exact same total across
  the affected accounts (`mint == burn`), then moves the remainder of a
  partially-drained account to its destination (transfer — also supply-neutral).
  Total supply is unchanged.
- **Gated.** Runs only on a chain-id present in `ParamsByChainID`, and only at that
  entry's `Height`. On any other network / height the `PreBlocker` skips it and
  touches no state.
  - Mainnet (`tacchain_239-1`): armed at height **24,671,476** (halt height + 1).
- **Guarded.** All amounts are hardcoded absolute base-unit (`utac`) values, audited
  against the halt state. The migration asserts its invariants before and after —
  pool solvency (`bank(pool) == Σ bonded validator.tokens`), the credited
  recipient, and unchanged total supply — and aborts deterministically (halt) on
  any mismatch.

> Verified on the real mainnet halt-state (resume-from-DB via `in-place-testnet`
> on the 24,671,475 snapshot): migration executed, block committed, chain
> continued, `supply_delta = 0`.

---

## Security Fix

### `cosmos/evm` GHSA hotfix (Aug-2026 primitive)

Bumps the `cosmos/evm` fork from `v0.6.0-tac.13` to `v0.6.0-tac.14`: `tac.13`
plus the upstream `cosmos/evm` security fix for the Aug-2026 incident, which
closes the phantom-balance primitive that was exploited.

---

## Upgrade Handler

No-op: runs module migrations and returns.

- No `StoreUpgrades`, no KV migration, no parameter change.
- No `ConsensusVersion` bump in any module.
- Exists only to coordinate the network version bump; all fixes are binary-level.

---

## Dependency Versions

| Dependency | v1.6.1 | v1.6.2 |
|------------|--------|--------|
| `cosmos/evm` | fork @ `v0.6.0-tac.13` | fork @ `v0.6.0-tac.14` |
| `cosmos/cosmos-sdk` | fork @ `v0.53.6-tac.3` | unchanged |
| `ethereum/go-ethereum` | `v1.16.2-cosmos-1` | unchanged |
| `cometbft/cometbft` | `v0.38.21` | unchanged |
| `cosmos/ibc-go/v10` | `v10.3.1` | unchanged |
| Go | 1.23.8 | unchanged |

Cumulative from the mainnet **v1.6.0** binary (v1.6.1 was testnet-only):
`cosmos/evm` `v0.6.0-tac.8` → `v0.6.0-tac.14`, `cosmos/cosmos-sdk`
`v0.53.6-tac.2` → `v0.53.6-tac.3`. For the intermediate `tac.8` → `tac.13`
commit list, see the [v1.6.1 changelog](./CHANGELOG-v160-v161.md).

**`cosmos/evm` `v0.6.0-tac.13` → `v0.6.0-tac.14`**

| Change |
|--------|
| Upstream GHSA hotfix: close the Aug-2026 phantom-balance primitive |
