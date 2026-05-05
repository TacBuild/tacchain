# TacChain Technical Debt

> Tracker for known technical debt and architectural risks that don't block
> current functionality but should be addressed in future releases.

---

## 🔴 High priority

### 1. `x/epochs` module name collision with cosmos-sdk

**Status:** dormant — does not affect v1.6.0, but blocks future SDK feature adoption.

**Problem:**
TacChain currently uses the `x/epochs` module from the cosmos/evm fork
(`github.com/cosmos/evm/x/epochs`) for the LiquidStake autocompound /
rebalance scheduling. Starting from cosmos-sdk v0.53, the SDK ships its own
`x/epochs` module (`github.com/cosmos/cosmos-sdk/x/epochs`).

The two modules use **identical** `ModuleName` and `StoreKey`:

| Parameter | cosmos/evm fork (`x/epochs`) | cosmos-sdk (`x/epochs`) |
|-----------|------------------------------|--------------------------|
| `ModuleName` | `"epochs"` | `"epochs"` |
| `StoreKey` | `"epochs"` | `"epochs"` |
| Proto package | `tac.epochs.v1beta1` ✅ | `cosmos.epochs.v1beta1` |

Proto packages are different (tac fork was renamed to avoid registry collision),
so currently **everything works** because only one of the two epochs modules
is wired in `app/app.go`.

**Risk:**
If in the future we need to add the SDK's `x/epochs` (or any third-party module
that depends on it transitively and registers it), the chain will panic at
startup:
- `NewKVStoreKeys(...)` will panic on duplicate store key
- ModuleManager will refuse registration of two modules with the same name

**Mitigation options:**
1. **Rename our fork's epochs** — change `ModuleName` and `StoreKey` to
   `"tacepochs"` (or `"evmepochs"`). This is a **store-key migration** that
   requires copying all state from the old key prefix to the new one in an
   upgrade handler. Should be done before any need to wire SDK epochs.
2. **Don't adopt SDK epochs** — explicitly avoid any feature/module that
   requires it. Document this constraint in onboarding docs.
3. **Replace with SDK epochs entirely** — drop the fork's version, port
   liquidstake hooks to use SDK's `EpochHooks` interface. State migration still
   needed, but ends up with mainline cosmos-sdk module — less long-term debt.

**Recommended:** option 3 long-term, deferred until needed. Until then —
document the constraint (this file).

---

## 🟡 Medium priority

### 2. EVM Chain ID source deviates from upstream

**Status:** intentional deviation, working correctly.

In upstream cosmos/evm v0.6.0, `net_version` and several other RPC backends
read EVM Chain ID from `app.toml` (`cfg.EVM.EVMChainID`). TacChain sources it
from `evmtypes.GetChainConfig().ChainId` (set by the VM keeper from genesis
params) instead, to keep `app.toml` backward-compatible with v1.0.4
operators (no required config change).

**Affected file:** `evm/rpc/namespaces/ethereum/net/api.go` (TacBuild commit
`fe067892`).

**Risk:** carrying a non-trivial deviation from upstream — if upstream ever
refactors the chain ID lookup, our patch may need rework. Cherry-pick conflict
risk on future upstream merges.

**Mitigation options:**
1. Keep deviation, monitor upstream for refactors.
2. Adopt upstream behaviour and require all node operators to set
   `evm-chain-id = 2390` in `app.toml`. Requires a coordinated config update,
   communication to all validator/RPC operators, and potentially a config
   migration helper command.

**Recommended:** keep deviation for v1.6.0; reconsider in v2.x if upstream
makes further chain-ID-related refactors.

---

## 🟢 Low priority / nice to have

### 3. Adopt `x/protocolpool` for community pool

**Status:** not adopted; current behaviour preserved via `x/distribution`.

cosmos-sdk v0.53 introduces `x/protocolpool` as the standard community-pool
mechanism. TacChain v1.6.0 explicitly does **not** wire it — `x/distribution`
keeps managing community pool funds. This means:
- `x/distribution` query `CommunityPool` and msgs `CommunityPoolSpend` /
  `FundCommunityPool` continue to work
- TacChain misses out on protocolpool features (continuous funds,
  enabled-distribution-denoms, etc.)

**Risk:** none today, but cosmos-sdk may eventually deprecate community-pool
APIs in `x/distribution`.

**Recommended:** evaluate adoption in v1.7.x+ when there's a concrete need
(e.g. continuous funding for ecosystem grants).

### 4. Consider `Unordered Transactions` (SDK v0.53 feature)

**Status:** not enabled.

SDK v0.53 added unordered tx support via `authkeeper.WithUnorderedTransactions(true)`.
Useful for parallel transaction submission patterns (e.g. high-throughput
indexers, market-makers). Not enabled in v1.6.0.

**Recommended:** enable when a concrete user need is identified.

---

## See also

- [`docs/project-context.md`](./project-context.md) — full project context
- [`docs/CHANGELOG-v104-v160.md`](./CHANGELOG-v104-v160.md) — v1.6.0 changelog
- [`docs/upgrade-proto-migration.md`](./upgrade-proto-migration.md) — KV/proto migration analysis
