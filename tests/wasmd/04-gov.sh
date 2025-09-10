#!/bin/bash -e
set -o errexit -o nounset -o pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

HOMEDIR=.test-wasmd

sleep 1
echo "## Submit a CosmWasm gov proposal"
RESP=$(tacchaind tx wasm submit-proposal store-instantiate "$DIR/testdata/reflect_2_0.wasm" \
  '{}' --label="testing" \
  --title "testing" --summary "Testing" --deposit "1000000000utac" \
  --admin $(tacchaind keys show -a validator --keyring-backend=test --home $HOMEDIR) \
  --amount 123utac \
  --keyring-backend=test \
  --gas 1500000 \
  --gas-prices 25000000000utac \
  --from validator -y --node=http://localhost:26657 -b sync -o json --home $HOMEDIR)
echo $RESP
sleep 6
tacchaind q tx $(echo "$RESP"| jq -r '.txhash') -o json --home $HOMEDIR | jq

