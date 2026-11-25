# TacChain v1.6.6 — Changelog

> **Status:** Draft; release not published.
> **Upgrade name:** `v1.6.6`
> **Release tag:** `v1.6.6-sequirity-manual`
> **Previous release:** `v1.6.4`
> **Upgrade path:** Coordinated software upgrade.
> **Activation height and time:** To be announced.

## Summary

This release contains private security fixes delivered in manually built
release binaries.

## Upgrade handler

- Registers the `v1.6.6` upgrade handler.
- Runs standard module migrations and updates the active precompile list.
- Preserves the remaining EVM parameters.

Operators must switch binaries at the approved upgrade height. Follow the
[upgrade instructions](README-v166.md) when the activation details are announced.

## Operator requirements

- Use the official Linux amd64 or arm64 release binaries and verify their SHA-256
  checksums against the release's `checksums.txt`.
- Public source builds do not include the private security fixes.
- RPC nodes using `json-rpc.enable-indexer` must rebuild `evmindexer` after the
  upgrade. A full chain resync is not part of the upgrade procedure.
- Cosmovisor archives contain `bin/tacchaind`; use the upgrade slot `v1.6.6`.

## Validation

- Application and upgrade package tests passed.
- Handler tests cover removal of a retired precompile and the case where it is
  already absent, while preserving the other EVM parameters.
- Linux amd64 and arm64 binaries were built with the private security fixes.
- Embedded binary version and commit metadata were checked.
- Archive contents and uploaded artifact SHA-256 checksums were verified.
