#!/bin/bash -e

# environment variables
TACCHAIND=${TACCHAIND:-$(which tacchaind)}
HOMEDIR=${HOMEDIR:-$HOME/.tacchaind}
OWNER_KEY=${OWNER_KEY:-validator}
KEYRING_BACKEND=${KEYRING_BACKEND:-test}
RPC_URL=${RPC_URL:-http://localhost:26657}

BABYLON_CONTRACT_FILE=${BABYLON_CONTRACT_FILE:-"./artifacts/contracts/babylon_contract_v0.14.0.wasm"}
BTC_LIGHT_CLIENT_CONTRACT_FILE=${BTC_LIGHT_CLIENT_CONTRACT_FILE:-"./artifacts/contracts/btc_light_client_v0.14.0.wasm"}
BTC_STAKING_CONTRACT_FILE=${BTC_STAKING_CONTRACT_FILE:-"./artifacts/contracts/btc_staking_v0.14.0.wasm"}
BTC_FINALITY_CONTRACT_FILE=${BTC_FINALITY_CONTRACT_FILE:-"./artifacts/contracts/btc_finality_v0.14.0.wasm"}

# TODO: check if already initialized

# upload babylon contract
echo "Uploading babylon contract code $BABYLON_CONTRACT_FILE..."
BABYLON_CONTRACT_CODE_ID=1
$TACCHAIND tx wasm store "$BABYLON_CONTRACT_FILE" --from $OWNER_KEY --keyring-backend $KEYRING_BACKEND --home $HOMEDIR --node $RPC_URL --gas-prices 25000000000utac --gas 20000000000 -y
sleep 5
if ! $TACCHAIND query wasm code $BABYLON_CONTRACT_CODE_ID --node $RPC_URL; then
  echo "Failed to upload babylon contract code."
  exit 1
fi

# upload btc light client contract
echo "Uploading btc light client contract code $BTC_LIGHT_CLIENT_CONTRACT_FILE..."
BTC_LIGHT_CLIENT_CONTRACT_CODE_ID=2
$TACCHAIND tx wasm store "$BTC_LIGHT_CLIENT_CONTRACT_FILE" --from $OWNER_KEY --keyring-backend $KEYRING_BACKEND --home $HOMEDIR --node $RPC_URL --gas-prices 25000000000utac --gas 20000000000 -y
sleep 5
if ! $TACCHAIND query wasm code $BTC_LIGHT_CLIENT_CONTRACT_CODE_ID --node $RPC_URL; then
  echo "Failed to upload btc light client contract code."
  exit 1
fi

# upload btc staking contract
echo "Uploading btc staking contract code $BTC_STAKING_CONTRACT_FILE..."
BTC_STAKING_CONTRACT_CODE_ID=3
$TACCHAIND tx wasm store "$BTC_STAKING_CONTRACT_FILE" --from $OWNER_KEY --keyring-backend $KEYRING_BACKEND --home $HOMEDIR --node $RPC_URL --gas-prices 25000000000utac --gas 20000000000 -y
sleep 5
if ! $TACCHAIND query wasm code $BTC_STAKING_CONTRACT_CODE_ID --node $RPC_URL; then
  echo "Failed to upload btc staking contract code."
  exit 1
fi

# upload btc finality contract
echo "Uploading btc finality contract code $BTC_FINALITY_CONTRACT_FILE..."
BTC_FINALITY_CONTRACT_CODE_ID=4
$TACCHAIND tx wasm store "$BTC_FINALITY_CONTRACT_FILE" --from $OWNER_KEY --keyring-backend $KEYRING_BACKEND --home $HOMEDIR --node $RPC_URL --gas-prices 25000000000utac --gas 20000000000 -y
sleep 5
if ! $TACCHAIND query wasm code $BTC_FINALITY_CONTRACT_CODE_ID --node $RPC_URL; then
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

$TACCHAIND tx babylon instantiate-babylon-contracts \
  $BABYLON_CONTRACT_CODE_ID \
  $BTC_LIGHT_CLIENT_CONTRACT_CODE_ID \
  $BTC_STAKING_CONTRACT_CODE_ID \
  $BTC_FINALITY_CONTRACT_CODE_ID \
  "regtest" \ 
  "01020304" \
  1 2 false \
  "$STAKING_MSG" \
  "$FINALITY_MSG" \
  "test-consumer" \
  "test-consumer-description" \
  --admin=$ADMIN \
  --ibc-transfer-channel-id=channel-0 \
  --from $OWNER_KEY \
  --home $HOMEDIR \
  --node $RPC_URL \
  --keyring-backend=$KEYRING_BACKEND \
  --gas 20000000000 \
  --gas-prices 25000000000utac \
  -y