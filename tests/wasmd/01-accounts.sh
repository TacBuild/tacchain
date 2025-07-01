#!/bin/bash -e
set -o errexit -o nounset -o pipefail

HOMEDIR=.test-localnet-wasmd

BASE_ACCOUNT=$(tacchaind keys show validator -a --keyring-backend=test --home $HOMEDIR)
tacchaind q auth account "$BASE_ACCOUNT" -o json --home $HOMEDIR | jq

echo "## Add new account"
tacchaind keys add fred --keyring-backend=test --home $HOMEDIR

echo "## Check balance"
NEW_ACCOUNT=$(tacchaind keys show fred -a --keyring-backend=test --home $HOMEDIR)
tacchaind q bank balances "$NEW_ACCOUNT" -o json --home $HOMEDIR || true

echo "## Transfer tokens"
tacchaind tx bank send validator "$NEW_ACCOUNT" 1000000000utac --gas 1000000 --gas-prices 25000000000utac -y -b sync -o json --keyring-backend=test --home $HOMEDIR | jq
sleep 6

echo "## Check balance again"
tacchaind q bank balances "$NEW_ACCOUNT" -o json --home $HOMEDIR | jq
