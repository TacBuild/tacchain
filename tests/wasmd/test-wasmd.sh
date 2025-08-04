#!/bin/bash -e

export HOMEDIR=.test-wasmd
export MIN_GAS_PRICE=25000000000

# start new network
echo "Starting localnet"
echo y | make localnet > /dev/null 2>&1 &

# wait for network to start
echo "Waiting for network to start"
timeout=120
elapsed=0
interval=2
while ! tacchaind query block --type=height 3 --node http://localhost:26657 > /dev/null 2>&1; do
  sleep $interval
  elapsed=$((elapsed + interval))
  if [ $elapsed -ge $timeout ]; then
    echo "Failed to start network. Timeout waiting for block height 3"

    killall tacchaind
    exit 1
  fi
done
echo "Network started successfully"

# test accounts
echo "Testing accounts"
if ! $(dirname "$0")/01-accounts.sh; then
  echo "Accounts test failed"
  killall tacchaind
  exit 1
fi
echo "Accounts test passed successfully"

# test contracts
echo "Testing contracts"
if ! $(dirname "$0")/02-contracts.sh; then
  echo "Contracts test failed"
  killall tacchaind
  exit 1
fi
echo "Contracts test passed successfully"

# test gRPC queries
echo "Testing gRPC queries"
if ! $(dirname "$0")/03-grpc-queries.sh; then
  echo "gRPC queries test failed"
  killall tacchaind
  exit 1
fi
echo "gRPC queries test passed successfully"

# test governance
echo "Testing governance"
if ! $(dirname "$0")/04-gov.sh; then
  echo "Governance test failed"
  killall tacchaind
  exit 1
fi
echo "Governance test passed successfully"

killall tacchaind
echo "All tests passed successfully"
