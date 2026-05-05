# TacChain v1.6.0 — Changelog & Migration Guide

> **Status:** DRAFT
> **Upgrade name:** `v1.6.0`
> **Previous version:** v1.0.4
> **Chain:** TacChain Mainnet

---

## Table of Contents

1. [Summary](#summary)
2. [Breaking Changes](#breaking-changes)
3. [New Features](#new-features)
4. [Bug Fixes & Security](#bug-fixes--security)
5. [Node Operator Migration Guide](#node-operator-migration-guide)
   - [app.toml Changes](#apptom-changes)
   - [On-chain Upgrade Process](#on-chain-upgrade-process)
6. [Developer Migration Guide](#developer-migration-guide)
   - [EVM RPC Changes](#evm-rpc-changes)
   - [Precompile Changes](#precompile-changes)
7. [Upgrade Handler Details](#upgrade-handler-details)
8. [Dependency Versions](#dependency-versions)

---

## Summary

TacChain v1.6.0 is a major upgrade that replaces the EVM stack from the internal
evmos-based `cosmos/evm v0.1.4` to the upstream `cosmos/evm v0.6.0`, and bumps
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

---

## Breaking Changes

### EVM

- **go-ethereum v1.10 → v1.16**: three major Geth version bumps with new EVM opcodes,
  new transaction types, and updated JSON-RPC alignment.
- **EIP-7702** (Set EOA account code): delegated accounts now supported.
- **EIP-2935** (Block hash history): historical block hashes stored on-chain.
  New param `history_serve_window` (default: 8192 blocks).
- **`allow_unprotected_txs` removed** from `x/vm` params (#415). This is no longer
  a consensus-level setting — it now lives only in `app.toml` under
  `[json-rpc] allow-unprotected-txs` as a **local node policy** controlling
  whether the JSON-RPC endpoint accepts non-EIP-155 (unprotected) transactions.
  Replay protection itself is enforced by clients signing with the proper
  `chainID` (all modern wallets do this by default).
- **`chain_config` removed** from `x/vm` params. Chain config is now derived
  from genesis and managed by the VM keeper.
- **Single EVM tx per Cosmos tx** is now enforced.

### Precompiles

- **`Run()` → `Execute()`**: all stateful precompiles now implement `Execute()` instead of `Run()`.
- **authz removed from all precompiles**: staking, distribution, and gov precompiles
  no longer use `x/authz` for delegation on behalf of others.
- **`increaseAllowance` / `decreaseAllowance` removed** (deprecated ERC20 methods, #472).
- **Evidence precompile removed** (address `0x...0807`) — no use cases found.
  The address is preserved in the precompile registry for backwards compatibility
  but the implementation is removed.
- **`BalanceChangeEntry` removed** (#506).

### IBC

- **`feeibc` store removed**: the `feeibc` store key is deleted during upgrade
  (artifact of the transitive ibc-go bump pulled in by `cosmos/evm v0.6.0`).
  TacChain does not use IBC, so no user-facing impact.

### Configuration (`app.toml`)

Several new fields have been added to `app.toml`. See the
[Node Operator Migration Guide](#node-operator-migration-guide) for details.

**Removed field:**
- `[json-rpc] fix-revert-gas-refund-height` — removed, no longer needed.

### Cosmos SDK

- **v0.50 → v0.53**: major SDK upgrade.

---

## New Features

### EVM

- **App-side EVM mempool** (geth-compatible): full transaction pool with priority
  ordering, nonce gap handling, and configurable limits. Replaces the previous
  simple FIFO mempool for EVM transactions.
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

- **ERC20 mint/burn event for gTAC**: `x/liquidstake` now emits ERC20-compatible events
  when gTAC (liquid staked TAC) is minted or burned.

### RPC

- **WebSocket origin whitelist** (`ws-origins` in `app.toml`): configurable list
  of allowed origins for WebSocket connections.
- **`enable-profiling`** flag for `debug` namespace (disabled by default).

---

## Bug Fixes & Security

### Security

- **Sherlock audit** completed with 5 batches of post-audit fixes.
- **ISA-2025-004** (Partial Precompile State Writes): fixed via journal-based
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

## Node Operator Migration Guide

### app.toml Changes

Please update your `app.toml` with the new fields described below.

#### `[grpc]` section — new field

```toml
[grpc]
# ... existing fields unchanged ...

# Historical gRPC addresses with block ranges for historical query routing.
# Maps gRPC addresses to block ranges so queries for old heights can be
# transparently forwarded to a node running an older binary.
# Format: '{"address1": [start_block, end_block], "address2": [start_block, end_block]}'
# Leave as "{}" to disable.
historical-grpc-address-block-range = "{}"
```

#### `[evm]` section — new fields

> **`evm-chain-id` is REQUIRED.** This field must be present in your
> `app.toml` — the node will fail to start without it. Set it to `239` for
> TacChain mainnet (matches the EIP-155 replay-protection chain ID used by
> the EVM).

```toml
[evm]
# ... existing fields unchanged ...

# EIP-155 replay-protection chain ID.
evm-chain-id = 239

# Minimum priority fee (tip) for EVM transactions (in wei).
min-tip = 0

# Address for the geth-compatible metrics HTTP server.
geth-metrics-address = "127.0.0.1:8100"
```

#### `[evm.mempool]` section — NEW

This section is new and was not present in v1.0.4. It configures the app-side
EVM transaction mempool (geth-compatible).

```toml
[evm.mempool]
# Minimum gas price to enforce for acceptance into the pool (in wei).
price-limit = 1

# Minimum price bump percentage to replace an already existing transaction (same nonce).
price-bump = 10

# Number of executable transaction slots guaranteed per account.
account-slots = 16

# Maximum number of executable transaction slots for all accounts combined.
global-slots = 5120

# Maximum number of non-executable (queued) transaction slots per account.
account-queue = 64

# Maximum number of non-executable (queued) transaction slots for all accounts combined.
global-queue = 1024

# Maximum time a non-executable transaction stays in the queue before eviction.
lifetime = "3h0m0s"
```

#### `[json-rpc]` section — new fields

```toml
[json-rpc]
# ... existing fields unchanged ...

# Maximum number of requests allowed in a single JSON-RPC batch call.
batch-request-limit = 1000

# Maximum response size (in bytes) for a batched JSON-RPC call.
batch-response-max-size = 25000000

# Allowed origins for WebSocket connections.
# ⚠️  If you serve WebSocket connections from external clients, add their
# origins here. The default restricts WS to localhost only.
ws-origins = ["127.0.0.1", "localhost"]

# Enable profiling endpoints in the `debug` namespace.
# WARNING: do NOT enable on public-facing nodes.
enable-profiling = false
```

#### `[json-rpc]` section — removed field

Remove this line if present in your existing `app.toml`:

```toml
# REMOVE THIS LINE — no longer supported in v1.6.0:
fix-revert-gas-refund-height = 0
```

### ⚠️ WebSocket Origin Warning

The new `ws-origins` field defaults to `["127.0.0.1", "localhost"]`, which means
WebSocket connections from external hosts will be **rejected by default**.

If your node exposes WebSocket RPC to external clients (e.g., dApps, indexers,
wallets), you must explicitly add their origins:

```toml
ws-origins = ["127.0.0.1", "localhost", "myapp.example.com", "*"]
```

Use `"*"` to allow all origins (equivalent to the old behavior).

### On-chain Upgrade Process

The upgrade handler runs automatically when the chain reaches the upgrade height.
It performs the following steps (no manual action required):

1. **Vesting account rescue** — migrates a compromised `PeriodicVestingAccount`
   to a new safe address.
2. **x/vm Params re-encoding** — fixes protobuf field number shifts between
   cosmos/evm v0.1.4 and v0.6.0.
3. **x/vm EvmCoinInfo initialisation** — writes new KV key required by v0.6.0.
4. **x/erc20 Params update** — writes new flag-based param keys.
5. **x/erc20 native precompile migration** — migrates precompile address storage
   from old concatenated blob format to per-address KV keys.
6. **`feeibc` store cleanup** — deleted store key from ibc-go v8 (no longer used).

---

## Developer Migration Guide

### EVM RPC Changes

#### New Methods
| Method | Description |
|--------|-------------|
| `eth_createAccessList` | Generate access list for a transaction |
| `debug_traceCall` | Trace a call without broadcasting |
| `eth_call` with `stateOverrides` | Simulate with modified state |

#### Removed Methods
Non-standard (non-geth) JSON-RPC methods have been removed. Use standard geth-compatible methods.

#### Batch Requests
Batch RPC calls are now limited:
- Max requests per batch: 1000 (configurable via `batch-request-limit`)
- Max response size: 25 MB (configurable via `batch-response-max-size`)

### Precompile Changes

#### Interface Change: `Run()` → `Execute()`
All stateful precompiles now implement the `Execute()` method instead of `Run()`.
If you have custom precompiles or call precompiles directly in Solidity, the ABI
interface itself is unchanged — this is an internal Go implementation change only.

#### Removed Precompile: Evidence (`0x...0807`)
The Evidence precompile has been removed. Calls to this address will fail.

#### authz No Longer Required
Staking, distribution, and gov precompiles no longer require an `x/authz` grant
to act on behalf of another account. The `CallerAddress` is used directly.

#### New Precompile Features
- **Gov**: `submitProposal(...)`, `deposit(...)`, `cancelProposal(...)`, `constitution()`
- **Distribution**: `depositValidatorRewardsPool(...)`, `communityPool()`
- **Staking**: full validator description fields in query responses

#### LiquidStake Precompile
Address unchanged: `0x0000000000000000000000000000000000001600`

New behavior: staking via the liquidstake precompile emits ERC20-compatible
`Transfer` events for gTAC (the liquid staking token), enabling better
indexer and dApp integration.

#### Ed25519 Precompile
Address unchanged: `0x00000000000000000000000000000000000008f3`

Gas costs updated:
- Base gas: 1500 → 2000
- SHA512 per-word gas: 8 → 12
- Input offset for gas calculation: fixed (now correctly excludes method selector)

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
| `cosmos/evm` | fork @ `b1c973f1` (evmos-based v0.1.4) | fork @ v0.6.0 + TacBuild patches |
| `cosmos/cosmos-sdk` | fork @ v0.50.15 | fork @ v0.53.6 + TacBuild LSM patches |
| `ethereum/go-ethereum` | v1.10.x | v1.16.x |
| Go | 1.23.6 | 1.23.8 |
