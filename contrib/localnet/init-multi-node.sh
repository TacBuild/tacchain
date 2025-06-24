#!/bin/bash -e

# environment variables
export TACCHAIND=${TACCHAIND:-$(which tacchaind)}
export HOMEDIR=${HOMEDIR:-./.testnet}
export CHAIN_ID=${CHAIN_ID:-tacchain_239-1}
export KEYRING_BACKEND=${KEYRING_BACKEND:-test}
export VALIDATORS_COUNT=4
export VALIDATOR_NAME=${VALIDATOR_NAME:-TAC Validator}
export VALIDATOR_IDENTITY=${VALIDATOR_IDENTITY:-4DD1A5E1D03FA12D}
export VALIDATOR_WEBSITE=${VALIDATOR_WEBSITE:-https://tac.build/}
export VALIDATOR_1_MNEMONIC=${VALIDATOR_1_MNEMONIC:-"island mail dice alien project surround orchard ball twist worth innocent arrange assume dragon rotate enough flee rapid rookie swim addict ice destroy run"} # tac15lvhklny0khnwy7hgrxsxut6t6ku2cgknw79fr
export VALIDATOR_2_MNEMONIC=${VALIDATOR_2_MNEMONIC:-"margin funny awkward answer squirrel inner venue spell expose close tank salute series neck oval real bunker can text chronic capital teach arena extend"} # tac16p9nqhd348aaungp5p5vjuwedaw03pvywdzwdk
export VALIDATOR_3_MNEMONIC=${VALIDATOR_3_MNEMONIC:-"that away spike absorb aspect loan shuffle purchase number knock cover night library shock mask cheese upset float churn wall fox veteran actress motor"} # tac1qp4h82jgqqa5ezgzck8z75dn8q0t0nv45pzh6v
export VALIDATOR_4_MNEMONIC=${VALIDATOR_4_MNEMONIC:-"floor wrong idle cloth nose material forward urge grape always into buyer atom excuse odor decade crouch purchase shadow energy voyage pact skate pigeon"} # tac1d30q62hl0wn6n5m39sd0yqswq6jr3hntt2cm4n
export GENESIS_ACC_1_ADDRESS=${GENESIS_ACC_1_ADDRESS:-}
export GENESIS_ACC_2_ADDRESS=${GENESIS_ACC_2_ADDRESS:-}
export INITIAL_SUPPLY=${INITIAL_SUPPLY:-10000000000000000000000000000}
export BLOCK_TIME_SECONDS=${BLOCK_TIME_SECONDS:-2}
export MAX_GAS=${MAX_GAS:-90000000}
export MIN_GAS_PRICE=${MIN_GAS_PRICE:-25000000000}
export GOV_TIME_SECONDS=${GOV_TIME_SECONDS:-43200}
export GOV_MIN_DEPOSIT=${GOV_MIN_DEPOSIT:-10000000000000000}
export GOV_MIN_EXPEDITED_DEPOSIT=${GOV_MIN_EXPEDITED_DEPOSIT:-50000000000000000}
export GOV_MIN_INITIAL_DEPOSIT_RATIO=${GOV_MIN_INITIAL_DEPOSIT_RATIO:-1}
export INFLATION_MAX=${INFLATION_MAX:-0.05}
export INFLATION_MIN=${INFLATION_MIN:-0.01}
export GOAL_BONDED=${GOAL_BONDED:-0.6}
export SLASH_DOWNTIME_PENALTY=${SLASH_DOWNTIME_PENALTY:-0.001}
export SLASH_SIGNED_BLOCKS_WINDOW=${SLASH_SIGNED_BLOCKS_WINDOW:-21600}
export MAX_VALIDATORS=${MAX_VALIDATORS:-14}

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

# token distribution
# - genesis acc 1: initial supply - 0.2% (validators) - 10k (genesis acc 2)
# - genesis acc 2: 10k TAC
# - validators: 0.2% of initial supply split between 4 validators, self-delegation - 0.2% / validators_count - 100TAC for emergency

# allocating 0.2% of initial supply split between 4 validators
VALIDATOR_BALANCE=$(echo "$INITIAL_SUPPLY * 0.002 / $VALIDATORS_COUNT" | bc)
# keeping 100TAC for emergency, e.g. unjailing tx fees
VALIDATOR_EMERGENCY_BALANCE=100000000000000000000
# self delegeting the rest
VALIDATOR_SELF_DELEGATION=$(echo "$VALIDATOR_BALANCE - $VALIDATOR_EMERGENCY_BALANCE" | bc)

# allocating 10k TAC to genesis account 2
GENESIS_ACC_2_BALANCE=10000000000000000000000

# deduct validator balances from initial supply and mint to genesis account
GENESIS_ACC_1_BALANCE=$(echo "$INITIAL_SUPPLY - ($VALIDATOR_BALANCE * $VALIDATORS_COUNT) - $GENESIS_ACC_2_BALANCE" | bc)

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

  export NODE_MONIKER="$VALIDATOR_NAME $((i + 1))"
  
  export INITIAL_BALANCE=$VALIDATOR_BALANCE
  export INITIAL_STAKE=$VALIDATOR_SELF_DELEGATION

  # dynamically get mnemonic env var for each validator
  mnemonic_var="VALIDATOR_$((i+1))_MNEMONIC"
  export VALIDATOR_MNEMONIC="${!mnemonic_var}"

  # call init.sh script to initialize the node
  echo y | HOMEDIR=$NODEDIR $(dirname "$0")/./init.sh

  # explicitly add balances to first node(node0) which will be used to collect gentxs later
  ADDRESS=$($TACCHAIND keys show validator --keyring-backend $KEYRING_BACKEND --home $NODEDIR -a)
  $TACCHAIND genesis add-genesis-account $ADDRESS ${VALIDATOR_BALANCE}utac --keyring-backend $KEYRING_BACKEND --home $HOMEDIR/node0  &> /dev/null || true

  # copy gentx into main gentxs
  cp $NODEDIR/config/gentx/* "$HOMEDIR/gentxs/"
done

# add genesis account 1
if [ -z "$GENESIS_ACC_1_ADDRESS" ]; then
  $TACCHAIND keys add genesis_acc_1 --keyring-backend $KEYRING_BACKEND --home $HOMEDIR/node0
  GENESIS_ACC_1_ADDRESS=$($TACCHAIND keys show genesis_acc_1 --keyring-backend $KEYRING_BACKEND --home $HOMEDIR/node0 -a)
fi
$TACCHAIND genesis add-genesis-account $GENESIS_ACC_1_ADDRESS ${GENESIS_ACC_1_BALANCE}utac --keyring-backend $KEYRING_BACKEND --home $HOMEDIR/node0

# add genesis account 2
if [ -z "$GENESIS_ACC_2_ADDRESS" ]; then
  $TACCHAIND keys add genesis_acc_2 --keyring-backend $KEYRING_BACKEND --home $HOMEDIR/node0
  GENESIS_ACC_2_ADDRESS=$($TACCHAIND keys show genesis_acc_2 --keyring-backend $KEYRING_BACKEND --home $HOMEDIR/node0 -a)
fi
$TACCHAIND genesis add-genesis-account $GENESIS_ACC_2_ADDRESS ${GENESIS_ACC_2_BALANCE}utac --keyring-backend $KEYRING_BACKEND --home $HOMEDIR/node0

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
      NODE_ID=$($TACCHAIND tendermint show-node-id --home $HOMEDIR/node$j)
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