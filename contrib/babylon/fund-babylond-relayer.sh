#!/bin/bash -e

HOMEDIR=${HOMEDIR:-/data}
RELAYER_ADDRESS=${RELAYER_ADDRESS:-bbn1mjyd6ksf0ay5j5ta3x7hqeptxs6j3cr9lawv3d}
SENDER_KEY=${SENDER_KEY:-test-spending-key}
RPC_URL=${RPC_URL:-http://babylond:26657}
CHAIN_ID=${CHAIN_ID:-babylon-localnet}

# wait for network to start
echo "Waiting for network to start"
timeout=120
elapsed=0
interval=2
while ! babylond query block --type=height 3 --node $RPC_URL > /dev/null 2>&1; do
  sleep $interval
  elapsed=$((elapsed + interval))
  if [ $elapsed -ge $timeout ]; then
    echo "Failed to start network. Timeout waiting for block height 3"
    exit 1
  fi
done
echo "Network started successfully"

babylond keys list --keyring-backend test --home $HOMEDIR

babylond tx bank send $SENDER_KEY $RELAYER_ADDRESS 100000000000ubbn --keyring-backend test --home $HOMEDIR --node $RPC_URL --chain-id $CHAIN_ID --gas-prices 1ubbn --yes
sleep 5

BALANCE=$(babylond q bank balances "$RELAYER_ADDRESS" -o json --node $RPC_URL | jq -r '.balances[] | select(.denom=="ubbn") | .amount')
echo "Balance: $BALANCE ubbn"
if [[ -z "$BALANCE" || "$BALANCE" -lt 100000000000 ]]; then
    echo "Error: Balance is less than 100000000000ubbn"
    exit 1
fi
