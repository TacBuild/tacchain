# TacChain v1.6.4 — Changelog

> **Status:** Ready for release
> **Upgrade name:** none — no upgrade handler
> **Release tag:** `v1.6.4` (to be cut from `main`)
> **Previous version:** v1.6.3
> **Mainnet upgrade path:** v1.6.3 → v1.6.4, rolling
> **Chains:** TacChain Mainnet

---

## Summary

A single JSON-RPC read-path fix: Ethereum transactions written before the v1.6.0
migration are decodable again.

**This release needs no coordination.** There is no upgrade handler, no state
migration, no parameter change and no governance proposal. Only the way a node
answers queries about old blocks changes, so operators can restart whenever
convenient and nodes on v1.6.3 and v1.6.4 cannot diverge.

---

## Breaking Changes

None.

---

## Fix — historical Ethereum transactions

`app/legacyeth/`

Transactions written before the v1.6.0 migration carry the current type URL,
`/cosmos.evm.vm.v1.MsgEthereumTx`, but the pre-refactor field layout:

| old | new |
| --- | --- |
| 1 `Any data` (TxData) | 5 `bytes from` |
| 2 `double size` (deprecated) | 6 `EthereumTx raw` |
| 3 `string hash` | |
| 4 `string deprecated_from` | |
| 5 `bytes from` | |

The decoder resolved the type and then rejected field 1 as unknown, dropping the
transaction. Every read path that decodes one was affected:

- `eth_getBlockByNumber` returned an empty `transactions` array, with
  `transactionsRoot`, `receiptsRoot` and `logsBloom` computed as if the block were
  empty — while `gasUsed`, which comes from the header, stayed correct;
- `eth_getTransactionByHash`, `eth_getTransactionReceipt` and `eth_getBlockReceipts`
  returned null or empty;
- `eth_getLogs`, which reads the event index and needs no decoding, **kept
  returning that block's logs**.

The result was a block reporting gas used by zero transactions, with logs pointing
at a transaction the block denied having. Indexers reconciling blocks against logs
could not converge.

### How it is fixed

The decoder retries a message the current layout cannot parse through the old one
and rebuilds it in the current form.

- **Only the failing case takes the legacy path.** A current transaction decodes on
  the first attempt and never reaches it, so there is no cost on the hot path and
  no risk of misreading a modern message.
- **A reconstruction is discarded unless its hash matches** the one the legacy
  message recorded. A mismatch falls back to today's behaviour — an absent
  transaction — rather than serving a fabricated one.
- **Applied at the client context**, where all ten decode sites in the EVM backend
  share a single `TxDecoder`. The `evm` fork is unchanged.

### Verification

Confirmed on mainnet block 12,489,259 against an archive node. `eth_getBlockByNumber`,
`eth_getTransactionByHash`, `eth_getTransactionReceipt`, `eth_getBlockReceipts`,
`eth_getBlockTransactionCountByNumber`, `debug_traceTransaction` and
`debug_traceBlockByNumber` all agree, and the hash, `from` and `gas_used` match the
block explorer. Recent blocks are unaffected.

`LegacyTx` and `DynamicFeeTx` are exercised against real history. `AccessListTx`
does not appear in 6,000 blocks sampled across the pre-migration range — EIP-2930
went essentially unused on this chain — so its parser is verified against the old
proto definition rather than live data.

---

## Dependency Versions

Unchanged from v1.6.3.

| Dependency | Version |
|------------|---------|
| `cosmos/evm` | fork @ `v0.6.0-tac.15` |
| `cosmos/cosmos-sdk` | fork @ `v0.53.6-tac.3` |
| `ethereum/go-ethereum` | `v1.16.2-cosmos-1` |
| `cometbft/cometbft` | `v0.38.21` |
| `cosmos/ibc-go/v10` | `v10.3.1` |
| Go | 1.23.8 |
