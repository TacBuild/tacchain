#!/bin/bash -e
set -o errexit -o nounset -o pipefail

HOMEDIR=.test-wasmd

BASE_ACCOUNT=$(tacchaind keys show validator -a --keyring-backend=test --home $HOMEDIR)
tacchaind q auth account "$BASE_ACCOUNT" -o json --home $HOMEDIR | jq

echo "## Add new account"
tacchaind keys add fred --keyring-backend=test --home $HOMEDIR

echo "## Check balance"
NEW_ACCOUNT=$(tacchaind keys show fred -a --keyring-backend=test --home $HOMEDIR)
tacchaind q bank balances "$NEW_ACCOUNT" -o json --home $HOMEDIR || true

echo "## TEMP Feemarket params"
tacchaind q feemarket params  -o json --home $HOMEDIR | jq

echo "## Transfer tokens"
tacchaind tx bank send validator "$NEW_ACCOUNT" 50000000000000000utac --gas-prices 25000000000utac -y -b sync -o json --keyring-backend=test --home $HOMEDIR | jq
sleep 6

echo "## Check balance again"
BALANCE=$(tacchaind q bank balances "$NEW_ACCOUNT" -o json --home $HOMEDIR | jq -r '.balances[] | select(.denom=="utac") | .amount')
echo "Balance: $BALANCE utac"
if [[ -z "$BALANCE" || "$BALANCE" -lt 50000000000000000 ]]; then
    echo "Error: Balance is less than 50000000000000000 utac"
    exit 1
fi
