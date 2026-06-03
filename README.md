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

Use the built-in `tacchaind debug addr` command to convert between EVM hex
addresses and TAC bech32 account addresses deterministically.

EVM -> TAC:

```sh
tacchaind debug addr 0x123456789abcdef0123456789abcdef012345678 --prefix tac
# Bech32 tac1zg69v7y6hn00qy352euf40x77qfrg4nchk34lw
```

TAC -> EVM:

```sh
tacchaind debug addr tac1zg69v7y6hn00qy352euf40x77qfrg4nchk34lw
# Address hex: 0x123456789aBCdef0123456789AbCDEF012345678
```

`debug addr` works offline and does not require a running node. The EVM output is
EIP-55 checksummed; the lower-case form is the same address. To inspect the
configured TAC bech32 prefixes, run:

```sh
tacchaind debug prefixes
```

The repository also includes a small JavaScript/TypeScript address conversion
utility in [`contrib/tac-address-converter`](contrib/tac-address-converter).
It can be used directly from JS projects or as a reference implementation for
the same deterministic EVM hex <-> TAC bech32 conversion.

### Learn more

- [Cosmos SDK docs](https://docs.cosmos.network)
- [CosmosEVM docs](https://evm.cosmos.network/)
