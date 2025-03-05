# Tac Networks

| Chain ID        | Type      | Status     | Version  | Notes         |
|-----------------|-----------|------------|----------|---------------|
| tacchain_2390-1 | `testnet` | **Active** | `v0.0.4` | Turin Testnet |

## Turin Testnet (`tacchain_2390-1`)

| Chain ID                    | `tacchain_2390-1`                                                                             |
|-----------------------------|-----------------------------------------------------------------------------------------------|
| Tacchaind version           | `v0.0.4`                                                                                      |
| RPC                         | <https://newyork-inap-72-251-230-233.ankr.com/tac_tacd_testnet_full_tendermint_rpc_1>         |
| Genesis                     | <https://newyork-inap-72-251-230-233.ankr.com/tac_tacd_testnet_full_tendermint_rpc_1/genesis> |
| gRPC                        | <https://newyork-inap-72-251-230-233.ankr.com/tac_tacd_testnet_full_tendermint_grpc_web_1>    |
| REST API                    | <https://newyork-inap-72-251-230-233.ankr.com/tac_tacd_testnet_full_tendermint_rest_1>        |
| EVM JSON RPC                | <https://newyork-inap-72-251-230-233.ankr.com:443/tac_tacd_testnet_full_rpc_1>                |
| Faucet                      | <https://faucet.tac-turin.ankr.com>                                                           |
| EVM Explorer                | <https://explorer.tac-turin.ankr.com>                                                         |
| Cosmos Explorer             | <https://explorer.tacchain-turin.ankr.com/tac>                                                |
| Timeout commit (block time) | 3s                                                                                            |
| Peer 1                      | f8124878e3526a9814c0a5f865820c5ea7eb26f8@72.251.230.233:45130                                 |
| Peer 2                      | 4a03d6622a2ad923d79e81951fe651a17faf0be8@107.6.94.246:45130                                   |
| Peer 3                      | ea5719fe6587b18ed0fee81f960e23c65c0e0ccc@206.217.210.164:45130                                |
| Snapshots                   | TBD                                                                                           |
| Frontend                    | TBD                                                                                           |

## Join a network

This example guide connects to testnet. You can replace `chain-id`, `persistent_peers`, `timeout_commit`, `genesis url` with the network you want to join. `--home` flag specifies the path to be used. The example will create [.testnet](.testnet) folder.

### Prerequisites

  - [Go >= v1.21](https://go.dev/doc/install)
  - jq
  - curl

### 1. Install `tacchaind`

``` sh
make install
```

### 2. Initialize network folder

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
curl https://newyork-inap-72-251-230-233.ankr.com/tac_tacd_testnet_full_tendermint_rpc_1/genesis | jq '.result.genesis' > .testnet/config/genesis.json
```

### 5. Start node

``` sh
tacchaind start --chain-id tacchain_2390-1 --home .testnet
```

## Join as a validator

### 1. Make sure you followed [Join a network](#join-a-network) guide and you have a fully synced node to the latest block.

### 2. Fund account and import key

1. Use the [faucet](https://faucet.tac-turin.ankr.com/) to get funds.

2. Export your metamask private key

3. Import private key using the following command. Make sure to replace `<PRIVATE_KEY>` with your funded private key.

``` sh
tacchaind --home .testnet keys unsafe-import-eth-key validator <PRIVATE_KEY> --keyring-backend test
```

### 3. Send `MsgCreateValidator` transaction

1. Generate tx json file

``` sh
echo "{\"pubkey\":$(tacchaind --home .testnet tendermint show-validator),\"amount\":\"1000000000utac\",\"moniker\":\"testnode\",\"identity\":null,\"website\":null,\"security\":null,\"details\":null,\"commission-rate\":\"0.1\",\"commission-max-rate\":\"0.2\",\"commission-max-change-rate\":\"0.01\",\"min-self-delegation\":\"1\"}" > validatortx.json
```

2. Broadcast tx

``` sh
tacchaind --home .testnet tx staking create-validator validatortx.json --from validator --keyring-backend test -y
```

### 4. Delegate more tokens (optional)

``` sh
tacchaind --home .testnet tx staking delegate $(tacchaind --home .testnet q staking validators --output json | jq -r '.validators[] | select(.description.moniker == "testnode") | .operator_address') 1000000000utac --keyring-backend test --from validator -y
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
`curl https://newyork-inap-72-251-230-233.ankr.com/tac_tacd_testnet_full_tendermint_rpc_1/genesis | jq '.result.genesis' > ./config/genesis.json` 

### 2. Update `config.toml` for sentry node

`private_peer_ids` field is used to specify peers that will not be gossiped to the outside world, in our case the validator node we want it to represent. Example: `private_peer_ids = "3e16af0cead27979e1fc3dac57d03df3c7a77acc@3.87.179.235:26656"`

``` toml
..
timeout_commit = "3s"
..
persistent_peers = "9b4995a048f930776ee5b799f201e9b00727ffcc@107.6.94.246:45120,e3c2479a6f418841bd64bae6dff027ea3efc1987@72.251.230.233:45120,fbf04b3d67705ed48831aa80ebe733775e672d1a@107.6.94.246:45110,5a6f0e342ea66cb769194c81141ffbff7417fbcd@72.251.230.233:45110"
..
private_peer_ids = "<VALIDATOR_PEER_ID>@<VALIDATOR_IP:PORT>
..
```

- NOTE: Make sure you add persistent peers as described in previous steps for validator setup

### 3. Update `config.toml` for validator node

Using the sentry node setup, our validator node will be represented by our sentry node, therefore it no longer has to be connected with other peers. We will replace `peristent_peers` so it points to our sentry node, this way it can no longer be accessed by the outter world. We will also disable `pex` field.

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
