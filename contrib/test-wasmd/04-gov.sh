#!/bin/bash
set -o errexit -o nounset -o pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

sleep 1
echo "## Submit a CosmWasm gov proposal"
RESP=$(tacchaind tx wasm submit-proposal store-instantiate "$DIR/testdata/reflect_2_0.wasm" \
  '{}' --label="testing" \
  --title "testing" --summary "Testing" --deposit "1000000000utac" \
  --admin $(tacchaind keys show -a validator --keyring-backend=test) \
  --amount 123utac \
  --keyring-backend=test \
  --gas 1500000 \
  --from validator -y --node=http://localhost:26657 -b sync -o json)
echo $RESP
sleep 6
tacchaind q tx $(echo "$RESP"| jq -r '.txhash') -o json | jq

