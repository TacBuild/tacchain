#!/bin/bash -e

if ! command -v cast >/dev/null 2>&1; then
  echo "Error: Foundry (cast) is not installed. Please install it from https://book.getfoundry.sh/getting-started/installation"
  exit 1
fi

cleanup() {
  killall tacchaind >/dev/null 2>&1 || true
}
trap cleanup EXIT

export HOMEDIR=.test-localnet-evm
export CHAIN_ID=tacchain_2391-1
export MIN_GAS_PRICE=25000000000
export TACCHAIND=${TACCHAIND:-$(command -v tacchaind 2>/dev/null || echo ./build/tacchaind)}

tacchaind() {
  "$TACCHAIND" "$@"
}

EVM_CHAIN_ID=$(echo "$CHAIN_ID" | sed -E 's/.*_([0-9]+)-.*/\1/')
if [[ -z "$EVM_CHAIN_ID" ]]; then
  echo "Invalid CHAIN_ID format. Expected format: <any_string>_<number>-<number>"
  exit 1
fi

echo "Starting localnet"
echo y | make localnet > /dev/null 2>&1 &

echo "Waiting for network to start"
timeout=120
elapsed=0
interval=2
while ! tacchaind query block --type=height 3 --node http://localhost:26657 > /dev/null 2>&1; do
  sleep $interval
  elapsed=$((elapsed + interval))
  if [ $elapsed -ge $timeout ]; then
    echo "Failed to start network. Timeout waiting for block height 3"
    exit 1
  fi
done
echo "Network started successfully"

echo "Sending protected EVM transaction"
validator_private_key=$(tacchaind keys unsafe-export-eth-key validator --home "$HOMEDIR" --keyring-backend test | tail -n 1)
tx_hash=$(cast send 0x000000000000000000000000000000000000dEaD \
  --value 1 \
  --private-key "$validator_private_key" \
  --rpc-url http://localhost:8545 \
  --chain "$EVM_CHAIN_ID" \
  --gas-price "$MIN_GAS_PRICE" \
  --legacy \
  --async | tail -n 1 | tr -d '\r')

if [[ ! "$tx_hash" =~ ^0x[0-9a-fA-F]{64}$ ]]; then
  echo "Failed to send EVM transaction"
  echo "Got: $tx_hash"
  exit 1
fi
echo "EVM transaction sent: $tx_hash"

echo "Waiting for EVM receipt"
tx_receipt="null"
for _ in $(seq 1 30); do
  tx_receipt=$(curl -s http://localhost:8545 \
    -X POST \
    -H "Content-Type: application/json" \
    --data "{\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$tx_hash\"],\"id\":1,\"jsonrpc\":\"2.0\"}" | jq -S -c '.result')
  if [[ "$tx_receipt" != "null" ]]; then
    break
  fi
  sleep 2
done

if [[ "$tx_receipt" == "null" ]]; then
  echo "Failed to get EVM transaction receipt"
  exit 1
fi

block_height=$(echo "$tx_receipt" | jq -r '.blockNumber')
if [[ "$block_height" == "null" || -z "$block_height" ]]; then
  echo "Failed to get receipt block number"
  echo "Receipt: $tx_receipt"
  exit 1
fi

echo "Verifying eth_getBlockReceipts"
block_receipts=$(curl -s http://localhost:8545 \
  -X POST \
  -H "Content-Type: application/json" \
  --data "{\"method\":\"eth_getBlockReceipts\",\"params\":[\"$block_height\"],\"id\":1,\"jsonrpc\":\"2.0\"}" | jq -S -c '.result[0]')

if [ "$block_receipts" != "$tx_receipt" ]; then
  echo "Failed to verify eth_getBlockReceipts"
  echo "Expected: $tx_receipt"
  echo "Got:      $block_receipts"
  exit 1
fi

echo "All tests passed successfully"
