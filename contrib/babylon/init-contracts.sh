#!/bin/bash -e

# environment variables
TACCHAIND=${TACCHAIND:-$(which tacchaind)}
HOMEDIR=${HOMEDIR:-$HOME/.tacchaind}
OWNER_KEY=${OWNER_KEY:-validator}
KEYRING_BACKEND=${KEYRING_BACKEND:-test}
RPC_URL=${RPC_URL:-http://tacchaind:27657}
CHAIN_ID=${CHAIN_ID:-tacchain_2391-1}

BABYLON_CONTRACT_FILE=${BABYLON_CONTRACT_FILE:-"$(dirname "$0")/contracts/babylon_contract_v0.14.0.wasm"}
BTC_LIGHT_CLIENT_CONTRACT_FILE=${BTC_LIGHT_CLIENT_CONTRACT_FILE:-"$(dirname "$0")/contracts/btc_light_client_v0.14.0.wasm"}
BTC_STAKING_CONTRACT_FILE=${BTC_STAKING_CONTRACT_FILE:-"$(dirname "$0")/contracts/btc_staking_v0.14.0.wasm"}
BTC_FINALITY_CONTRACT_FILE=${BTC_FINALITY_CONTRACT_FILE:-"$(dirname "$0")/contracts/btc_finality_v0.14.0.wasm"}

# wait for network to start
echo "Waiting for network to start"
timeout=120
elapsed=0
interval=2
while ! tacchaind query block --type=height 3 --node $RPC_URL > /dev/null 2>&1; do
  sleep $interval
  elapsed=$((elapsed + interval))
  if [ $elapsed -ge $timeout ]; then
    echo "Failed to start network. Timeout waiting for block height 3"
    exit 1
  fi
done
echo "Network started successfully"

$TACCHAIND config set client chain-id $CHAIN_ID

# check if already initialized
DEFAULT_PARAMS='{"babylon_contract_code_id":"0","btc_light_client_contract_code_id":"0","btc_staking_contract_code_id":"0","btc_finality_contract_code_id":"0","babylon_contract_address":"","btc_light_client_contract_address":"","btc_staking_contract_address":"","btc_finality_contract_address":"","max_gas_begin_blocker":500000}'
CURRENT_PARAMS=$($TACCHAIND q babylon params --node $RPC_URL --output json)
if [ "$CURRENT_PARAMS" != "$DEFAULT_PARAMS" ]; then
  echo "Babylon Contracts already initialized. Skipping babylon contract init."
  exit 0
fi

# upload babylon contract
echo "Uploading babylon contract code $BABYLON_CONTRACT_FILE..."
TX_HASH=$($TACCHAIND tx wasm store "$BABYLON_CONTRACT_FILE" --from $OWNER_KEY --keyring-backend $KEYRING_BACKEND --home $HOMEDIR --node $RPC_URL --gas-prices 25000000000utac --gas 6000000 -y --output json | jq -r '.txhash')
echo "Waiting for transaction $TX_HASH to be included in a block..."
sleep 5
BABYLON_CONTRACT_CODE_ID=$($TACCHAIND query tx $TX_HASH --node $RPC_URL --output json | jq -r '.events[] | select(.type == "store_code") | .attributes[] | select(.key == "code_id") | .value' | tr -d '"')
BABYLON_CONTRACT_CODE_ID=$(printf "%d" "$BABYLON_CONTRACT_CODE_ID" 2>/dev/null || echo "$BABYLON_CONTRACT_CODE_ID")
if ! $TACCHAIND query wasm code $BABYLON_CONTRACT_CODE_ID temp.txt --node $RPC_URL; then
  echo "Failed to upload babylon contract code."
  exit 1
fi

# upload btc light client contract
echo "Uploading btc light client contract code $BTC_LIGHT_CLIENT_CONTRACT_FILE..."
TX_HASH=$($TACCHAIND tx wasm store "$BTC_LIGHT_CLIENT_CONTRACT_FILE" --from $OWNER_KEY --keyring-backend $KEYRING_BACKEND --home $HOMEDIR --node $RPC_URL --gas-prices 25000000000utac --gas 6000000 -y --output json | jq -r '.txhash')
echo "Waiting for transaction $TX_HASH to be included in a block..."
sleep 5
BTC_LIGHT_CLIENT_CONTRACT_CODE_ID=$($TACCHAIND query tx $TX_HASH --node $RPC_URL --output json | jq -r '.events[] | select(.type == "store_code") | .attributes[] | select(.key == "code_id") | .value' | tr -d '"')  
if ! $TACCHAIND query wasm code $BTC_LIGHT_CLIENT_CONTRACT_CODE_ID temp.txt --node $RPC_URL; then
  echo "Failed to upload btc light client contract code."
  exit 1
fi

# upload btc staking contract
echo "Uploading btc staking contract code $BTC_STAKING_CONTRACT_FILE..."
TX_HASH=$($TACCHAIND tx wasm store "$BTC_STAKING_CONTRACT_FILE" --from $OWNER_KEY --keyring-backend $KEYRING_BACKEND --home $HOMEDIR --node $RPC_URL --gas-prices 25000000000utac --gas 6000000 -y --output json | jq -r '.txhash')
echo "Waiting for transaction $TX_HASH to be included in a block..."
sleep 5
BTC_STAKING_CONTRACT_CODE_ID=$($TACCHAIND query tx $TX_HASH --node $RPC_URL --output json | jq -r '.events[] | select(.type == "store_code") | .attributes[] | select(.key == "code_id") | .value' | tr -d '"')
if ! $TACCHAIND query wasm code $BTC_STAKING_CONTRACT_CODE_ID temp.txt --node $RPC_URL; then
  echo "Failed to upload btc staking contract code."
  exit 1
fi

# upload btc finality contract
echo "Uploading btc finality contract code $BTC_FINALITY_CONTRACT_FILE..."
TX_HASH=$($TACCHAIND tx wasm store "$BTC_FINALITY_CONTRACT_FILE" --from $OWNER_KEY --keyring-backend $KEYRING_BACKEND --home $HOMEDIR --node $RPC_URL --gas-prices 25000000000utac --gas 6000000 -y --output json | jq -r '.txhash')
echo "Waiting for transaction $TX_HASH to be included in a block..."
sleep 5
BTC_FINALITY_CONTRACT_CODE_ID=$($TACCHAIND query tx $TX_HASH --node $RPC_URL --output json | jq -r '.events[] | select(.type == "store_code") | .attributes[] | select(.key == "code_id") | .value' | tr -d '"')
if ! $TACCHAIND query wasm code $BTC_FINALITY_CONTRACT_CODE_ID temp.txt --node $RPC_URL; then
  echo "Failed to upload btc finality contract code."
  exit 1
fi

# Instantiate contracts
echo "Instantiating contracts..."
ADMIN=$(tacchaind keys show $OWNER_KEY --keyring-backend $KEYRING_BACKEND -a --home $HOMEDIR)
STAKING_MSG='{
  "admin": "'"$ADMIN"'"
}'
FINALITY_MSG='{
  "params": {
    "max_active_finality_providers": 100,
    "min_pub_rand": 1,
    "finality_inflation_rate": "0.035",
    "epoch_length": 10,
    "missed_blocks_window": 250,
    "jail_duration": 86400
  },
  "admin": "'"$ADMIN"'"
}'

echo BABYLON_CONTRACT_CODE_ID=$BABYLON_CONTRACT_CODE_ID
echo BTC_LIGHT_CLIENT_CONTRACT_CODE_ID=$BTC_LIGHT_CLIENT_CONTRACT_CODE_ID
echo BTC_STAKING_CONTRACT_CODE_ID=$BTC_STAKING_CONTRACT_CODE_ID
echo BTC_FINALITY_CONTRACT_CODE_ID=$BTC_FINALITY_CONTRACT_CODE_ID
echo STAKING_MSG="$STAKING_MSG"
echo FINALITY_MSG="$FINALITY_MSG"
echo ADMIN="$ADMIN"

# TODO: currently after initialising the node logs throw warning on each block "cannot get tx contracts from context module=x/wasm", seems to be from x/babylon abci.go?
$TACCHAIND tx babylon instantiate-babylon-contracts "$BABYLON_CONTRACT_CODE_ID" "$BTC_LIGHT_CLIENT_CONTRACT_CODE_ID" "$BTC_STAKING_CONTRACT_CODE_ID" "$BTC_FINALITY_CONTRACT_CODE_ID" "regtest" "01020304" 1 2 false "$STAKING_MSG" "$FINALITY_MSG" "test-consumer" "test-consumer-description" \
  --admin=$ADMIN \
  --ibc-transfer-channel-id=channel-0 \
  --from $OWNER_KEY \
  --home $HOMEDIR \
  --node $RPC_URL \
  --keyring-backend=$KEYRING_BACKEND \
  --gas 6000000 \
  --gas-prices 25000000000utac \
  -y

rm -rf temp.txt