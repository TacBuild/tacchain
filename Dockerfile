# docker build . -t tacchaind:latest
# docker run --rm -it tacchaind:latest tacchaind --help

FROM golang:1.23.8-alpine3.20 AS go-builder

# this comes from standard alpine nightly file
#  https://github.com/rust-lang/docker-rust-nightly/blob/master/alpine3.12/Dockerfile
# with some changes to support our toolchain, etc
RUN set -eux; apk add --no-cache ca-certificates build-base libusb-dev linux-headers;

WORKDIR /code
COPY . /code/

# See https://github.com/CosmWasm/wasmvm/releases
ADD https://github.com/CosmWasm/wasmvm/releases/download/v2.2.3/libwasmvm_muslc.aarch64.a /lib/libwasmvm_muslc.aarch64.a
ADD https://github.com/CosmWasm/wasmvm/releases/download/v2.2.3/libwasmvm_muslc.x86_64.a /lib/libwasmvm_muslc.x86_64.a
RUN sha256sum /lib/libwasmvm_muslc.aarch64.a | grep 6641730781bb1adc4bdf04a1e0f822b9ad4fb8ed57dcbbf575527e63b791ae41
RUN sha256sum /lib/libwasmvm_muslc.x86_64.a | grep 32503fe35a7be202c5f7c3051497d6e4b3cd83079a61f5a0bf72a2a455b6d820

# force it to use static lib (from above) not standard libgo_cosmwasm.so file
RUN LEDGER_ENABLED=true BUILD_TAGS=muslc LINK_STATICALLY=true make build
RUN echo "Ensuring binary is statically linked ..." \
    && (file /code/build/tacchaind | grep "statically linked")

# --------------------------------------------------------
FROM alpine:3.18

RUN apk update && apk add bash jq

COPY --from=go-builder /code/build/tacchaind /usr/bin/tacchaind

WORKDIR /opt

# rest server
EXPOSE 1317
# tendermint p2p
EXPOSE 26656
# tendermint rpc
EXPOSE 26657

CMD ["/usr/bin/tacchaind", "version"]
