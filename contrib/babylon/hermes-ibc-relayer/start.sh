#!/bin/sh -e

if ! hermes keys list --chain babylon-localnet | grep -q 'testkey'; then
    hermes keys add --chain babylon-localnet --mnemonic-file "/home/hermes-ibc-relayer/mnemonic.txt" --hd-path "m/44'/60'/0'/0/0"
fi

if ! hermes keys list --chain tacchain_2391-1 | grep -q 'testkey'; then
    hermes keys add --chain tacchain_2391-1 --mnemonic-file "/home/hermes-ibc-relayer/mnemonic.txt" --hd-path "m/44'/60'/0'/0/0"
fi

echo y | hermes create channel --a-chain babylon-localnet --b-chain tacchain_2391-1 --a-port transfer --b-port transfer --new-client-connection
hermes start

