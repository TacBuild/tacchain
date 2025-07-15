#!/usr/bin/env bash
set -e

echo "BITCOIN_NETWORK: $BITCOIN_NETWORK"
echo "BITCOIN_RPC_PORT: $BITCOIN_RPC_PORT"

if [[ -z "$BITCOIN_NETWORK" ]]; then
  BITCOIN_NETWORK="regtest"
fi

if [[ -z "$BITCOIN_RPC_PORT" ]]; then
  BITCOIN_RPC_PORT="18443"
fi

if [[ "$BITCOIN_NETWORK" != "regtest" && "$BITCOIN_NETWORK" != "signet" ]]; then
  echo "Unsupported network: $BITCOIN_NETWORK"
  exit 1
fi

# Create bitcoin data directory and initialize bitcoin configuration file.
mkdir -p "$BITCOIN_DATA"
cat <<EOF > "$BITCOIN_CONF"
# Enable ${BITCOIN_NETWORK} mode.
${BITCOIN_NETWORK}=1

# Accept command line and JSON-RPC commands
server=1

# RPC user and password.
rpcuser=$RPC_USER
rpcpassword=$RPC_PASS

# ZMQ notification options.
# Enable publish hash block and tx sequence
zmqpubsequence=tcp://*:$ZMQ_SEQUENCE_PORT
# Enable publishing of raw block hex.
zmqpubrawblock=tcp://*:$ZMQ_RAWBLOCK_PORT
# Enable publishing of raw transaction.
zmqpubrawtx=tcp://*:$ZMQ_RAWTR_PORT

txindex=1
deprecatedrpc=create_bdb

# Fallback fee
fallbackfee=0.00001

# Allow all IPs to access the RPC server.
[${BITCOIN_NETWORK}]
rpcbind=0.0.0.0
rpcallowip=0.0.0.0/0
EOF

echo "Starting bitcoind..."
bitcoind -${BITCOIN_NETWORK} -datadir="$BITCOIN_DATA" -conf="$BITCOIN_CONF" -daemon
# Allow some time for bitcoind to start
sleep 3

if [[ "$BITCOIN_NETWORK" == "regtest" ]]; then
  echo "Creating a wallet..."
  bitcoin-cli -${BITCOIN_NETWORK} -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASS" createwallet "$WALLET_NAME" false false "$WALLET_PASS" false false

  echo "Creating a wallet for btcstaker..."
  bitcoin-cli -${BITCOIN_NETWORK} -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASS" createwallet "$BTCSTAKER_WALLET_NAME" false false "$WALLET_PASS" false false

  echo "Generating 110 blocks for the first coinbases to mature..."
  bitcoin-cli -regtest -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASS" -rpcwallet="$WALLET_NAME" -generate 110

  echo "Creating $BTCSTAKER_WALLET_ADDR_COUNT addresses for btcstaker..."
  BTCSTAKER_ADDRS=()
  for i in `seq 0 1 $((BTCSTAKER_WALLET_ADDR_COUNT - 1))`
  do
    BTCSTAKER_ADDRS+=($(bitcoin-cli -regtest -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASS" -rpcwallet="$BTCSTAKER_WALLET_NAME" getnewaddress))
  done

  # Generate a UTXO for each btc-staker address
  bitcoin-cli -regtest -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASS" -rpcwallet="$WALLET_NAME" walletpassphrase "$WALLET_PASS" 1
  for addr in "${BTCSTAKER_ADDRS[@]}"
  do
    bitcoin-cli -regtest -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASS" -rpcwallet="$WALLET_NAME" sendtoaddress "$addr" 10
  done

  # Allow some time for the wallet to catch up.
  sleep 5

  echo "Checking balance..."
  bitcoin-cli -regtest -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASS" -rpcwallet="$WALLET_NAME" getbalance
  
  echo "Generating a block every ${GENERATE_INTERVAL_SECS} seconds."
  echo "Press [CTRL+C] to stop..."
  while true
  do  
    bitcoin-cli -regtest -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASS" -rpcwallet="$WALLET_NAME" -generate 1
    if [[ "$GENERATE_STAKER_WALLET" == "true" ]]; then
      echo "Periodically send funds to btcstaker addresses..."
      bitcoin-cli -regtest -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASS" -rpcwallet="$WALLET_NAME" walletpassphrase "$WALLET_PASS" 10
      for addr in "${BTCSTAKER_ADDRS[@]}"
      do
        bitcoin-cli -regtest -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASS" -rpcwallet="$WALLET_NAME" sendtoaddress "$addr" 10
      done
    fi
    sleep "${GENERATE_INTERVAL_SECS}"
  done
elif [[ "$BITCOIN_NETWORK" == "signet" ]]; then
  # Check if the wallet database already exists.
  if [[ -d "$BITCOIN_DATA"/signet/wallets/"$BTCSTAKER_WALLET_NAME" ]]; then
    echo "Wallet already exists and removing it..."
    rm -rf "$BITCOIN_DATA"/signet/wallets/"$BTCSTAKER_WALLET_NAME"
  fi
  # Keep the container running
  echo "Bitcoind is running. Press CTRL+C to stop..."
  tail -f /dev/null
fi