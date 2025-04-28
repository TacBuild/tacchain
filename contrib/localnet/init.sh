#!/bin/bash -e

# environment variables
TACCHAIND=${TACCHAIND:-$(which tacchaind)}
DENOM=${DENOM:-utac}
CHAIN_ID=${CHAIN_ID:-tacchain_2390-1}
KEYRING_BACKEND=${KEYRING_BACKEND:-test}
HOMEDIR=${HOMEDIR:-$HOME/.tacchaind}
INITIAL_BALANCE=${INITIAL_BALANCE:-2000000000000000000$DENOM}
INITIAL_STAKE=${INITIAL_STAKE:-1000000000000000000$DENOM}
RPC_PORT=${RPC_PORT:-26657}
P2P_PORT=${P2P_PORT:-26656}
GRPC_PORT=${GRPC_PORT:-9090}
GRPC_WEB_PORT=${GRPC_WEB_PORT:-9091}
API_PORT=${API_PORT:-1317}
JSON_RPC_PORT=${JSON_RPC_PORT:-8545}
JSON_WS_PORT=${JSON_WS_PORT:-8546}
METRICS_PORT=${METRICS_PORT:-6065}
PROMETHEUS_PORT=${PROMETHEUS_PORT:-26660}
PPROF_PORT=${PPROF_PORT:-6060}
PROXY_PORT=${PROXY_PORT:-26658}

# prompt user for confirmation before cleanup
read -p "This will remove all existing data in $HOMEDIR. Do you want to proceed? (y/n): " confirm
if [[ $confirm != "y" && $confirm != "Y" ]]; then
    echo "Cleanup aborted."
    exit 1
fi

# cleanup old data
rm -rf $HOMEDIR

# set cli options default values
$TACCHAIND config set client chain-id $CHAIN_ID
$TACCHAIND config set client keyring-backend $KEYRING_BACKEND
$TACCHAIND config set client output json

# init genesis file
$TACCHAIND init test --chain-id $CHAIN_ID --default-denom $DENOM --home $HOMEDIR

# setup and add validator to genesis
$TACCHAIND keys add validator --keyring-backend $KEYRING_BACKEND --home $HOMEDIR
$TACCHAIND genesis add-genesis-account validator $INITIAL_BALANCE --keyring-backend $KEYRING_BACKEND --home $HOMEDIR
$TACCHAIND genesis gentx validator $INITIAL_STAKE --chain-id $CHAIN_ID --keyring-backend $KEYRING_BACKEND --home $HOMEDIR
$TACCHAIND genesis collect-gentxs --keyring-backend $KEYRING_BACKEND --home $HOMEDIR

# edit configs

# set EVM config
# get ethereum chain id from CHAIN_ID
EVM_CHAIN_ID=$(echo $CHAIN_ID | sed -E 's/.*_([0-9]+)-.*/\1/')
if [[ -z $EVM_CHAIN_ID ]]; then
    echo "Invalid CHAIN_ID format. Expected format: <any_string>_<number>-<number>"
    exit 1
fi

sed -i.bak "s/\"chain_id\": \"262144\"/\"chain_id\": \"$EVM_CHAIN_ID\"/g" $HOMEDIR/config/genesis.json
sed -i.bak "s/\"denom\": \"atest\"/\"denom\": \"$DENOM\"/g" $HOMEDIR/config/genesis.json
sed -i.bak "s/\"evm_denom\": \"atest\"/\"evm_denom\": \"$DENOM\"/g" $HOMEDIR/config/genesis.json

# set max gas which is required for evm txs
sed -i.bak "s/\"max_gas\": \"-1\"/\"max_gas\": \"20000000\"/g" $HOMEDIR/config/genesis.json

# enable evm eip-3855
sed -i.bak "s/\"extra_eips\": \[\]/\"extra_eips\": \[\"3855\"\]/g" $HOMEDIR/config/genesis.json

# disable EIP-155
sed -i.bak "s/\"allow_unprotected_txs\": false/\"allow_unprotected_txs\": true/g" $HOMEDIR/config/genesis.json
sed -i.bak "s/allow-unprotected-txs = false/allow-unprotected-txs = true/g" $HOMEDIR/config/app.toml

# set evm/erc20 precompiles
sed -i.bak "s/\"active_static_precompiles\": \[\]/\"active_static_precompiles\": \[\"0x0000000000000000000000000000000000000100\",\"0x0000000000000000000000000000000000000400\",\"0x0000000000000000000000000000000000000800\",\"0x0000000000000000000000000000000000000801\",\"0x0000000000000000000000000000000000000802\",\"0x0000000000000000000000000000000000000803\",\"0x0000000000000000000000000000000000000804\",\"0x0000000000000000000000000000000000000805\",\"0x0000000000000000000000000000000000000806\",\"0x0000000000000000000000000000000000000807\"\]/g" $HOMEDIR/config/genesis.json
sed -i.bak "s/\"native_precompiles\": \[\]/\"native_precompiles\": \[\"0xD4949664cD82660AaE99bEdc034a0deA8A0bd517\"\]/g" $HOMEDIR/config/genesis.json
sed -i.bak "s/\"token_pairs\": \[\]/\"token_pairs\": \[{\"contract_owner\":1,\"erc20_address\":\"0xD4949664cD82660AaE99bEdc034a0deA8A0bd517\",\"denom\":\"$DENOM\",\"enabled\":true}\]/g" $HOMEDIR/config/genesis.json

# set block time to 3s
sed -i.bak "s/timeout_commit = \"5s\"/timeout_commit = \"3s\"/g" $HOMEDIR/config/config.toml

# reduce proposal time
sed -i.bak "s/\"voting_period\": \"172800s\"/\"voting_period\": \"900s\"/g" $HOMEDIR/config/genesis.json
sed -i.bak "s/\"expedited_voting_period\": \"86400s\"/\"expedited_voting_period\": \"600s\"/g" $HOMEDIR/config/genesis.json

# enable apis
sed -i.bak "s/enable = false/enable = true/g" $HOMEDIR/config/app.toml

# enable rpc cors
sed -i.bak "s/cors_allowed_origins = \[\]/cors_allowed_origins = \[\"*\"\]/g" $HOMEDIR/config/config.toml

# update blocks per year to match 3s block time
sed -i.bak "s/\"blocks_per_year\": \"6311520\"/\"blocks_per_year\": \"10512000\"/g" $HOMEDIR/config/genesis.json

# set ports
sed -i.bak "s/26657/$RPC_PORT/g" $HOMEDIR/config/config.toml
sed -i.bak "s/26656/$P2P_PORT/g" $HOMEDIR/config/config.toml
sed -i.bak "s/9090/$GRPC_PORT/g" $HOMEDIR/config/app.toml
sed -i.bak "s/9091/$GRPC_WEB_PORT/g" $HOMEDIR/config/app.toml
sed -i.bak "s/1317/$API_PORT/g" $HOMEDIR/config/app.toml
sed -i.bak "s/8545/$JSON_RPC_PORT/g" $HOMEDIR/config/app.toml
sed -i.bak "s/8546/$JSON_WS_PORT/g" $HOMEDIR/config/app.toml
sed -i.bak "s/6065/$METRICS_PORT/g" $HOMEDIR/config/app.toml
sed -i.bak "s/26660/$PROMETHEUS_PORT/g" $HOMEDIR/config/config.toml
sed -i.bak "s/6060/$PPROF_PORT/g" $HOMEDIR/config/config.toml
sed -i.bak "s/26658/$PROXY_PORT/g" $HOMEDIR/config/config.toml
