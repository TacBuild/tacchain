# TacChain v1.6.1 — Changelog

> **Status:** DRAFT
> **Upgrade name:** `v1.6.1`
> **Previous version:** v1.6.0
> **Chain:** TacChain Mainnet

---

## Summary

TacChain v1.6.1 is a maintenance release on top of v1.6.0. It carries **no state
migration**: the upgrade handler exists only so the network can coordinate a
version bump to ship binary-level fixes.

Everything in this release comes from the `cosmos/evm` and `cosmos-sdk` forks:

- Delegating vesting-locked tokens through the EVM staking precompile no longer
  burns coins or panics
- Historical EVM queries below the v1.6.0 store-migration height work again
- New `tac_simulate` JSON-RPC method

---

## Breaking Changes

None. No consensus-affecting change, no state migration, no parameter change.

---

## New Features

### RPC

- **`tac_simulate`** — a simulated, non-committing call that reports in one
  round-trip what `eth_call`, `eth_estimateGas` and a log inspection would take
  three of:

  ```json
  {
    "success": true,
    "output": "0x...",
    "vmError": "",
    "logs": [],
    "gasEstimated": "0x..."
  }
  ```

  Parameters mirror `eth_call`: call args, block number or hash, and state
  overrides.

  - **Event logs** emitted during the simulation are returned in `logs`.
  - **`gasEstimated`** is a full `eth_estimateGas` binary search run against the
    *same* overridden state as the call, so an estimate can be produced for state
    that does not exist on chain yet.
  - **A revert is a result, not an error.** Where `eth_call` turns a revert into
    a JSON-RPC error, `tac_simulate` returns `success: false` together with
    `vmError` and the revert data in `output`, so a caller can decode a custom
    error instead of just seeing the request fail.

  Served under the **`tac` namespace, which is not enabled by default** — it has
  to be listed in `app.toml` under `[json-rpc] api` to be reachable.

---

## Bug Fixes

### Vesting delegations via the EVM staking precompile

Delegating vesting-**locked** tokens through the staking precompile either
silently burned coins (when `amount <= spendable`) or aborted with an
integer-overflow panic (when `amount > spendable`).

The EVM balance handler subtracted the full `CoinSpent` amount from an EVM
balance that only ever reflects the *spendable* portion, while a delegation from
a vesting account is drawn from the locked portion and does not reduce spendable
at all.

`x/bank` now tags `coin_spent` with a `locked_amount` attribute
(`cosmos-sdk` fork), and the handler subtracts only the spendable part
(`amount - locked`). An inconsistent event where `locked > amount` is now an
error rather than a silent clamp. `SubBalance` is additionally guarded against
underflow (upstream `cosmos/evm` #1176).

> Requires both fork bumps together — the EVM-side handler depends on the
> `locked_amount` emission from the SDK side.

### Historical EVM queries below the v1.6.0 migration height

`eth_call`, `eth_estimateGas` and traces at heights **below** the v1.6.0 store
migration failed or returned wrong results, because pre-migration state was
being decoded with the current, incompatible layout:

- **`x/vm` params** — proto field numbers shifted in v1.6.0 (old field 10
  `active_static_precompiles` vs. new `history_serve_window`), so decoding
  panicked with `wrong wireType = 2 for field HistoryServeWindow`.
- **`x/erc20` precompiles** — the native/dynamic lists moved from a single
  concatenated-blob key to per-address keys, so precompiles looked unregistered
  at historical heights and calls executed the decoy ERC20 bytecode instead of
  the precompile. In practice `gTAC.balanceOf` at an old height returned `0`
  instead of the real balance.

A lazy, height-gated shim now decodes the legacy layout on the read path below
the migration height. The height is resolved from the applied `x/upgrade`
done-height and cached; the caches are warmed in the `x/vm` `BeginBlock`.

> **Query path only.** At current heights the code path is byte-for-byte
> unchanged, so this is not consensus-affecting.

### Gas estimation with a balance override

The upper bound of the gas estimation binary search is capped by what the sender
can pay for gas, and that balance was read straight off the bank keeper — the
state override never reached it. Pricing a call for an account that is not funded
yet therefore returned no estimate at all: the call itself succeeded on the
overridden state while `gasEstimated` came back `0`.

The balance now comes from the override when it carries one for the sender, the
way go-ethereum reads it off the overridden state. Without an override nothing
changes, so `eth_estimateGas` keeps its behaviour.

> Only observable when the caller passes a gas price; otherwise the fee cap
> defaults to `0` and the recap is skipped entirely.

---

## Upgrade Handler Details

`v1.6.1` is a **no-op state upgrade**. The handler runs the standard module
migrations and returns:

- No `StoreUpgrades` — no store added, renamed or deleted.
- No KV migration.
- No parameter change.
- No `ConsensusVersion` bump in any module.

A vesting schedule shift and an account migration were considered for this
release and **dropped** — they were not approved — so no vesting logic ships in
the handler.

---

## Documentation & Tooling

- Cosmovisor setup guide now covers migrating an already running node.
- Deprecated Turin testnet (`tacchain_2390-1`) removed: `NETWORKS.md` section and
  the `networks/tacchain_2390-1/` directory (genesis, compose file, env).
- The localnet script now lists `tac` in `json-rpc.api`, so `tac_simulate` is
  reachable out of the box on a locally initialised chain.

---

## Dependency Versions

| Dependency | v1.6.0 | v1.6.1 |
|------------|--------|--------|
| `cosmos/evm` | fork @ `v0.6.0-tac.8` | fork @ `v0.6.0-tac.12` |
| `cosmos/cosmos-sdk` | fork @ `v0.53.6-tac.2` | fork @ `v0.53.6-tac.3` |
| `ethereum/go-ethereum` | `v1.16.2-cosmos-1` | unchanged |
| `cometbft/cometbft` | `v0.38.21` | unchanged |
| `cosmos/ibc-go/v10` | `v10.3.1` | unchanged |
| Go | 1.23.8 | unchanged |

### Fork changes in detail

**`cosmos/evm` `v0.6.0-tac.8` → `v0.6.0-tac.12`**

| Commit | Change |
|--------|--------|
| `6fac41af` | Allow delegating locked vesting tokens via the staking precompile |
| `a585881b` | Tests for vesting-locked delegation via the staking precompile |
| `993cc964` | Decode pre-migration EVM state for historical queries |
| `436c16e0` | Warm the legacy-decode height cache from live state |
| `9d115d6c` | Move the per-block cache warming into the `x/vm` `BeginBlock` |
| `5796b415` | Guard nil `erc20Keeper` in `BeginBlock` cache warming |
| `9a602662` | Add the `tac_simulate` JSON-RPC method |
| `5f530858` | Let a balance override reach the gas estimation |

**`cosmos/cosmos-sdk` `v0.53.6-tac.2` → `v0.53.6-tac.3`**

| Commit | Change |
|--------|--------|
| `7bb449d6` | `x/bank` emits `locked_amount` on `coin_spent` for vesting delegations |
