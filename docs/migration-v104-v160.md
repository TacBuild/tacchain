# TacChain v1.6.0 — Migration Guide

> **Status:** DRAFT
> **Upgrade name:** `v1.6.0`
> **Previous version:** v1.0.4
>
> 📋 **Full changelog:** [CHANGELOG-v104-v160.md](CHANGELOG-v104-v160.md)

---

## Node Operator Migration Guide

### On-chain Upgrade Process

1. Wait for the upgrade halt at the governance-approved block height.
2. Stop `tacchaind`.
3. Replace the binary with the v1.6.0 build.
4. **Update `app.toml`** — see the section below. In particular, `evm-chain-id` is **required**.
5. Start `tacchaind` — the upgrade handler runs automatically on the first block.

No manual state migration is needed. All KV store migrations run inside the upgrade handler.

---

### `app.toml` Changes

#### Removed fields

| Key | Reason |
|-----|--------|
| `[json-rpc] fix-revert-gas-refund-height` | Removed upstream — no longer needed |

Remove this line if present in your existing `app.toml`:

```toml
# REMOVE THIS LINE — no longer supported in v1.6.0:
fix-revert-gas-refund-height = 0
```

#### Required fields

> ⚠️ **`evm-chain-id` must be set correctly.** Starting from v1.6.0 the node reads
> the EVM chain ID directly from `app.toml`. If the field is absent or set to `0`,
> the node falls back to the upstream default `262144` instead of the correct TacChain
> value. This will silently break EIP-155 replay protection, `eth_chainId`, and
> transaction signing for all EVM clients.

Add to the `[evm]` section the value matching your network:

```toml
[evm]
# EIP-155 replay-protection chain ID. Must match the numeric part of the Cosmos chain ID.
# TacChain mainnet:  tacchain_239-1   →  evm-chain-id = 239
# TacChain testnet:  tacchain_2391-1  →  evm-chain-id = 2391
evm-chain-id = 239
```

#### Added / Changed fields

The following fields are **new in v1.6.0**. They will be missing from your existing
`app.toml` and must be added manually (or regenerate the file with `tacchaind init`).

##### `[grpc]`

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

##### `[evm]`

```toml
[evm]
# ... existing fields unchanged ...

# Minimum priority fee (tip) for EVM transactions (in wei).
min-tip = 0

# Address for the geth-compatible metrics HTTP server.
geth-metrics-address = "127.0.0.1:8100"

# Number of blocks in the block hash history window (EIP-2935).
history-serve-window = 8192
```

##### `[evm.mempool]` — new section

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

##### `[json-rpc]`

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

# Allow unsigned / legacy unprotected transactions via JSON-RPC.
# This is now a LOCAL node policy only (removed from on-chain params).
allow-unprotected-txs = false

# Enable profiling endpoints in the `debug` namespace.
# WARNING: do NOT enable on public-facing nodes.
enable-profiling = false
```

---

### WebSocket Warning

The new `ws-origins` field defaults to `["127.0.0.1", "localhost"]`, which means
WebSocket connections from external hosts will be **rejected by default**.

If your node exposes WebSocket RPC to external clients (e.g., dApps, indexers,
wallets), you must explicitly add their origins:

```toml
ws-origins = ["127.0.0.1", "localhost", "myapp.example.com", "*"]
```

Use `"*"` to allow all origins (equivalent to the old behavior).

---

## Developer Migration Guide

### EVM RPC Changes

#### Removed RPC params / fields

| Field / Param | Change |
|---------------|--------|
| `allow_unprotected_txs` | Removed from `x/vm` on-chain params. Still configurable per-node via `app.toml`. |
| `chain_config` | Removed from `x/vm` on-chain params. Managed internally by the VM keeper. |
| `fix_revert_gas_refund_height` | Removed from JSON-RPC config. |

#### New RPC methods

| Method | Description |
|--------|-------------|
| `eth_createAccessList` | Estimate access list for a transaction |
| `debug_traceCall` | Trace a call without broadcasting it |

#### State overrides in `eth_call`

`eth_call` now supports a `stateOverrides` parameter:

```json
{
  "balance": "0xde0b6b3a7640000",
  "nonce": 1,
  "code": "0x...",
  "stateDiff": { "0x...slot": "0x...value" }
}
```

---

### Precompile Changes

#### Method renames / removals

| Old | New | Address |
|-----|-----|---------|
| `Run()` | `Execute()` | all stateful precompiles |
| `increaseAllowance` | *(removed)* | ERC20 precompile |
| `decreaseAllowance` | *(removed)* | ERC20 precompile |
| `BalanceChangeEntry` | *(removed)* | — |
| Evidence precompile | *(removed, address preserved)* | `0x...0807` |

#### authz removed

All precompiles previously using `x/authz` for delegated calls have been refactored.
If your contract calls staking / distribution / gov precompiles via `authz`, update
your integration — direct calls now work without authz grants.

#### Gov precompile — new methods

- `submitProposal(...)` — submit a governance proposal
- `deposit(proposalId, amount)` — deposit to a proposal
- `cancelProposal(proposalId)` — cancel own proposal
- `constitution()` — query the chain constitution

#### LiquidStake — gTAC ERC20 events

The `x/liquidstake` module now emits ERC20 `Transfer(address,address,uint256)` events
when gTAC is minted (on stake) or burned (on unstake). Indexers and dApps can now
track gTAC supply changes through standard ERC20 event filters.

---

### x/params removal

The `x/params` keeper has been removed from the app. If your custom module stores
parameters via `x/params` (legacy subspace pattern), you must migrate to native
param storage (`ParamStore` in the keeper itself) before or as part of the v1.6.0
upgrade handler.

### x/crisis removal

The `x/crisis` module and `MsgVerifyInvariant` transaction type are no longer available.
Any tooling that submits invariant-checking transactions should be updated accordingly.
