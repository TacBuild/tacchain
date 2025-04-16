#!/bin/bash -e

# environment variables
TACCHAIND=${TACCHAIND:-$(which tacchaind)}
DENOM=${DENOM:-utac}
CHAIN_ID=${CHAIN_ID:-tacchain_2390-1}
KEYRING_BACKEND=${KEYRING_BACKEND:-test}
HOMEDIR=${HOMEDIR:-$HOME/.tacchaind}

# prompt user for confirmation before cleanup
read -p "This will remove all existing data in $HOMEDIR. Do you want to proceed? (y/n): " confirm
if [[ $confirm != "y" ]]; then
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

# set evm_denom to $DENOM in genesis
sed -i.bak "s/\"evm_denom\": \"aphoton\"/\"evm_denom\": \"$DENOM\"/g" $HOMEDIR/config/genesis.json
sed -i.bak "s/\"no_base_fee\": false/\"no_base_fee\": true/g" $HOMEDIR/config/genesis.json
# set max gas which is required for evm txs
sed -i.bak "s/\"max_gas\": \"-1\"/\"max_gas\": \"20000000\"/g" $HOMEDIR/config/genesis.json
# enable evm eip-3855
sed -i.bak "s/\"extra_eips\": \[\]/\"extra_eips\": \[\"3855\"\,\"5656\"\]/g" $HOMEDIR/config/genesis.json
# disable EIP-155
sed -i.bak "s/\"allow_unprotected_txs\": false/\"allow_unprotected_txs\": true/g" $HOMEDIR/config/genesis.json
sed -i.bak "s/allow-unprotected-txs = false/\allow-unprotected-txs = true/g" $HOMEDIR/config/app.toml
# set evm/erc20 precompiles etc (see local_node.sh in cosmos/evm)
sed -i.bak "s/\"active_static_precompiles\": \[\]/\"active_static_precompiles\": \[\"0x0000000000000000000000000000000000000100\",\"0x0000000000000000000000000000000000000400\",\"0x0000000000000000000000000000000000000800\",\"0x0000000000000000000000000000000000000801\",\"0x0000000000000000000000000000000000000802\",\"0x0000000000000000000000000000000000000803\",\"0x0000000000000000000000000000000000000804\",\"0x0000000000000000000000000000000000000805\"\]/g" $HOMEDIR/config/genesis.json
sed -i.bak "s/\"native_precompiles\": \[\]/\"native_precompiles\": \[\"0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE\"\]/g" $HOMEDIR/config/genesis.json
sed -i.bak "s/\"token_pairs\": \[\]/\"token_pairs\": \[{\"contract_owner\":1,\"erc20_address\":\"0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE\",\"denom\":\"wtac\",\"enabled\":true}\]/g" $HOMEDIR/config/genesis.json
# set block time to 3s
sed -i.bak "s/timeout_commit = "5s"/timeout_commit = "3s"/g" $HOMEDIR/config/config.toml
# reduce proposal time
sed -i.bak "s/\"voting_period\": \"172800s\"/\"voting_period\": \"90s\"/g" $HOMEDIR/config/genesis.json
sed -i.bak "s/\"expedited_voting_period\": \"86400s\"/\"expedited_voting_period\": \"60s\"/g" $HOMEDIR/config/genesis.json
# enable apis
sed -i.bak "s/enable = false/enable = true/g" $HOMEDIR/config/app.toml
# enable rpc cors
sed -i.bak "s/cors_allowed_origins = \[\]/cors_allowed_origins = \[\"*\"\]/g" $HOMEDIR/config/config.toml

# setup and add validator to genesis
$TACCHAIND keys add validator --keyring-backend $KEYRING_BACKEND --home $HOMEDIR
$TACCHAIND genesis add-genesis-account validator 1000000000000000000000000000000$DENOM --keyring-backend $KEYRING_BACKEND --home $HOMEDIR
$TACCHAIND genesis gentx validator 10000000000000000000000000000$DENOM --chain-id $CHAIN_ID --keyring-backend $KEYRING_BACKEND --home $HOMEDIR
$TACCHAIND genesis collect-gentxs --keyring-backend $KEYRING_BACKEND --home $HOMEDIR