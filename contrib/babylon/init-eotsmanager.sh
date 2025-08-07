#!/bin/bash -e

HOMEDIR=${HOMEDIR:-/home/finality-provider/.eotsd}
CHAIN_ID=${CHAIN_ID:-babylon-localnet}
MNEMONIC=${MNEMONIC:-"glad slab inch unfold ticket lonely canyon gadget short eager chimney post baby round unknown upset village random club away voice obscure quote cheap"} # bbn1fezaz37eqxxgg5rzm3xg5u53qahp0q6ae55ktt / tac1vwkhvx42xjwktp6khs72ehrnawjyyz6v53ml5u

if ! eotsd keys show finality-provider --home "$HOMEDIR" --keyring-backend=test > /dev/null 2>&1; then
    echo "$MNEMONIC" | eotsd keys add finality-provider --keyring-backend=test --home "$HOMEDIR" --recover
fi