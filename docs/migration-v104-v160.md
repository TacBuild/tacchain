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

##### `[json-rpc]`

```toml
# WebSocket origin whitelist.
# "*" allows all origins (default for permissive nodes).
# Restrict to specific domains for production validators.
ws-origins = "*"

# Allow unsigned / legacy unprotected transactions via JSON-RPC.
# This is now a LOCAL node policy only (removed from on-chain params).
allow-unprotected-txs = false

# Enable the debug namespace profiling endpoints.
enable-profiling = false

# Maximum number of requests in a single batch RPC call.
max-batch-request-len = 1000

# Maximum number of responses returned by a batch RPC call.
max-batch-response-len = 1000
```

##### `[geth-metrics]`

```toml
[geth-metrics]
# Enable Geth-compatible metrics endpoint.
enable = false
# Address for the Geth metrics HTTP server.
address = "127.0.0.1:8100"
```

##### `[evm]` (formerly `[json-rpc]` overlap)

```toml
[evm]
# Number of blocks in the block hash history window (EIP-2935).
history-serve-window = 8192
```

---

### WebSocket Warning

If you expose WebSocket endpoints publicly, set `ws-origins` to a restrictive list.
Leaving it as `"*"` allows cross-origin WebSocket connections from any domain.

```toml
[json-rpc]
ws-origins = "https://yourdomain.com"
```

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
