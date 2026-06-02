# TacChain v1.6.0 — Changelog

> **Status:** DRAFT
> **Upgrade name:** `v1.6.0`
> **Previous version:** v1.0.4
> **Chain:** TacChain Mainnet
>
> 📋 **Migration guide for node operators and developers:** [migration-v104-v160.md](migration-v104-v160.md)

---

## Summary

TacChain v1.6.0 is a major upgrade that replaces the EVM stack from the internal
evmos-based `cosmos/evm v0.2.0` to the upstream `cosmos/evm v0.6.0`, and bumps
Cosmos SDK from `v0.50.15` to `v0.53.6`.

The upgrade includes:
- Full EVM stack replacement (284 upstream commits in cosmos/evm)
- go-ethereum upgraded from v1.10 to v1.16
- Cosmos SDK upgraded to v0.53.6
- App-side EVM mempool (geth-compatible)
- New EVM precompile features and fixes
- EIP-7702 (delegated accounts) and EIP-2935 (block hash history) support
- On-chain KV state migration for x/vm and x/erc20
- Vesting account rescue (migration of compromised account to new safe address)
- Removal of deprecated `x/params` and `x/crisis` keepers

---

## Breaking Changes

### EVM

- **go-ethereum v1.10 → v1.16**: three major Geth version bumps with new EVM opcodes,
  new transaction types, and updated JSON-RPC alignment.
- **EIP-7702** (Set EOA account code): delegated accounts now supported.
- **EIP-2935** (Block hash history): historical block hashes stored on-chain.
  New param `history_serve_window` (default: 8192 blocks).
- **`allow_unprotected_txs` removed** from `x/vm` params. This is no longer
  a consensus-level setting — it now lives only in `app.toml` under
  `[json-rpc] allow-unprotected-txs` as a **local node policy**.
- **`chain_config` removed** from `x/vm` params. Chain config is now derived
  from genesis and managed by the VM keeper.
- **Single EVM tx per Cosmos tx** is now enforced.

### Precompiles

- **`Run()` → `Execute()`**: all stateful precompiles now implement `Execute()` instead of `Run()`.
- **authz removed from all precompiles**: staking, distribution, and gov precompiles
  no longer use `x/authz` for delegation on behalf of others.
- **`increaseAllowance` / `decreaseAllowance` removed** (deprecated ERC20 methods).
- **Evidence precompile removed** (address `0x...0807`) — no use cases found.
  The address is preserved in the precompile registry for backwards compatibility
  but the implementation is removed.
- **`BalanceChangeEntry` removed**.

### IBC

- **`feeibc` store removed**: the `feeibc` store key is deleted during upgrade
  (artifact of the transitive ibc-go bump pulled in by `cosmos/evm v0.6.0`).
  TacChain does not use IBC fee middleware, so no user-facing impact.

### Cosmos SDK

- **v0.50 → v0.53**: major SDK upgrade.
- **`x/params` keeper removed**: the legacy `x/params` keeper, store keys
  (`params`, `params_transient`) and module entry are removed from the app.
  All SDK modules migrated to native param storage in v0.47+; the `x/params`
  store was confirmed empty on mainnet prior to this upgrade.
- **`x/crisis` keeper removed**: the `CrisisKeeper` and the `x/crisis` module
  are removed. Periodic on-chain invariant checking is deprecated upstream
  and was never active on TacChain mainnet.

### Configuration (`app.toml`)

- `[json-rpc] fix-revert-gas-refund-height` **removed** — no longer needed.
- Several new fields added — see the [Migration Guide](migration-v104-v160.md).

---

## New Features

### EVM

- **App-side EVM mempool** (geth-compatible): full transaction pool with priority
  ordering, nonce gap handling, and configurable limits.
- **`eth_createAccessList`** RPC method.
- **State overrides in `eth_call`**: simulate calls with modified state.
- **`debug_traceCall`** RPC method.
- **Block time in derived logs**.
- **Batch RPC request/response size limits** (configurable).
- **Geth metrics on a separate HTTP server** (default `127.0.0.1:8100`).

### Precompiles

- **Gov precompile**: `submitProposal`, `deposit`, `cancelProposal`, `constitution` query.
- **Distribution precompile**: `DepositValidatorRewardsPool`, `CommunityPool`.
- **Staking precompile**: full validator description from queries.
- **Journal-based revert** for precompile state changes (security fix for
  partial state writes, ISA-2025-004).
- **Cached precompile ABIs** for better performance.
- **Static precompiles builder** — precompile registration refactored.

### LiquidStake (TacBuild)

- **ERC20 mint/burn event for gTAC**: `x/liquidstake` now emits ERC20-compatible
  `Transfer` events when gTAC is minted or burned, enabling better indexer and
  dApp integration.

### RPC

- **WebSocket origin whitelist** (`ws-origins` in `app.toml`).
- **`enable-profiling`** flag for `debug` namespace (disabled by default).

---

## Bug Fixes & Security

### Security

> The security audits and fixes listed below were performed **upstream** in
> `cosmos/evm v0.6.0` and are inherited by TacChain as part of this upgrade.

- **Sherlock audit** completed (upstream cosmos/evm) with 5 batches of post-audit fixes.
- **ISA-2025-004** (Partial Precompile State Writes): fixed upstream via journal-based
  revert — precompile state changes are now fully atomic.
- **Single EVM tx per Cosmos tx** enforcement prevents certain attack vectors.

### Bug Fixes

- Fix non-determinism in EVM state transitions.
- Fix race conditions in the EVM mempool.
- Fix inconsistent block hash in JSON-RPC responses.
- Fix wrong `TransactionIndex` in transaction receipts.
- Fix `debug_traceTransaction` block height mismatch.
- Fix gas estimation for new transaction types (EIP-7702, etc.).
- Fix nil pointer panics (BaseFee=0, evmCoinInfo not initialised, etc.).
- Fix ERC20 IBC middleware sender validation.
- Fix indexer service shutdown.

---

## Upgrade Handler Details

| File | Description |
|------|-------------|
| `app/upgrades/v1.6.0/upgrades.go` | Main handler — orchestrates all migration steps |
| `app/upgrades/v1.6.0/evm_params_migration.go` | Raw protobuf re-encoding for x/vm Params + ERC20 native precompile address migration |
| `app/upgrades/v1.6.0/vesting_account_migration.go` | Compromised vesting account rescue |

### KV Store Changes

| Store | Change |
|-------|--------|
| `x/vm` | Params bytes re-encoded (field number shift fix). New key `{0x05}` (EvmCoinInfo) initialised. |
| `x/erc20` | Params migrated to flag-keys. Native precompiles migrated from blob to per-address keys. |
| `feeibc` | Store deleted entirely (ibc-go v8 → v10 removal). |
| `x/params` | Store keys removed from app. Store was empty on mainnet. |
| `x/crisis` | Store key removed from app. |

### ConsensusVersion Handling

The old evmos-based cosmos/evm set high ConsensusVersions:
- `evm`: 9 (old) → 1 (new)
- `erc20`: 4 (old) → 1 (new)
- `feemarket`: 5 (old) → 1 (new)

`RunMigrations` skips modules where `toVersion <= 1`. The upgrade handler
explicitly sets `fromVM["evm"] = fromVM["erc20"] = fromVM["feemarket"] = 0`
to make this intent clear and ensure correct behavior.

---

## Dependency Versions

| Dependency | v1.0.4 | v1.6.0 |
|------------|--------|--------|
| `cosmos/evm` | fork @ `b1c973f1` (evmos-based v0.2.0) | fork @ v0.6.0 + TacBuild patches |
| `cosmos/cosmos-sdk` | fork @ v0.50.15 | fork @ v0.53.6 + TacBuild LSM patches |
| `ethereum/go-ethereum` | v1.10.x | v1.16.x |
| Go | 1.23.6 | 1.23.8 |
