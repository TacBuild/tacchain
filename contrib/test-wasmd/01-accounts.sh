#!/bin/bash
set -o errexit -o nounset -o pipefail

BASE_ACCOUNT=$(tacchaind keys show validator -a --keyring-backend=test)
tacchaind q auth account "$BASE_ACCOUNT" -o json | jq

echo "## Add new account"
tacchaind keys add fred --keyring-backend=test

echo "## Check balance"
NEW_ACCOUNT=$(tacchaind keys show fred -a --keyring-backend=test)
tacchaind q bank balances "$NEW_ACCOUNT" -o json || true

echo "## Transfer tokens"
tacchaind tx bank send validator "$NEW_ACCOUNT" 1000000000utac --gas 1000000 -y -b sync -o json --keyring-backend=test | jq
sleep 6

echo "## Check balance again"
tacchaind q bank balances "$NEW_ACCOUNT" -o json | jq
