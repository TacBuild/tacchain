# TacChain v1.6.1 — Changelog

> **Status:** Ready for release
> **Upgrade name:** `v1.6.1`
> **Release tag:** `v1.6.1` (to be cut from `main`)
> **Previous version:** v1.6.0
> **Chains:** SPB testnet — upgraded 2026-07-30 at height 23,205,042 with
> `v1.6.1-beta.1`; TacChain Mainnet — planned

---

## Summary

Maintenance release on top of v1.6.0. No state migration, no consensus-affecting
change: the handler exists only to coordinate a version bump for binary fixes.

---

## Rollout

`v1.6.1` is **not** byte-identical to the `v1.6.1-beta.1` pre-release running on SPB.

| Build | `cosmos/evm` fork | `cosmos-sdk` fork | Where |
|-------|-------------------|-------------------|-------|
| `v1.6.1-beta.1` | `v0.6.0-tac.10` | `v0.53.6-tac.3` | SPB testnet since 2026-07-30 |
| `v1.6.1` | `v0.6.0-tac.13` | `v0.53.6-tac.3` | mainnet target |

In `v1.6.1`, not in `v1.6.1-beta.1`:

| Change | Commit | Path |
|--------|--------|------|
| `tac_simulate` JSON-RPC method | `9a602662` | query |
| Balance override reaches gas estimation | `5f530858` | query |
| Static precompiles kept under an `eth_call` state override | `96d5164b` | query |
| EVM transaction broadcast to peers | `6759d93`, `548387b`, `b87bd40` | p2p |

None affects execution or consensus, so the SPB run covers the consensus-relevant
part of the release.

---

## Breaking Changes

None. No state migration, no parameter change, no consensus version bump.

---

## New Features

### `tac_simulate` (JSON-RPC)

Non-committing simulated call. Parameters mirror `eth_call` (call args, block
number or hash, state overrides). Returns in one round-trip:

```json
{ "success": true, "output": "0x...", "vmError": "", "logs": [], "gasEstimated": "0x..." }
```

- `logs` — events emitted during simulation.
- `gasEstimated` — full `eth_estimateGas` search against the same overridden state.
- A revert returns `success: false` with `vmError` and revert data in `output`,
  instead of a JSON-RPC error.

Served under the `tac` namespace, **not enabled by default** — must be listed in
`app.toml` under `[json-rpc] api`.

---

## Bug Fixes

### Vesting delegations via the EVM staking precompile

Delegating vesting-**locked** tokens silently burned coins (`amount <= spendable`)
or panicked on integer overflow (`amount > spendable`): the EVM balance handler
subtracted the full `CoinSpent` from a balance that only reflects the spendable
portion.

`x/bank` now tags `coin_spent` with `locked_amount`, and the handler subtracts
`amount - locked`. `locked > amount` is an error, not a silent clamp. `SubBalance`
guarded against underflow (upstream `cosmos/evm` #1176).

> Needs both fork bumps together — the EVM handler depends on the SDK-side emission.

### Historical EVM queries below the v1.6.0 migration height

`eth_call`, `eth_estimateGas` and traces below the v1.6.0 store migration failed
or returned wrong results — pre-migration state was decoded with the current layout:

- `x/vm` params — field numbers shifted in v1.6.0, decoding panicked with
  `wrong wireType = 2 for field HistoryServeWindow`.
- `x/erc20` precompiles — lists moved from a concatenated-blob key to per-address
  keys, so precompiles looked unregistered and calls hit the decoy ERC20 bytecode
  (`gTAC.balanceOf` at an old height returned `0`).

A height-gated shim now decodes the legacy layout on the read path; the height
comes from the applied `x/upgrade` done-height, caches warmed in `x/vm` `BeginBlock`.

> Query path only — at current heights the code path is byte-for-byte unchanged.

### State overrides dropped the static precompiles

Backport of upstream [cosmos/evm #1096](https://github.com/cosmos/evm/pull/1096),
fixed in 0.7.x and never backported to 0.6.x, so **v1.6.0 is affected**.

An `eth_call` with state overrides — an empty `{}` was enough — replaced the
precompile set with go-ethereum's stock one. Calls to `0x800` (staking), `0x801`
(distribution), `0x804` (bank) returned empty output, intrinsic-only gas, no error.

`tac_simulate` inherited it: gas estimate ~4.6x too low on precompile calls
(`delegate`: 25 390 vs 115 613). Both agree after the backport.

> Query path only — transactions never carry overrides.

### Gas estimation with a balance override

The gas-search upper bound is capped by what the sender can pay, and that balance
was read off the bank keeper, never the state override — so pricing a call for an
unfunded account returned `gasEstimated: 0` while the call itself succeeded.

The balance now comes from the override when it carries one. No override, no change.

> Only observable when the caller passes a gas price.

### EVM transaction broadcast to peers

The broadcast message was assembled by hand and refused by the receiving mempool:
no sender (`code=18`), then no `ExtensionOptionsEthereumTx` option, fee or gas
limit (`code=29`). It is now built through `MsgEthereumTx.BuildTx`, as
`SendRawTransaction` does. A duplicate from the node's own cache no longer counts
as a failure, and one bad transaction no longer aborts its batch.

Transactions still landed (a proposing node reads its own mempool) but did not
reach other validators.

> p2p propagation only, not execution.

---

## Upgrade Handler

No-op: runs module migrations and returns.

- No `StoreUpgrades`, no KV migration, no parameter change.
- No `ConsensusVersion` bump in any module.
- Vesting schedule shift and account migration were considered and dropped — not
  approved, no vesting logic ships.

---

## Documentation & Tooling

- Cosmovisor setup guide covers migrating an already running node.
- Deprecated Turin testnet (`tacchain_2390-1`) removed from `NETWORKS.md` and
  `networks/tacchain_2390-1/`.
- Localnet script lists `tac` in `json-rpc.api`, so `tac_simulate` works out of the box.
- `gen_localnode.sh` — stray `exit` cut the script short before the docker build.

---

## Dependency Versions

| Dependency | v1.6.0 | v1.6.1 |
|------------|--------|--------|
| `cosmos/evm` | fork @ `v0.6.0-tac.8` | fork @ `v0.6.0-tac.13` |
| `cosmos/cosmos-sdk` | fork @ `v0.53.6-tac.2` | fork @ `v0.53.6-tac.3` |
| `ethereum/go-ethereum` | `v1.16.2-cosmos-1` | unchanged |
| `cometbft/cometbft` | `v0.38.21` | unchanged |
| `cosmos/ibc-go/v10` | `v10.3.1` | unchanged |
| Go | 1.23.8 | unchanged |

**`cosmos/evm` `v0.6.0-tac.8` → `v0.6.0-tac.13`**

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
| `96d5164b` | Backport upstream #1096: keep the static precompiles under a state override |

**`cosmos/cosmos-sdk` `v0.53.6-tac.2` → `v0.53.6-tac.3`**

| Commit | Change |
|--------|--------|
| `7bb449d6` | `x/bank` emits `locked_amount` on `coin_spent` for vesting delegations |
