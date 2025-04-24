#!/bin/bash -e

# environment variables
export TACCHAIND=${TACCHAIND:-$(which tacchaind)}
export HOMEDIR=${HOMEDIR:-./.testnet}
export KEYRING_BACKEND=${KEYRING_BACKEND:-test}
export VALIDATORS_COUNT=${VALIDATORS_COUNT:-4}
export DENOM=${DENOM:-utac}
export INITIAL_BALANCE=${INITIAL_BALANCE:-2000000000000000000$DENOM}
export INITIAL_STAKE=${INITIAL_STAKE:-1000000000000000000$DENOM}
export FAUCET_BALANCE=${FAUCET_BALANCE:-1000000000000000000000000000$DENOM}

# validate validators count is at least 2
if [[ "$VALIDATORS_COUNT" -le 1 ]]; then
  echo "Error: VALIDATORS_COUNT must at least 2. For single node setup, use init.sh (make localnet-init)."
  exit 1
fi

# prompt user for confirmation before cleanup
read -p "This will remove all existing data in $HOMEDIR. Do you want to proceed? (y/n): " confirm
if [[ $confirm != "y" && $confirm != "Y" ]]; then
    echo "Cleanup aborted."
    exit 1
fi

# cleanup old data
rm -rf $HOMEDIR

# create folder to collect validator gentxs
mkdir -p $HOMEDIR/gentxs

# initialize config folder for each validator
for ((i = 0 ; i < VALIDATORS_COUNT ; i++)); do
  NODE_KEY="node$i"
  NODEDIR="$HOMEDIR/$NODE_KEY"

  # set ports
  export RPC_PORT=$((26657 + i * 1000))
  export P2P_PORT=$((26656 + i * 1000))
  export GRPC_PORT=$((9090 + i * 1000))
  export GRPC_WEB_PORT=$((9091 + i * 1000))
  export API_PORT=$((1317 + i * 1000))
  export JSON_RPC_PORT=$((8545 + i * 1000))
  export JSON_WS_PORT=$((8546 + i * 1000))
  export METRICS_PORT=$((6065 + i * 1000))
  export PROMETHEUS_PORT=$((26660 + i * 1000))
  export PPROF_PORT=$((6060 + i * 1000))

  # call init.sh script to initialize the node
  echo y | HOMEDIR=$NODEDIR $(dirname "$0")/./init.sh

  # explicitly add balances to first node(node0) which will be used to collect gentxs later
  ADDRESS=$(tacchaind keys show validator --keyring-backend $KEYRING_BACKEND --home $NODEDIR -a)
  tacchaind genesis add-genesis-account $ADDRESS $INITIAL_BALANCE --keyring-backend $KEYRING_BACKEND --home $HOMEDIR/node0  &> /dev/null || true

  # copy gentx into main gentxs
  cp $NODEDIR/config/gentx/* "$HOMEDIR/gentxs/"
done

# add faucet account
tacchaind keys add faucet --keyring-backend $KEYRING_BACKEND --home $HOMEDIR/node0
tacchaind genesis add-genesis-account faucet $FAUCET_BALANCE --keyring-backend $KEYRING_BACKEND --home $HOMEDIR/node0

# collect gentxs from first node, then copy updated genesis to all validators, then update persistent peers
cp $HOMEDIR/gentxs/* "$HOMEDIR/node0/config/gentx/"

# clear gentx in genesis
jq '.app_state.genutil.gen_txs = []' "$HOMEDIR/node0/config/genesis.json" > "$HOMEDIR/node0/config/genesis_tmp.json" && mv "$HOMEDIR/node0/config/genesis_tmp.json" "$HOMEDIR/node0/config/genesis.json"

$TACCHAIND genesis collect-gentxs --home $HOMEDIR/node0

# copy genesis to main directory for reference
cp $HOMEDIR/node0/config/genesis.json $HOMEDIR/genesis.json

for ((i = 0 ; i < VALIDATORS_COUNT ; i++)); do
  # copy final genesis to all validators
  cp $HOMEDIR/node0/config/genesis.json $HOMEDIR/node$i/config/genesis.json &> /dev/null || true
  
  # update persistent peers
  PERSISTENT_PEERS=""
  CURRENT_PEER=0
  for ((j = 0 ; j < VALIDATORS_COUNT ; j++)); do
    # add all nodes except the current one
    if [ "$i" != "$j" ]; then
      CURRENT_PEER=$((CURRENT_PEER + 1))
      NODE_ID=$(tacchaind tendermint show-node-id --home $HOMEDIR/node$j)
      P2P_PORT=$((26656 + j * 1000))
      PERSISTENT_PEERS+=$NODE_ID@0.0.0.0:$P2P_PORT
      # add comma if not last node
      if [ "$CURRENT_PEER" != "$((VALIDATORS_COUNT-1))" ]; then
        PERSISTENT_PEERS+=","
      fi
    fi
  done

  sed -i.bak "s/persistent_peers = \"\"/persistent_peers = \"$PERSISTENT_PEERS\"/g" $HOMEDIR/node$i/config/config.toml
done