# TacChain v1.6.6 — Upgrade instructions

> **Status:** Draft; release publication and activation details are pending.
> **Upgrade plan name:** `v1.6.6`
> **Release tag:** `v1.6.6-sequirity-manual`
> **Binary version:** `1.6.6-sequirity-manual`
> **Previous release:** `v1.6.4`
> **Activation height and time:** To be announced.

This upgrade contains private security fixes and requires a coordinated switch
at the scheduled upgrade height. Follow the approved on-chain upgrade plan and
the operator announcement when they become available.

## Release binaries

Use the binaries attached to the official
[release](https://github.com/TacBuild/tacchain/releases/tag/v1.6.6-sequirity-manual)
once it is published. Builds from the public tagged source do not include the
private security fixes. The version string alone does not establish that a
binary is the official build; verify its SHA-256 checksum.

| Platform | Binary | Cosmovisor archive |
| --- | --- | --- |
| Linux amd64 | `tacchaind-linux-amd64` | `tacchaind-linux-amd64.tar.gz` |
| Linux arm64 | `tacchaind-linux-arm64` | `tacchaind-linux-arm64.tar.gz` |

Download the artifact for your architecture and `checksums.txt` from the same
release. On Linux, verify the selected file, for example:

```bash
sha256sum --check --ignore-missing checksums.txt
```

Confirm that the downloaded binary or archive is listed as `OK` before using it.
Each archive contains an executable at `bin/tacchaind`.

## Prepare the upgrade

1. Back up the node configuration, keys and data using your normal backup process.
2. Confirm that the approved upgrade plan is named exactly `v1.6.6` and check its
   activation height against the operator announcement.
3. Download and verify the official binary for your architecture.
4. Stage the new binary for the upgrade height. Keep the current binary active
   until the coordinated switch.

For Cosmovisor, use your existing `DAEMON_HOME`. The upgrade slot is
`$DAEMON_HOME/cosmovisor/upgrades/v1.6.6/bin/tacchaind`. The slot name follows the
upgrade plan name, including the leading `v`; the release tag has a different
suffix. Follow the [Cosmovisor setup guide](cosmovisor-setup.md#prepare-the-next-binary)
to stage the verified binary or archive.

For a manually managed service, switch to the verified binary when the node
halts for the approved upgrade plan, then restart it with its existing home and
configuration.

## RPC nodes: rebuild the EVM index

RPC nodes with `json-rpc.enable-indexer` enabled must rebuild `evmindexer` after
upgrading. This is an EVM index rebuild; a full chain resync is not required as
part of the upgrade.

Plan for RPC downtime during the offline rebuild. The command reads local blocks
and their execution results. Verify that the required history is retained before
moving the existing index. If that history has been pruned, arrange its recovery
before proceeding.

1. Stop the RPC node and confirm that its process has exited.
2. Back up and move aside only the `evmindexer` database, using the storage
   location and database backend configured for that node. Retain the application,
   blockstore and state databases.
3. With the verified v1.6.6 binary, rebuild the empty index from local history:

   ```bash
   /path/to/verified/tacchaind index-eth-tx forward --home "$DAEMON_HOME"
   ```

4. Wait for successful completion, then restart the node with its existing
   configuration and EVM indexer enabled.

Use the actual node home and verified binary path in the command. Running the
command against the existing index continues indexing from its last recorded
height; start with an empty EVM index for a rebuild.

## After the upgrade

- Check the running binary version and the verified artifact checksum.
- Confirm that the upgrade completed and the node is following new blocks.
- On RPC nodes, confirm that the index rebuild completed successfully and check
  JSON-RPC responses before restoring client traffic.

See the [changelog](CHANGELOG-v164-v166.md) for the release summary.
