# Tac Networks

| Chain ID        | Type      | Status     | Version  | Notes         |
|-----------------|-----------|------------|----------|---------------|
| tacchain_239-1 | `mainnet` | **Active** | `v1.0.0`         | Mainnet |
| tacchain_2391-1 | `testnet` | **Active** | `v0.0.12`         | Saint Petersburg Testnet |
| tacchain_2390-1 | `testnet` | **Active** | `v0.0.7-testnet` | Turin Testnet            |

# Mainnet (`tacchain_239-1`)

| Chain ID                    | `tacchain_239-1`                                                                             |
|-----------------------------|-----------------------------------------------------------------------------------------------|
| Tacchaind version           | `v1.0.0`                                                                                      |
| RPC                         | <https://tendermint.rpc.tac.build>                                                                                           |
| Genesis                     | <https://tendermint-rest.rpc.tac.build/genesis>                                                                                           |
| gRPC                        | <https://grpc.rpc.tac.build>                                                                                           |
| REST API                    | <https://cosmos-api.rpc.tac.build>                                                                                           |
| EVM JSON RPC                | <https://rpc.tac.build>                                                                                           |
| EVM Explorer                | <https://evm.explorer.tac.build/>                                                                                           |
| Cosmos Explorer             | <https://ping.explorer.tac.build/>                                                                                           |
| Staking UI                  | <https://staking.tac.build/>                                                           |
| Timeout commit | 1s                                                                                            |
| Block time | 2s                                                                                            |
| Minimum gas price           | 25000000000utac                                                                                            |
| Peer 1                      | d0a80c43a10a6b60475864728db6d9ba4ead42d2@107.6.113.60:58960                                                                                           |
| Peer 2                      | 10550a03e4f7fa487c78fbd07e0770e2b0f085c7@64.46.115.78:58960                                                                                           |
| Peer 3                      | 0efae9d157f0ef60ad7d25507d6939799f832e34@69.4.239.26:58960                                                                                           |
| Peer 4                      | 78079166d06e345dbf4a5c932ee3c69a04148e92@107.6.91.38:58960                                                                                           |
| Snapshots                   | http://snapshot.tac.ankr.com/tac-{mainnet,spb,turin}-{full,archive}-latest.{tar.lz4,shasum}   |
| - full                      | http://snapshot.tac.ankr.com/tac-mainnet-full-latest.tar.lz4                                  |
| - archive                   | http://snapshot.tac.ankr.com/tac-mainnet-archive-latest.tar.lz4                               |
### Hardware Requirements

  - CPU: 8 cores
  - RAM: 16GB (rpc) / 32GB (validator)
  - SSD: 500GB NVMe

### Join Tac Mainnet Manually

This example guide connects to mainnet. You can replace `chain-id`, `persistent_peers`, `genesis url` with the network you want to join. `--home` flag specifies the path to be used. The example will create [.mainnet](.mainnet) folder.

#### Prerequisites

  - [Go >= 1.23.6](https://go.dev/doc/install)
  - jq
  - curl

#### 1. Install `tacchaind` [v1.0.0](https://github.com/TacBuild/tacchain/tree/v1.0.0)

``` shell
git checkout v1.0.0
make install
```

#### 2. Initialize network folder

In this example our node moniker will be `testnode`, don't forget to name your own node differently.

``` sh
tacchaind init testnode --chain-id tacchain_239-1 --home .mainnet
```

#### 3. Modify your [config.toml](.mainnet/config/config.toml)

- config.toml:
``` toml
..
persistent_peers = "d0a80c43a10a6b60475864728db6d9ba4ead42d2@107.6.113.60:58960,10550a03e4f7fa487c78fbd07e0770e2b0f085c7@64.46.115.78:58960,0efae9d157f0ef60ad7d25507d6939799f832e34@69.4.239.26:58960,78079166d06e345dbf4a5c932ee3c69a04148e92@107.6.91.38:58960"
..
```

#### 4. Fetch genesis

``` sh
curl https://raw.githubusercontent.com/TacBuild/tacchain/refs/heads/main/networks/tacchain_239-1/genesis.json > .mainnet/config/genesis.json
```

#### 5. Start node

``` shell
tacchaind start --chain-id tacchain_239-1 --home .mainnet
```

### Join Tac Mainnet Using Official Snapshots

This example guide connects to mainnet. You can replace `chain-id`, `persistent_peers`, `genesis url` with the network you want to join. `--home` flag specifies the path to be used. The example will create [.mainnet](.mainnet) folder.

#### Prerequisites

  - [Go >= v1.23.6](https://go.dev/doc/install)
  - jq
  - curl
  - tar
  - lz4
  - wget

#### 1. Install `tacchaind` [v1.0.0](https://github.com/TacBuild/tacchain/tree/v1.0.0)

``` shell
git checkout v1.0.0
make install
```

#### 2. Initialize network folder

In this example our node moniker will be `testnode`, don't forget to name your own node differently.

``` sh
tacchaind init testnode --chain-id tacchain_239-1 --home .mainnet
```

#### 3. Modify your [config.toml](.mainnet/config/config.toml)

- config.toml:
``` toml
..
persistent_peers = "d0a80c43a10a6b60475864728db6d9ba4ead42d2@107.6.113.60:58960,10550a03e4f7fa487c78fbd07e0770e2b0f085c7@64.46.115.78:58960,0efae9d157f0ef60ad7d25507d6939799f832e34@69.4.239.26:58960,78079166d06e345dbf4a5c932ee3c69a04148e92@107.6.91.38:58960"
..
```

#### 4. Fetch genesis

``` sh
curl https://raw.githubusercontent.com/TacBuild/tacchain/refs/heads/main/networks/tacchain_239-1/genesis.json > .mainnet/config/genesis.json
```

#### 5. Fetch snapshot

``` sh
cd .mainnet
rm -rf data
wget http://snapshot.tac.ankr.com/tac-mainnet-full-latest.tar.lz4
lz4 -dc < tac-mainnet-full-latest.tar.lz4 | tar -xvf -
```

#### 6. Start node

``` shell
tacchaind start --chain-id tacchain_239-1 --home .mainnet
```

### Join Tac Mainnet Using Docker

#### Prerequisites

  - [Go >= v1.23.6](https://go.dev/doc/install)
  - jq
  - curl
  - lz4
  - docker
  - docker compose

``` shell
export TAC_HOME="~/.tacchain"
export VERSION="v1.0.0"

git clone https://github.com/TacBuild/tacchain.git && cd tacchain
mkdir -p $TAC_HOME
cp networks/tacchain_239-1/{docker-compose.yaml,.env.mainnet} $TAC_HOME/
git checkout ${VERSION}
docker build -t tacchain:${VERSION} .
cd $TAC_HOME
wget http://snapshot.tac.ankr.com/tac-mainnet-full-latest.tar.lz4
wget http://snapshot.tac.ankr.com/tac-mainnet-full-latest.shasum
shasum -c tac-mainnet-full-latest.shasum
lz4 -dc < tac-mainnet-full-latest.tar.lz4 | tar -xvf -
docker compose --env-file=.env.mainnet up -d
## Test
curl -L localhost:45138 -H "Content-Type: application/json" -d '{"jsonrpc": "2.0","method": "eth_blockNumber","params": [],"id": 1}'
```

Assuming all is working you can now proceed from "Join as a validator”

### Join Tac Mainnet as a validator

NOTE: The provided examples use `--keyring-backend test`. This is not recommended for production validator nodes. Please use `os` or `file` for encryption features and more advanced security.

#### 1. Make sure you followed one of our join guides above and have a fully synced running node before you proceed

#### 2. Make sure you have imported a funded account into your tacchaind wallet

- Check `tacchaind keys --help` for more information

- Note: the next steps of this guide assume you have named your imported funded private key as "validator"

#### 3. Send `MsgCreateValidator` transaction

1. Generate tx json file

In this example our moniker is `testnode` as named when we initialized our node. Don't forget to replace with your node moniker.

``` sh
echo "{\"pubkey\":$(tacchaind --home .mainnet tendermint show-validator),\"amount\":\"1000000000000000000utac\",\"moniker\":\"testnode\",\"identity\":null,\"website\":null,\"security\":null,\"details\":null,\"commission-rate\":\"0.1\",\"commission-max-rate\":\"0.2\",\"commission-max-change-rate\":\"0.01\",\"min-self-delegation\":\"1\"}" > validatortx.json
```

2. Broadcast tx

``` sh
tacchaind --home .mainnet tx staking create-validator validatortx.json --from validator --keyring-backend test --gas 400000 --gas-prices 100000000000utac -y
```

#### 4. Delegate more tokens (optional)

In this example our moniker is `testnode` as named when we initialized our node. Don't forget to replace with your node moniker.

``` sh
tacchaind --home .mainnet tx staking delegate $(tacchaind --home .mainnet q staking validators --output json | jq -r '.validators[] | select(.description.moniker == "testnode") | .operator_address') 1000000000000000000utac --keyring-backend test --from validator --gas 400000 --gas-prices 100000000000utac -y
```

### Tac Mainnet Validator Sentry Node Setup

Validators are responsible for ensuring that the network can sustain denial of service attacks.

One recommended way to mitigate these risks is for validators to carefully structure their network topology in a so-called sentry node architecture.

Validator nodes should only connect to full-nodes they trust because they operate them themselves or are run by other validators they know socially. A validator node will typically run in a data center. Most data centers provide direct links to the networks of major cloud providers. The validator can use those links to connect to sentry nodes in the cloud. This shifts the burden of denial-of-service from the validator's node directly to its sentry nodes, and may require new sentry nodes be spun up or activated to mitigate attacks on existing ones.

Sentry nodes can be quickly spun up or change their IP addresses. Because the links to the sentry nodes are in private IP space, an internet based attack cannot disturb them directly. This will ensure validator block proposals and votes always make it to the rest of the network.

To setup your sentry node architecture you can follow the instructions below:

#### 1. Initialize a new config folder for the sentry node on a new machine with tacchaind binary installed

`tacchaind init <sentry_node_moniker> --chain-id tacchaind_239-1 --default-denom utac`

- NOTE: This will initialize config folder in $HOME/.tacchaind

- NOTE: Make sure you have replaced your genesis file with the one for Tac Mainnet. Example script to download it:
`curl https://raw.githubusercontent.com/TacBuild/tacchain/refs/heads/main/networks/tacchain_239-1/genesis.json > .mainnet/config/genesis.json` 

#### 2. Update `config.toml` for sentry node

`private_peer_ids` field is used to specify peers that will not be gossiped to the outside world, in our case the validator node we want it to represent. Example: `private_peer_ids = "3e16af0cead27979e1fc3dac57d03df3c7a77acc@3.87.179.235:26656"`

``` toml
..
persistent_peers = "d0a80c43a10a6b60475864728db6d9ba4ead42d2@107.6.113.60:58960,10550a03e4f7fa487c78fbd07e0770e2b0f085c7@64.46.115.78:58960,0efae9d157f0ef60ad7d25507d6939799f832e34@69.4.239.26:58960,78079166d06e345dbf4a5c932ee3c69a04148e92@107.6.91.38:58960"
..
private_peer_ids = "<VALIDATOR_PEER_ID>@<VALIDATOR_IP:PORT>
..
```

- NOTE: Make sure you add persistent peers as described in previous steps for validator setup

#### 3. Update `config.toml` for validator node

Using the sentry node setup, our validator node will be represented by our sentry node, therefore it no longer has to be connected with other peers. We will replace `persistent_peers` so it points to our sentry node, this way it can no longer be accessed by the outter world. We will also disable `pex` field.

```toml
..
persistent_peers = <SENTRY_NODE_ID>@<SENTRY_NODE_IP:PORT>
..
pex = false
..
```

#### 4. Restart validator node and start sentry node.

# Saint Petersburg Testnet (`tacchain_2391-1`)

| Chain ID                    | `tacchain_2391-1`                                                                             |
|-----------------------------|-----------------------------------------------------------------------------------------------|
| Tacchaind version           | `v0.0.12`                                                                                      |
| RPC                         | <https://spb.tendermint.rpc.tac.build>                                                                                           |
| Genesis                     | <https://spb.tendermint-rest.rpc.tac.build/genesis>                                                                                           |
| gRPC                        | <https://spb-grpc.rpc.tac.build>                                                                                           |
| REST API                    | <https://spb.cosmos-api.rpc.tac.build>                                                                                           |
| EVM JSON RPC                | <https://spb.rpc.tac.build>                                                                                           |
| Faucet                      | <https://spb.faucet.tac.build/>                                                                                           |
| EVM Explorer                | <https://spb.explorer.tac.build/>                                                                                           |
| Cosmos Explorer             | <https://pp-explorer.tac-spb.tac.build/>                                                                                           |
| Staking UI                  | <https://staking.spb.tac.build/>                                                           |
| Timeout commit | 1s                                                                                            |
| Block time | 2s                                                                                            |
| Minimum gas price           | 25000000000utac                                                                                            |
| Peer 1                      | 9c32b3b959a2427bd2aa064f8c9a8efebdad4c23@206.217.210.164:45130                                                                                           |
| Peer 2                      | 04a2152eed9f73dc44779387a870ea6480c41fe7@206.217.210.164:45140                                                                                           |
| Peer 3                      | 5aaaf8140262d7416ac53abe4e0bd13b0f582168@23.92.177.41:45110                                                                                           |
| Peer 4                      | ddb3e8b8f4d051e914686302dafc2a73adf9b0d2@23.92.177.41:45120                                                                                           |
| Snapshots                   | http://snapshot.tac.ankr.com/tac-{mainnet,spb,turin}-{full,archive}-latest.{tar.lz4,shasum}   |
| - full                      | http://snapshot.tac.ankr.com/tac-spb-full-latest.tar.lz4                                      |
| - archive                   | http://snapshot.tac.ankr.com/tac-spb-archive-latest.tar.lz4                                   |

### Hardware Requirements

  - CPU: 8 cores
  - RAM: 16GB (rpc) / 32GB (validator)
  - SSD: 500GB NVMe

### Join Tac Saint Petersburg Testnet Manually

This example guide connects to testnet. You can replace `chain-id`, `persistent_peers`, `genesis url` with the network you want to join. `--home` flag specifies the path to be used. The example will create [.testnet](.testnet) folder.

#### Prerequisites

  - [Go >= 1.23.6](https://go.dev/doc/install)
  - jq
  - curl

#### 1. Install `tacchaind` [v0.0.8](https://github.com/TacBuild/tacchain/tree/v0.0.8)

``` shell
git checkout v0.0.8
make install
```

#### 2. Initialize network folder

In this example our node moniker will be `testnode`, don't forget to name your own node differently.

``` sh
tacchaind init testnode --chain-id tacchain_2391-1 --home .testnet
```

#### 3. Modify your [config.toml](.testnet/config/config.toml)

- config.toml:
``` toml
..
persistent_peers = "9c32b3b959a2427bd2aa064f8c9a8efebdad4c23@206.217.210.164:45130,04a2152eed9f73dc44779387a870ea6480c41fe7@206.217.210.164:45140,5aaaf8140262d7416ac53abe4e0bd13b0f582168@23.92.177.41:45110,ddb3e8b8f4d051e914686302dafc2a73adf9b0d2@23.92.177.41:45120"
..
```

#### 4. Fetch genesis

``` sh
curl https://raw.githubusercontent.com/TacBuild/tacchain/refs/heads/main/networks/tacchain_2391-1/genesis.json > .testnet/config/genesis.json
```

#### 5. Start node

``` shell
tacchaind start --chain-id tacchain_2391-1 --home .testnet
```

#### 6. Upgrade binary to v0.0.9 and restart

At block height 872601 your node should halt and throw error `"UPGRADE \"v0.0.9\" NEEDED at height: 872601: add eth_getBlockReceipts, disable x/feemarket, remove wasmd"`. Now you need to stop your node, upgrade binary and restart.

``` shell
git checkout v0.0.9
make install
tacchaind start --chain-id tacchain_2391-1 --home .testnet
```

#### 7. Upgrade binary to v0.0.10 and restart

At block height 939826 your node should halt and throw error `"UPGRADE \"v0.0.10\" NEEDED at height: 939826: enable x/feemarket tx fee checker"`. Now you need to stop your node, upgrade binary and restart.

``` shell
git checkout v0.0.10
make install
tacchaind start --chain-id tacchain_2391-1 --home .testnet
```

#### 8. Upgrade binary to v0.0.12 and restart

At block height 1297619 your node should halt and throw error `"UPGRADE \"v0.0.11\" NEEDED at height: 1297619: allow non-EOA to stake via evm staking precompile and force 0 inflation"`. Now you need to stop your node, upgrade binary and restart. Note that that the error states v0.0.11, but we are switching to v0.0.12 instead - it includes a non-state breaking change and also includes v0.0.11 upgrade.

``` shell
git checkout v0.0.12
make install
tacchaind start --chain-id tacchain_2391-1 --home .testnet
```

### Join Tac Saint Petersburg Testnet Using Official Snapshots

This example guide connects to testnet. You can replace `chain-id`, `persistent_peers`, `genesis url` with the network you want to join. `--home` flag specifies the path to be used. The example will create [.testnet](.testnet) folder.

#### Prerequisites

  - [Go >= v1.23.6](https://go.dev/doc/install)
  - jq
  - curl
  - tar
  - lz4
  - wget

#### 1. Install `tacchaind` [v0.0.11](https://github.com/TacBuild/tacchain/tree/v0.0.11)

``` shell
git checkout v0.0.11
make install
```

#### 2. Initialize network folder

In this example our node moniker will be `testnode`, don't forget to name your own node differently.

``` sh
tacchaind init testnode --chain-id tacchain_2391-1 --home .testnet
```

#### 3. Modify your [config.toml](.testnet/config/config.toml)

- config.toml:
``` toml
..
persistent_peers = "9c32b3b959a2427bd2aa064f8c9a8efebdad4c23@206.217.210.164:45130,04a2152eed9f73dc44779387a870ea6480c41fe7@206.217.210.164:45140,5aaaf8140262d7416ac53abe4e0bd13b0f582168@23.92.177.41:45110,ddb3e8b8f4d051e914686302dafc2a73adf9b0d2@23.92.177.41:45120"
..
```

#### 4. Fetch genesis

``` sh
curl https://raw.githubusercontent.com/TacBuild/tacchain/refs/heads/main/networks/tacchain_2391-1/genesis.json > .testnet/config/genesis.json
```

#### 5. Fetch snapshot

``` sh
cd .testnet
rm -rf data
wget http://snapshot.tac.ankr.com/tac-spb-full-latest.tar.lz4
lz4 -dc < tac-spb-full-latest.tar.lz4 | tar -xvf -
```

#### 6. Start node

``` shell
tacchaind start --chain-id tacchain_2391-1 --home .testnet
```

### Join Tac Saint Petersburg Testnet Using Docker

#### Prerequisites

  - [Go >= v1.23.6](https://go.dev/doc/install)
  - jq
  - curl
  - lz4
  - docker
  - docker compose

``` shell
export TAC_HOME="~/.tacchain"
export VERSION="v0.0.11"

git clone https://github.com/TacBuild/tacchain.git && cd tacchain
mkdir -p $TAC_HOME
cp networks/tacchain_2391-1/{docker-compose.yaml,.env.spb} $TAC_HOME/
git checkout ${VERSION}
docker build -t tacchain:${VERSION} .
cd $TAC_HOME
wget http://snapshot.tac.ankr.com/tac-spb-full-latest.tar.lz4
wget http://snapshot.tac.ankr.com/tac-spb-full-latest.shasum
shasum -c tac-spb-full-latest.shasum
lz4 -dc < tac-spb-full-latest.tar.lz4 | tar -xvf -
docker compose --env-file=.env.spb up -d
## Test
curl -L localhost:45138 -H "Content-Type: application/json" -d '{"jsonrpc": "2.0","method": "eth_blockNumber","params": [],"id": 1}'
```

Assuming all is working you can now proceed from "Join as a validator”

### Join Tac Saint Petersburg Testnet as a validator

NOTE: The provided examples use `--keyring-backend test`. This is not recommended for production validator nodes. Please use `os` or `file` for encryption features and more advanced security.

#### 1. Make sure you followed one of our join guides above and have a fully synced running node before you proceed

#### 2. Fund account and import key

1. You can use the [faucet](https://spb.faucet.tac.build/) to get funds.

2. Export your metamask private key

3. Import private key using the following command. Make sure to replace `<PRIVATE_KEY>` with your funded private key.

``` sh
tacchaind --home .testnet keys unsafe-import-eth-key validator <PRIVATE_KEY> --keyring-backend test
```

#### 3. Send `MsgCreateValidator` transaction

1. Generate tx json file

In this example our moniker is `testnode` as named when we initialized our node. Don't forget to replace with your node moniker.

``` sh
echo "{\"pubkey\":$(tacchaind --home .testnet tendermint show-validator),\"amount\":\"1000000000000000000utac\",\"moniker\":\"testnode\",\"identity\":null,\"website\":null,\"security\":null,\"details\":null,\"commission-rate\":\"0.1\",\"commission-max-rate\":\"0.2\",\"commission-max-change-rate\":\"0.01\",\"min-self-delegation\":\"1\"}" > validatortx.json
```

2. Broadcast tx

``` sh
tacchaind --home .testnet tx staking create-validator validatortx.json --from validator --keyring-backend test --gas 400000 --gas-prices 100000000000utac -y
```

#### 4. Delegate more tokens (optional)

In this example our moniker is `testnode` as named when we initialized our node. Don't forget to replace with your node moniker.

``` sh
tacchaind --home .testnet tx staking delegate $(tacchaind --home .testnet q staking validators --output json | jq -r '.validators[] | select(.description.moniker == "testnode") | .operator_address') 1000000000000000000utac --keyring-backend test --from validator --gas 400000 --gas-prices 100000000000utac -y
```

### Tac Saint Petersburg Testnet Validator Sentry Node Setup

Validators are responsible for ensuring that the network can sustain denial of service attacks.

One recommended way to mitigate these risks is for validators to carefully structure their network topology in a so-called sentry node architecture.

Validator nodes should only connect to full-nodes they trust because they operate them themselves or are run by other validators they know socially. A validator node will typically run in a data center. Most data centers provide direct links to the networks of major cloud providers. The validator can use those links to connect to sentry nodes in the cloud. This shifts the burden of denial-of-service from the validator's node directly to its sentry nodes, and may require new sentry nodes be spun up or activated to mitigate attacks on existing ones.

Sentry nodes can be quickly spun up or change their IP addresses. Because the links to the sentry nodes are in private IP space, an internet based attack cannot disturb them directly. This will ensure validator block proposals and votes always make it to the rest of the network.

To setup your sentry node architecture you can follow the instructions below:

#### 1. Initialize a new config folder for the sentry node on a new machine with tacchaind binary installed

`tacchaind init <sentry_node_moniker> --chain-id tacchaind_2391-1 --default-denom utac`

- NOTE: This will initialize config folder in $HOME/.tacchaind

- NOTE: Make sure you have replaced your genesis file with the one for Tac Saint Petersburg Testnet. Example script to download it:
`curl https://raw.githubusercontent.com/TacBuild/tacchain/refs/heads/main/networks/tacchain_2391-1/genesis.json > .testnet/config/genesis.json` 

#### 2. Update `config.toml` for sentry node

`private_peer_ids` field is used to specify peers that will not be gossiped to the outside world, in our case the validator node we want it to represent. Example: `private_peer_ids = "3e16af0cead27979e1fc3dac57d03df3c7a77acc@3.87.179.235:26656"`

``` toml
..
persistent_peers = "9c32b3b959a2427bd2aa064f8c9a8efebdad4c23@206.217.210.164:45130,04a2152eed9f73dc44779387a870ea6480c41fe7@206.217.210.164:45140,5aaaf8140262d7416ac53abe4e0bd13b0f582168@23.92.177.41:45110,ddb3e8b8f4d051e914686302dafc2a73adf9b0d2@23.92.177.41:45120"
..
private_peer_ids = "<VALIDATOR_PEER_ID>@<VALIDATOR_IP:PORT>
..
```

- NOTE: Make sure you add persistent peers as described in previous steps for validator setup

#### 3. Update `config.toml` for validator node

Using the sentry node setup, our validator node will be represented by our sentry node, therefore it no longer has to be connected with other peers. We will replace `persistent_peers` so it points to our sentry node, this way it can no longer be accessed by the outter world. We will also disable `pex` field.

```toml
..
persistent_peers = <SENTRY_NODE_ID>@<SENTRY_NODE_IP:PORT>
..
pex = false
..
```

#### 4. Restart validator node and start sentry node.

# Turin Testnet (`tacchain_2390-1`)

| Chain ID                    | `tacchain_2390-1`                                                                             |
|-----------------------------|-----------------------------------------------------------------------------------------------|
| Tacchaind version           | `v0.0.7-testnet`                                                                              |
| RPC                         | <https://newyork-inap-72-251-230-233.ankr.com/tac_tacd_testnet_full_tendermint_rpc_1>         |
| Genesis                     | <https://newyork-inap-72-251-230-233.ankr.com/tac_tacd_testnet_full_tendermint_rpc_1/genesis> |
| gRPC                        | <https://newyork-inap-72-251-230-233.ankr.com/tac_tacd_testnet_full_tendermint_grpc_web_1>    |
| REST API                    | <https://newyork-inap-72-251-230-233.ankr.com/tac_tacd_testnet_full_tendermint_rest_1>        |
| EVM JSON RPC                | <https://newyork-inap-72-251-230-233.ankr.com:443/tac_tacd_testnet_full_rpc_1>                |
| Faucet                      | <https://turin.faucet.tac.build>                                                           |
| EVM Explorer                | <https://turin.explorer.tac.build>                                                         |
| Cosmos Explorer             | <https://turin.bd-explorer.tac.build>                                                |
| Timeout commit (block time) | 3s                                                                                            |
| Peer 1                      | f8124878e3526a9814c0a5f865820c5ea7eb26f8@72.251.230.233:45130                                 |
| Peer 2                      | 4a03d6622a2ad923d79e81951fe651a17faf0be8@107.6.94.246:45130                                   |
| Peer 3                      | ea5719fe6587b18ed0fee81f960e23c65c0e0ccc@206.217.210.164:45130                                |
| Snapshots                   | http://snapshot.tac.ankr.com/tac-{mainnet,spb,turin}-{full,archive}-latest.{tar.lz4,shasum}   |
| - full                      | http://snapshot.tac.ankr.com/tac-turin-full-latest.tar.lz4                                    |
| - archive                   | http://snapshot.tac.ankr.com/tac-turin-archive-latest.tar.lz4                                 |
| Staking UI                  | https://staking.spb.tac.build/                                                                |

#### Hardware Requirements

  - CPU: 8 cores
  - RAM: 16GB (rpc) / 32GB (validator)
  - SSD: 500GB NVMe

### Join Tac Turin Testnet Using Official Snapshots

This example guide connects to testnet. You can replace `chain-id`, `persistent_peers`, `timeout_commit`, `genesis url` with the network you want to join. `--home` flag specifies the path to be used. The example will create [.testnet](.testnet) folder.

### Prerequisites

  - [Go >= v1.21](https://go.dev/doc/install)
  - jq
  - curl
  - tar
  - lz4
  - wget

### 1. Install latest `tacchaind` [v0.0.7-testnet](https://github.com/TacBuild/tacchain/tree/v0.0.7-testnet)

``` shell
git checkout v0.0.7-testnet
make install
```

### 2. Initialize network folder

In this example our node moniker will be `testnode`, don't forget to name your own node differently.

``` sh
tacchaind init testnode --chain-id tacchain_2390-1 --home .testnet
```

### 3. Modify your [config.toml](.testnet/config/config.toml)

``` toml
..
timeout_commit = "3s"
..
persistent_peers = "f8124878e3526a9814c0a5f865820c5ea7eb26f8@72.251.230.233:45130,4a03d6622a2ad923d79e81951fe651a17faf0be8@107.6.94.246:45130,ea5719fe6587b18ed0fee81f960e23c65c0e0ccc@206.217.210.164:45130"
..
```

### 4. Fetch genesis

``` sh
curl https://raw.githubusercontent.com/TacBuild/tacchain/refs/heads/main/networks/tacchain_2390-1/genesis.json > .testnet/config/genesis.json
```

### 5. Fetch snapshot

``` sh
cd .testnet
rm -rf data
wget http://snapshot.tac-turin.ankr.com/tac-turin-full-latest.tar.lz4
lz4 -dc < tac-turin-full-latest.tar.lz4 | tar -xvf -
```

### 6. Start node

``` shell
tacchaind start --chain-id tacchain_2390-1 --home .testnet
```

### Join Tac Turin Testnet Using Docker

#### Prerequisites

  - [Go >= v1.21](https://go.dev/doc/install)
  - jq
  - curl
  - lz4
  - docker
  - docker compose

``` shell
export TAC_HOME="~/.tacchain"
export VERSION="v0.0.7-testnet"

git clone https://github.com/TacBuild/tacchain.git && cd tacchain
mkdir -p $TAC_HOME
cp networks/tacchain_2390-1/{docker-compose.yaml,.env.turin} $TAC_HOME/
git checkout ${VERSION}
docker build -t tacchain:${VERSION} .
cd $TAC_HOME
wget http://snapshot.tac.ankr.com/tac-turin-full-latest.tar.lz4
wget http://snapshot.tac.ankr.com/tac-turin-full-latest.shasum
shasum -c tac-turin-full-latest.shasum
lz4 -dc < tac-turin-full-latest.tar.lz4 | tar -xvf -
docker compose --env-file=.env.turin up -d
## Test
curl -L localhost:45138 -H "Content-Type: application/json" -d '{"jsonrpc": "2.0","method": "eth_blockNumber","params": [],"id": 1}'
```

Assuming all is working you can now proceed from "Join as a validator”


## Join Tac Turin Testnet Manually

This example guide connects to testnet. You can replace `chain-id`, `persistent_peers`, `timeout_commit`, `genesis url` with the network you want to join. `--home` flag specifies the path to be used. The example will create [.testnet](.testnet) folder.

### Prerequisites

  - [Go >= 1.23.6](https://go.dev/doc/install)
  - jq
  - curl

### 1. Install `tacchaind` [v0.0.1](https://github.com/TacBuild/tacchain/tree/v0.0.1)

``` shell
git checkout v0.0.1
make install
```

### 2. Initialize network folder

In this example our node moniker will be `testnode`, don't forget to name your own node differently.

``` sh
tacchaind init testnode --chain-id tacchain_2390-1 --home .testnet
```

### 3. Modify your [config.toml](.testnet/config/config.toml)

``` toml
..
timeout_commit = "3s"
..
persistent_peers = "f8124878e3526a9814c0a5f865820c5ea7eb26f8@72.251.230.233:45130,4a03d6622a2ad923d79e81951fe651a17faf0be8@107.6.94.246:45130,ea5719fe6587b18ed0fee81f960e23c65c0e0ccc@206.217.210.164:45130"
..
```

### 4. Fetch genesis

``` sh
curl https://raw.githubusercontent.com/TacBuild/tacchain/refs/heads/main/networks/tacchain_2390-1/genesis.json > .testnet/config/genesis.json
```

### 5. Start node with `--halt-height` flag.

`--halt-height` flag which will automatically stop your node at specified block height - we want to run `v0.0.1` until block height `1727178`, then we will update our binary before we proceed.

``` shell
tacchaind start --chain-id tacchain_2390-1 --home .testnet --halt-height 1727178
```

### 6. Update binary to [v0.0.2](https://github.com/TacBuild/tacchain/tree/v0.0.2)

Once your node has stopped at specified height, we need to update our binary. This is required because it has breaking changes, which would break our state if run before that point. In this case we enabled EIP712 support.

``` shell
git checkout v0.0.2
make install
```

### 7. Start node with `--halt-height` flag.

We will repeat the same procedure and we need to stop our node once again at specified block, then update our binary.

``` shell
tacchaind start --chain-id tacchain_2390-1 --home .testnet --halt-height 2259069
```

### 8. Update binary to [v0.0.4](https://github.com/TacBuild/tacchain/tree/v0.0.4)

In `v0.0.4` we introduced support for `mcopy`, which is another breaking change.

``` shell
git checkout v0.0.4
make install
```

### 9. Start node with `--halt-height` flag.

We will repeat the same procedure and we need to stop our node once again at specified block, then update our binary.

``` shell
tacchaind start --chain-id tacchain_2390-1 --home .testnet --halt-height 3192449
```

### 10. Update binary to [v0.0.5](https://github.com/TacBuild/tacchain/tree/v0.0.5)

In `v0.0.5` we introduced changes to `DefaultPowerReduction` variable and updated validators state, which is another breaking change.

``` shell
git checkout v0.0.5
make install
```

### 11. Start node

This time we are not going to use `--halt-height` flag, instead we'll wait for our node to hit height `3408172`, at which we applied our next upgrade - `v0.0.6-testnet`. At the specified height, you should see a consensus error stating that you need to upgrade your binary version.

``` shell
tacchaind start --chain-id tacchain_2390-1 --home .testnet
```

### 12. Upgrade binary to [v0.0.6-testnet](https://github.com/TacBuild/tacchain/tree/v0.0.6-testnet)

Once you get the error we mentioned above, you can stop your node and proceed with next update. In this version bumped GETH to v1.13.15.

``` shell
git checkout v0.0.6-testnet
make install
```

### 13. Start node

``` shell
tacchaind start --chain-id tacchain_2390-1 --home .testnet
```

## Join as a validator

NOTE: The provided examples use `--keyring-backend test`. This is not recommended for production validator nodes. Please use `os` or `file` for encryption features and more advanced security.

### 1. Make sure you followed [Join Tac Turin Testnet](#join-tac-turin-testnet) guide and you have a fully synced node to the latest block.

### 2. Fund account and import key

1. Use the [faucet](https://faucet.tac-turin.ankr.com/) to get funds.

2. Export your metamask private key

3. Import private key using the following command. Make sure to replace `<PRIVATE_KEY>` with your funded private key.

``` sh
tacchaind --home .testnet keys unsafe-import-eth-key validator <PRIVATE_KEY> --keyring-backend test
```

### 3. Send `MsgCreateValidator` transaction

1. Generate tx json file

In this example our moniker is `testnode` as named when we initialized our node. Don't forget to replace with your node moniker.

``` sh
echo "{\"pubkey\":$(tacchaind --home .testnet tendermint show-validator),\"amount\":\"1000000000utac\",\"moniker\":\"testnode\",\"identity\":null,\"website\":null,\"security\":null,\"details\":null,\"commission-rate\":\"0.1\",\"commission-max-rate\":\"0.2\",\"commission-max-change-rate\":\"0.01\",\"min-self-delegation\":\"1\"}" > validatortx.json
```

2. Broadcast tx

``` sh
tacchaind --home .testnet tx staking create-validator validatortx.json --from validator --keyring-backend test -y
```

### 4. Delegate more tokens (optional)

In this example our moniker is `testnode` as named when we initialized our node. Don't forget to replace with your node moniker.

``` sh
tacchaind --home .testnet tx staking delegate $(tacchaind --home .testnet q staking validators --output json | jq -r '.validators[] | select(.description.moniker == "testnode") | .operator_address') 1000000000utac --keyring-backend test --from validator -y
```

### 5. Participating in governance (optional)

``` sh
# list all proposals on chain
tacchaind q gov proposals

# once you have identified the proposal (need `proposal_id`) you can place your vote. In the following example we vote with 'yes', alternatively you can vote with 'no'
tacchaind tx gov vote <PROPOSAL_ID> yes --from validator
```

## Validator Sentry Node Setup

Validators are responsible for ensuring that the network can sustain denial of service attacks.

One recommended way to mitigate these risks is for validators to carefully structure their network topology in a so-called sentry node architecture.

Validator nodes should only connect to full-nodes they trust because they operate them themselves or are run by other validators they know socially. A validator node will typically run in a data center. Most data centers provide direct links to the networks of major cloud providers. The validator can use those links to connect to sentry nodes in the cloud. This shifts the burden of denial-of-service from the validator's node directly to its sentry nodes, and may require new sentry nodes be spun up or activated to mitigate attacks on existing ones.

Sentry nodes can be quickly spun up or change their IP addresses. Because the links to the sentry nodes are in private IP space, an internet based attack cannot disturb them directly. This will ensure validator block proposals and votes always make it to the rest of the network.

To setup your sentry node architecture you can follow the instructions below:

### 1. Initialize a new config folder for the sentry node on a new machine with tacchaind binary installed

`tacchaind init <sentry_node_moniker> --chain-id tacchaind_2390-1 --default-denom utac`

- NOTE: This will initialize config folder in $HOME/.tacchaind

- NOTE: Make sure you have replaced your genesis file with the one for Tac Turin Testnet. Example script to download it:
`curl https://raw.githubusercontent.com/TacBuild/tacchain/refs/heads/main/networks/tacchain_2390-1/genesis.json > .testnet/config/genesis.json` 

### 2. Update `config.toml` for sentry node

`private_peer_ids` field is used to specify peers that will not be gossiped to the outside world, in our case the validator node we want it to represent. Example: `private_peer_ids = "3e16af0cead27979e1fc3dac57d03df3c7a77acc@3.87.179.235:26656"`

``` toml
..
timeout_commit = "3s"
..
persistent_peers = "f8124878e3526a9814c0a5f865820c5ea7eb26f8@72.251.230.233:45130,4a03d6622a2ad923d79e81951fe651a17faf0be8@107.6.94.246:45130,ea5719fe6587b18ed0fee81f960e23c65c0e0ccc@206.217.210.164:45130"
..
private_peer_ids = "<VALIDATOR_PEER_ID>@<VALIDATOR_IP:PORT>
..
```

- NOTE: Make sure you add persistent peers as described in previous steps for validator setup

### 3. Update `config.toml` for validator node

Using the sentry node setup, our validator node will be represented by our sentry node, therefore it no longer has to be connected with other peers. We will replace `persistent_peers` so it points to our sentry node, this way it can no longer be accessed by the outter world. We will also disable `pex` field.

```toml
..
persistent_peers = <SENTRY_NODE_ID>@<SENTRY_NODE_IP:PORT>
..
pex = false
..
```

### 4. Restart validator node and start sentry node.

## FAQ

**1) I need some funds on the `tacchain_2390-1` testnet, how can I get them?**

You can request testnet tokens for the `tacchain_2390-1` testnet from the faucet available at <https://faucet.tac-turin.ankr.com/>. Please note that the faucet currently dispenses up to 10 TAC per day per address.

**2) I have completed the guide to join as a validator, but my node is not in the active validator set?**

In order to be included in the active validator set, your validator must have atleast 1 voting power, or if the maximum validators limit has been reached your validator must have greater amount of TAC delegated to them than the validator with lowest amount delegated. Read more - https://forum.cosmos.network/t/why-is-my-newly-created-validator-unbonded/1841/2
