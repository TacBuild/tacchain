# docker build . -t tacchaind:latest
# docker run --rm -it tacchaind:latest tacchaind --help

FROM golang:1.23.8-alpine3.20 AS go-builder

# this comes from standard alpine nightly file
#  https://github.com/rust-lang/docker-rust-nightly/blob/master/alpine3.12/Dockerfile
# with some changes to support our toolchain, etc
RUN set -eux; apk add --no-cache ca-certificates build-base libusb-dev linux-headers;

WORKDIR /code
COPY . /code/

RUN LEDGER_ENABLED=true make build


# --------------------------------------------------------
FROM alpine:3.18

COPY --from=go-builder /code/build/tacchaind /usr/bin/tacchaind

WORKDIR /opt

# rest server
EXPOSE 1317
# tendermint p2p
EXPOSE 26656
# tendermint rpc
EXPOSE 26657

CMD ["/usr/bin/tacchaind", "version"]
