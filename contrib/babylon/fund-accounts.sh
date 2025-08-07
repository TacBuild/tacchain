#!/bin/bash -e

BINARY=${BINARY:-$(which tacchaind)}
HOMEDIR=${HOMEDIR:-/home/tacchaind/.tacchaind}
SENDER_KEY=${SENDER_KEY:-validator}
CHAIN_ID=${CHAIN_ID:-tacchain_2391-1}
RPC_URL=${RPC_URL:-http://tacchaind:27657}
AMOUNT=${AMOUNT:-200000000000000000000000}
GAS_PRICES=${GAS_PRICES:-400000000000}
DENOM=${DENOM:-utac}
RELAYER_ADDRESS=${RELAYER_ADDRESS:-tac1rt62vnvm008pay0g4rj58m5umq2jzzyyhvgvgz}
FPD_ADDRESS=${FPD_ADDRESS:-tac1ng3kqnv7rjcf7cv2sesqyz6fln8gne4x95c5ju}
EOTSMANAGER_ADDRESS=${EOTSMANAGER_ADDRESS:-tac1vwkhvx42xjwktp6khs72ehrnawjyyz6v53ml5u}

# establish connection with the network
echo "Establishing connection with the network..."
timeout=120
elapsed=0
interval=2
while ! $BINARY query block --type=height 3 --node $RPC_URL > /dev/null 2>&1; do
  sleep $interval
  elapsed=$((elapsed + interval))
  if [ $elapsed -ge $timeout ]; then
    echo "Failed to establish connection with the network. Timeout waiting for block height 3"
    exit 1
  fi
done
echo "Connection established successfully."

$BINARY keys list --keyring-backend test --home $HOMEDIR

# fund IBC Relayer
echo "Funding IBC Relayer address $RELAYER_ADDRESS with $AMOUNT $DENOM"
$BINARY tx bank send $SENDER_KEY $RELAYER_ADDRESS ${AMOUNT}${DENOM} --keyring-backend test --home $HOMEDIR --node $RPC_URL --chain-id $CHAIN_ID --gas-prices ${GAS_PRICES}${DENOM} --yes
sleep 5

BALANCE=$($BINARY q bank balances "$RELAYER_ADDRESS" -o json --node $RPC_URL | jq -r ".balances[] | select(.denom==\"$DENOM\") | .amount")
echo "IBC Relayer Balance: $BALANCE $DENOM"
if [[ -z "$BALANCE" || "$BALANCE" -lt $AMOUNT ]]; then
    echo "Error: IBC Relayer Balance is less than ${AMOUNT}${DENOM}"
    exit 1
fi

# fund Finality Provider
echo "Funding Finality Provider address $FPD_ADDRESS with $AMOUNT $DENOM"
$BINARY tx bank send $SENDER_KEY $FPD_ADDRESS ${AMOUNT}${DENOM} --keyring-backend test --home $HOMEDIR --node $RPC_URL --chain-id $CHAIN_ID --gas-prices ${GAS_PRICES}${DENOM} --yes
sleep 5

BALANCE=$($BINARY q bank balances "$FPD_ADDRESS" -o json --node $RPC_URL | jq -r ".balances[] | select(.denom==\"$DENOM\") | .amount")
echo "Finality Provider Balance: $BALANCE $DENOM"
if [[ -z "$BALANCE" || "$BALANCE" -lt $AMOUNT ]]; then
    echo "Error: Finality Provider Balance is less than ${AMOUNT}${DENOM}"
    exit 1
fi

# fund Eotsmanager
echo "Funding Eotsmanager address $EOTSMANAGER_ADDRESS with $AMOUNT $DENOM"
$BINARY tx bank send $SENDER_KEY $EOTSMANAGER_ADDRESS ${AMOUNT}${DENOM} --keyring-backend test --home $HOMEDIR --node $RPC_URL --chain-id $CHAIN_ID --gas-prices ${GAS_PRICES}${DENOM} --yes
sleep 5

BALANCE=$($BINARY q bank balances "$EOTSMANAGER_ADDRESS" -o json --node $RPC_URL | jq -r ".balances[] | select(.denom==\"$DENOM\") | .amount")
echo "Eotsmanager Balance: $BALANCE $DENOM"
if [[ -z "$BALANCE" || "$BALANCE" -lt $AMOUNT ]]; then
    echo "Error: Eotsmanager Balance is less than ${AMOUNT}${DENOM}"
    exit 1
fi