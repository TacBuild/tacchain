# Tac Chain

`tacchaind` is a TAC EVM Node based on CosmosSDK with EVM support.

### Quickstart

- Prerequisites
  - [Go >= v1.23.6](https://go.dev/doc/install)

```sh
git clone https://github.com/TacBuild/tacchain.git
cd tacchain
make install # install the tacchaind binary
make localnet-init # initialize local chain
make localnet-start # start the chain
```

- Network RPC can be accessed at <http://0.0.0.0:26657>

- NOTE: `make install` will build the project and install the app binary to `$GOPATH/bin/tacchaind`. You can verify the installation using `tacchaind --help`.

- NOTE: `make localnet-init` initializes a new chain and generates network config folder at `$HOME/.tacchaind`. The generated folder is used to persist the network state. It's important to backup this folder accordingly. Note that this command removes any existing `$HOME/.tacchaind`! Only use it if you want to start a local network for the first time or you want to reset your chain's state!

### Join a public TAC Network

Learn more: [NETWORKS.md](NETWORKS.md#join-a-network)

### Using Docker

```sh
docker build . -t tacchaind:latest # build image
docker run --rm -it tacchaind:latest tacchaind --help # example binary usage
```

### TAC Address Converter

Check our [tool](./contrib/tac-address-converter/) for converting between EVM <> TAC addresses deterministically.

### Learn more

- [Cosmos SDK docs](https://docs.cosmos.network)
- [CosmosEVM docs](https://evm.cosmos.network/)

