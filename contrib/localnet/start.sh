#!/bin/bash

CHAIN_ID=${CHAIN_ID:-tacchain_2391-1}
TACCHAIND=${TACCHAIND:-$(which tacchaind)}
HOMEDIR=${HOMEDIR:-$HOME/.tacchaind}

$TACCHAIND start --chain-id $CHAIN_ID --home $HOMEDIR
