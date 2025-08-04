#!/bin/bash -e

HOMEDIR=${HOMEDIR:-/home/tacchaind/.tacchaind}
RELAYER_ADDRESS=${RELAYER_ADDRESS:-tac1rt62vnvm008pay0g4rj58m5umq2jzzyyhvgvgz}
SENDER_KEY=${SENDER_KEY:-validator}
RPC_URL=${RPC_URL:-http://tacchaind:27657}
CHAIN_ID=${CHAIN_ID:-tacchain_2391-1}

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

tacchaind tx bank send $SENDER_KEY $RELAYER_ADDRESS 200000000000000000000000utac --keyring-backend test --home $HOMEDIR --node $RPC_URL --chain-id $CHAIN_ID --gas-prices 25000000000utac --yes
sleep 5

BALANCE=$(tacchaind q bank balances "$RELAYER_ADDRESS" -o json --node $RPC_URL | jq -r '.balances[] | select(.denom=="utac") | .amount')
echo "Balance: $BALANCE utac"
if [[ -z "$BALANCE" || "$BALANCE" -lt 200000000000000000000000 ]]; then
    echo "Error: Balance is less than 200000000000000000000000utac"
    exit 1
fi
