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
  export P2P_PORT=451$((i+1))0        # 45110
  export RPC_PORT=451$((i+1))1        # 45111
  export API_PORT=451$((i+1))2        # 45112
  export METRICS_PORT=451$((i+1))3    # 45113
  export PPROF_PORT=451$((i+1))4      # 45114
  export PROMETHEUS_PORT=451$((i+1))5 # 45115
  export GRPC_WEB_PORT=451$((i+1))6   # 45116
  export GRPC_PORT=451$((i+1))7       # 45117
  export JSON_RPC_PORT=451$((i+1))8   # 45118
  export JSON_WS_PORT=451$((i+1))9    # 45119
  export PROXY_PORT=451$((i+1))10     # 451110

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

# clear gentx in genesis because we already collect in init.sh, so recollect here instead changing the original script
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
      P2P_PORT=451$((j+1))0
      PERSISTENT_PEERS+=$NODE_ID@127.0.0.1:$P2P_PORT
      # add comma if not last node
      if [ "$CURRENT_PEER" != "$((VALIDATORS_COUNT-1))" ]; then
        PERSISTENT_PEERS+=","
      fi
    fi
  done
  sed -i.bak "s/persistent_peers = \"\"/persistent_peers = \"$PERSISTENT_PEERS\"/g" $HOMEDIR/node$i/config/config.toml
  sed -i.bak "s/seeds = \"\"/seeds = \"$PERSISTENT_PEERS\"/g" $HOMEDIR/node$i/config/config.toml

  # set multi node configs
  sed -i.bak "s/addr_book_strict = true/addr_book_strict = false/g" $HOMEDIR/node$i/config/config.toml
  sed -i.bak "s/allow_duplicate_ip = false/allow_duplicate_ip = true/g" $HOMEDIR/node$i/config/config.toml
done