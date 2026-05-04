#!/bin/bash

# This script is used to generate a initial state and node docker images for local testing.

export TACCHAIND=/$PWD/build/tacchaind &&
export INITIAL_BALANCE=10000000000000000000000000000000000000 &&
export CHAIN_ID=tacchain_2391337-1 &&
export UNBONDING_TIME=60s &&
export GOV_TIME_SECONDS=300 &&
./contrib/localnet/init.sh

# ./build/tacchaind keys unsafe-export-eth-key validator
# export TACCHAIND=/$PWD/build/tacchaind && ./contrib/localnet/start.sh

COPYFILE_DISABLE=1 tar --no-xattrs --format=ustar -C "$HOME" -czf .tacchaind.tar .tacchaind

export WORKSPACE=../

docker buildx build \
  --platform linux/amd64 \
  --load \
  -t tacchaind:amd64 \
  -f $WORKSPACE/tacchain/Dockerfile.workspace \
  $WORKSPACE
docker save -o tacchaind-amd64.tar tacchaind:amd64

docker buildx build \
  --platform linux/arm64 \
  --load \
  -t tacchaind:arm64 \
  -f $WORKSPACE/tacchain/Dockerfile.workspace \
  $WORKSPACE
docker save -o tacchaind-arm64.tar tacchaind:arm64

gzip tacchaind-*.tar

