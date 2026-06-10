# TacChain Cosmovisor Setup Guide

This guide describes how to run a TacChain node under Cosmovisor and how to
prepare for software upgrades in two modes:

1. Fully automatic binary download and upgrade.
2. Semi-automatic upgrade with the next binary prepared locally in advance.

The examples below use the SPB testnet layout:

```text
DAEMON_NAME=tacchaind
DAEMON_HOME=/root/tacchain/.testnet
CHAIN_ID=tacchain_2391-1
SERVICE_NAME=tac_node.service
```

For mainnet or another environment, replace `DAEMON_HOME`, `CHAIN_ID`, service
name, ports, and start flags with the values used by that node.

## How Cosmovisor Works

Cosmovisor is a small process manager that runs the chain binary and switches to
the next binary when the `x/upgrade` module writes an upgrade plan to:

```text
$DAEMON_HOME/data/upgrade-info.json
```

Cosmovisor stores application binaries under:

```text
$DAEMON_HOME/cosmovisor
```

Expected layout:

```text
$DAEMON_HOME/cosmovisor/
  current -> genesis or upgrades/<upgrade-name>
  genesis/
    bin/
      tacchaind
  upgrades/
    <upgrade-name>/
      bin/
        tacchaind
      upgrade-info.json
```

Important rules:

- `DAEMON_HOME` must be the same directory passed to `tacchaind --home`.
- `DAEMON_NAME` must match the binary filename: `tacchaind`.
- The upgrade directory name must match the on-chain upgrade plan name. In
  normal Cosmovisor mode, names are normalized to lowercase.
- Never replace the active `current/bin/tacchaind` with a future binary before
  the upgrade height.
- Put the future binary only under
  `$DAEMON_HOME/cosmovisor/upgrades/<upgrade-name>/bin/tacchaind`.

## Install Cosmovisor

Install a Cosmovisor version that has been tested by your team:

```bash
go install cosmossdk.io/tools/cosmovisor/cmd/cosmovisor@latest
sudo install -m 0755 "$(go env GOPATH)/bin/cosmovisor" /usr/local/bin/cosmovisor
```

Check the installed binary:

```bash
/usr/local/bin/cosmovisor version
```

## Initial Directory Setup

Set the environment for the shell session:

```bash
export DAEMON_NAME=tacchaind
export DAEMON_HOME=/root/tacchain/.testnet
```

Initialize Cosmovisor with the currently active `tacchaind` binary:

```bash
cosmovisor init /usr/local/bin/tacchaind
```

If you prefer explicit commands:

```bash
mkdir -p "$DAEMON_HOME/cosmovisor/genesis/bin"
mkdir -p "$DAEMON_HOME/cosmovisor/upgrades"
cp /usr/local/bin/tacchaind "$DAEMON_HOME/cosmovisor/genesis/bin/tacchaind"
chmod +x "$DAEMON_HOME/cosmovisor/genesis/bin/tacchaind"
ln -sfn "$DAEMON_HOME/cosmovisor/genesis" "$DAEMON_HOME/cosmovisor/current"
```

Verify:

```bash
readlink -f "$DAEMON_HOME/cosmovisor/current"
"$DAEMON_HOME/cosmovisor/current/bin/tacchaind" version
```

## Systemd Service

Create or update the service file:

```ini
[Unit]
Description=TacChain node managed by Cosmovisor
After=network-online.target
Wants=network-online.target

[Service]
User=root
WorkingDirectory=/root/tacchain
Environment="DAEMON_NAME=tacchaind"
Environment="DAEMON_HOME=/root/tacchain/.testnet"
Environment="DAEMON_RESTART_AFTER_UPGRADE=true"
Environment="DAEMON_SHUTDOWN_GRACE=30s"
Environment="UNSAFE_SKIP_BACKUP=false"
Environment="PATH=/usr/local/bin:/usr/bin:/bin"
ExecStart=/usr/local/bin/cosmovisor run start \
  --chain-id tacchain_2391-1 \
  --home /root/tacchain/.testnet \
  --json-rpc.enable \
  --json-rpc.ws-address 127.0.0.1:8546 \
  --json-rpc.address 127.0.0.1:8545
Restart=on-failure
RestartSec=10
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

Apply the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable tac_node.service
sudo systemctl restart tac_node.service
```

Useful checks:

```bash
sudo systemctl status tac_node.service --no-pager -l
sudo journalctl -u tac_node.service -n 100 --no-pager
curl -fsS http://127.0.0.1:26657/status | jq .
```

## Option 1: Fully Automatic Download and Upgrade

This mode lets Cosmovisor download the next binary from the on-chain upgrade
plan `info` field. It is convenient for full nodes. For validators, use it only
if you are comfortable with the operational risk: if the download or checksum
verification fails at upgrade height, the node will stop until fixed manually.

### Required Systemd Environment

Add these variables to the service:

```ini
Environment="DAEMON_ALLOW_DOWNLOAD_BINARIES=true"
Environment="DAEMON_DOWNLOAD_MUST_HAVE_CHECKSUM=true"
Environment="DAEMON_RESTART_AFTER_UPGRADE=true"
```

Recommended:

```ini
Environment="UNSAFE_SKIP_BACKUP=false"
```

If disk space or upgrade timing makes automatic backups impractical, operators
may set `UNSAFE_SKIP_BACKUP=true`, but that removes Cosmovisor's automatic data
backup before the switch.

Reload and restart:

```bash
sudo systemctl daemon-reload
sudo systemctl restart tac_node.service
```

### Check the Upgrade Plan Info

For automatic downloads, the accepted upgrade proposal must include a
`binaries` map in `plan.info`. Node operators do not need to create this field;
they only need to verify that the published proposal contains an entry for their
OS and CPU architecture.

Example:

```json
{
  "binaries": {
    "linux/amd64": "https://github.com/TacBuild/tacchain/releases/download/vX.Y.Z/tacchaind-linux-amd64.tar.gz?checksum=sha256:<sha256>",
    "linux/arm64": "https://github.com/TacBuild/tacchain/releases/download/vX.Y.Z/tacchaind-linux-arm64.tar.gz?checksum=sha256:<sha256>"
  }
}
```

If there is no matching entry for the node platform, use the semi-automatic
mode and place the binary locally before the upgrade height.

### Optional Pre-Download

On Cosmovisor versions that support it, operators can run:

```bash
export DAEMON_NAME=tacchaind
export DAEMON_HOME=/root/tacchain/.testnet
cosmovisor prepare-upgrade
```

This asks Cosmovisor to read the next on-chain upgrade plan, download the
matching binary, verify the checksum, and place it in the correct upgrade
directory before the upgrade height.

### What Happens at Upgrade Height

At the target height:

1. `x/upgrade` writes `$DAEMON_HOME/data/upgrade-info.json`.
2. The running `tacchaind` exits with the upgrade signal.
3. Cosmovisor reads the upgrade name and `plan.info`.
4. If the binary is not already present, Cosmovisor downloads it.
5. Cosmovisor verifies the checksum when provided or required.
6. Cosmovisor updates `current` to the new upgrade directory.
7. Cosmovisor restarts `tacchaind` with the same start flags.

## Option 2: Semi-Automatic Upgrade With Local Binary

This is the recommended validator mode when operators want to verify and place
the binary before the upgrade height. Cosmovisor still performs the switch and
restart automatically, but it does not download anything.

### Required Systemd Environment

Use:

```ini
Environment="DAEMON_ALLOW_DOWNLOAD_BINARIES=false"
Environment="DAEMON_RESTART_AFTER_UPGRADE=true"
```

Reload and restart if changed:

```bash
sudo systemctl daemon-reload
sudo systemctl restart tac_node.service
```

### Prepare the Next Binary

Set the upgrade name exactly as it appears in the on-chain upgrade plan:

```bash
export DAEMON_NAME=tacchaind
export DAEMON_HOME=/root/tacchain/.testnet
export UPGRADE_NAME=v1.6.0-spb-hotfix
```

Create the target directory:

```bash
mkdir -p "$DAEMON_HOME/cosmovisor/upgrades/$UPGRADE_NAME/bin"
```

If you have a raw binary:

```bash
cp ./tacchaind "$DAEMON_HOME/cosmovisor/upgrades/$UPGRADE_NAME/bin/tacchaind"
chmod +x "$DAEMON_HOME/cosmovisor/upgrades/$UPGRADE_NAME/bin/tacchaind"
```

If you have a downloaded archive that contains `bin/tacchaind`:

```bash
tar -xzf tacchaind-linux-amd64.tar.gz -C "$DAEMON_HOME/cosmovisor/upgrades/$UPGRADE_NAME"
chmod +x "$DAEMON_HOME/cosmovisor/upgrades/$UPGRADE_NAME/bin/tacchaind"
```

Alternatively, use Cosmovisor's helper:

```bash
cosmovisor add-upgrade "$UPGRADE_NAME" ./tacchaind
```

### Pre-Upgrade Verification

Before the upgrade height, verify both binaries:

```bash
readlink -f "$DAEMON_HOME/cosmovisor/current"
"$DAEMON_HOME/cosmovisor/current/bin/tacchaind" version
"$DAEMON_HOME/cosmovisor/upgrades/$UPGRADE_NAME/bin/tacchaind" version
```

Expected state:

```text
current/bin/tacchaind                         -> current network version
upgrades/<upgrade-name>/bin/tacchaind        -> next upgrade version
```

Do not copy the next binary into the active `current` directory. If the future
binary starts before the on-chain upgrade height, the node may stop with an
error similar to:

```text
BINARY UPDATED BEFORE TRIGGER
```

### What Happens at Upgrade Height

At the target height:

1. `x/upgrade` writes `$DAEMON_HOME/data/upgrade-info.json`.
2. The running `tacchaind` exits with the upgrade signal.
3. Cosmovisor finds the already prepared binary under
   `$DAEMON_HOME/cosmovisor/upgrades/<upgrade-name>/bin/tacchaind`.
4. Cosmovisor updates `current` to the prepared upgrade directory.
5. Cosmovisor restarts `tacchaind` with the same start flags.

## Post-Upgrade Verification

After the upgrade height, check:

```bash
export DAEMON_HOME=/root/tacchain/.testnet
export UPGRADE_NAME=v1.6.0-spb-hotfix

sudo systemctl status tac_node.service --no-pager -l
readlink -f "$DAEMON_HOME/cosmovisor/current"
"$DAEMON_HOME/cosmovisor/current/bin/tacchaind" version
curl -fsS http://127.0.0.1:26657/status | jq -r '.result.sync_info'
"$DAEMON_HOME/cosmovisor/current/bin/tacchaind" \
  --home "$DAEMON_HOME" \
  query upgrade applied "$UPGRADE_NAME" \
  --node tcp://127.0.0.1:26657 \
  --output json
```

Expected:

- `systemctl` shows the service as active.
- `current` points to `upgrades/<upgrade-name>`.
- `current/bin/tacchaind version` returns the new version.
- Latest block height is greater than the upgrade height.
- `catching_up` is `false` after the node catches up.
- `query upgrade applied <upgrade-name>` returns the applied height.

## Troubleshooting

### Node stops with `BINARY UPDATED BEFORE TRIGGER`

Most likely cause: the future binary was placed in the active `current` slot too
early, or the `current` symlink points to the future upgrade directory before
the on-chain upgrade height.

Fix:

1. Stop the service.
2. Restore the active slot binary to the current network version.
3. Keep the future binary only under `upgrades/<upgrade-name>/bin/tacchaind`.
4. Start the service again.

### Cosmovisor cannot find the upgrade binary

Check:

```bash
cat "$DAEMON_HOME/data/upgrade-info.json"
find "$DAEMON_HOME/cosmovisor/upgrades" -maxdepth 3 -type f -name tacchaind -print
```

The directory name under `upgrades/` must match the upgrade plan name expected
by Cosmovisor. Also verify executable permissions:

```bash
chmod +x "$DAEMON_HOME/cosmovisor/upgrades/<upgrade-name>/bin/tacchaind"
```

### Auto-download fails

Check:

- `DAEMON_ALLOW_DOWNLOAD_BINARIES=true`.
- The accepted proposal has a `binaries` entry for your OS and CPU architecture.
- The download URL from that entry is reachable from the node host.
- If `DAEMON_DOWNLOAD_MUST_HAVE_CHECKSUM=true`, the URL includes a checksum.
- If the published download data is missing or incorrect, switch to the
  semi-automatic mode and place the binary locally before the upgrade height.

Logs:

```bash
sudo journalctl -u tac_node.service --since "1 hour ago" --no-pager
```

### REST API is behind after upgrade

If the node process upgraded but a public API endpoint still reports the old
version or the upgrade height, the public API may be backed by another node or a
load balancer member that did not upgrade. Check the backend node info:

```bash
curl -fsS https://example-api/cosmos/base/tendermint/v1beta1/node_info | jq .
curl -fsS https://example-api/cosmos/base/tendermint/v1beta1/blocks/latest | jq .
```

## Operator Checklist

Before upgrade:

- Confirm the upgrade proposal name and target height.
- Confirm the announced target version and build tag.
- Confirm `DAEMON_HOME` equals the node `--home`.
- Confirm `DAEMON_NAME=tacchaind`.
- Confirm `current/bin/tacchaind` is still the current version.
- Confirm the next binary is either downloadable from `plan.info` or already
  placed under `upgrades/<upgrade-name>/bin/tacchaind`.
- Confirm `app.toml` changes listed in the upgrade announcement are already
  applied.

At upgrade:

- Watch logs with `journalctl -u tac_node.service -f`.
- Verify that `current` switches to `upgrades/<upgrade-name>`.
- Verify the new binary version.
- Verify the node continues past the upgrade height.

After upgrade:

- Confirm `catching_up=false`.
- Confirm RPC, REST, gRPC, and EVM JSON-RPC endpoints are healthy.
- Confirm governance and other critical module queries work through the current
  API version.

## References

- Cosmos SDK Cosmovisor README:
  https://github.com/cosmos/cosmos-sdk/blob/main/tools/cosmovisor/README.md
- Cosmos Hub node upgrade guide:
  https://docs.cosmos.network/hub/latest/hub-tutorials/upgrade-node
