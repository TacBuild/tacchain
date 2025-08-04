#!/bin/bash -e

HOMEDIR=${HOMEDIR:-/data/node0/babylond}
IP_ADDRESS=${IP_ADDRESS:-192.168.10.2}
CHAIN_ID=${CHAIN_ID:-babylon-localnet}

babylond testnet \
  --v 1 \
  -o /data \
  --starting-ip-address $IP_ADDRESS \
  --keyring-backend=test \
  --chain-id $CHAIN_ID \
  --epoch-interval 10 \
  --btc-finalization-timeout 2 \
  --btc-confirmation-depth 1 \
  --minimum-gas-prices 1ubbn \
  --btc-base-header 0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4adae5494dffff7f2002000000 \
  --btc-network regtest \
  --additional-sender-account \
  --slashing-pk-script "76a914010101010101010101010101010101010101010188ac" \
  --slashing-rate 0.1 \
  --min-staking-time-blocks 10 \
  --min-commission-rate 0.05 \
  --covenant-quorum 1 \
  --activation-height 39 \
  --unbonding-time 5 \
  --covenant-pks 2d4ccbe538f846a750d82a77cd742895e51afcf23d86d05004a356b783902748