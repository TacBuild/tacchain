#!/bin/bash -e

HOMEDIR=${HOMEDIR:-/home/finality-provider/.fpd}
CHAIN_ID=${CHAIN_ID:-tacchain_2391-1}
MNEMONIC=${MNEMONIC:-"city brick always tower shallow enemy alien sauce galaxy lonely where clown bronze garment grain genius summer program lounge hip infant scene outdoor door"} # bbn1jswmc6x22vlgs507n2wuqmparcqun67v0mg93p / tac1ng3kqnv7rjcf7cv2sesqyz6fln8gne4x95c5ju
BTC_PUBLIC_KEY=${BTC_PUBLIC_KEY:-"b5c03bcf902456e16b9beee180d0c71f6e293549759a4f96f7c5638beecd185c"}

echo $MNEMONIC | fpd keys add finality-provider --keyring-backend test --recover

# TODO link with eotsmanager

fpd cfp \
        --key-name finality-provider \
        --chain-id $CHAIN_ID \
        --eots-pk $BTC_PUBLIC_KEY \
        --commission-rate 0.05 \
        --commission-max-rate 0.20 \
        --commission-max-change-rate 0.01 \
        --moniker \"Babylon finality provider\" 2>&1"