#!/usr/bin/make -f

VERSION := $(shell echo $(shell git describe --tags --always) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')
LEDGER_ENABLED ?= true
BINDIR ?= $(GOPATH)/bin
SIMAPP = ./app

export GO111MODULE = on

# process build tags

build_tags = netgo
ifeq ($(LEDGER_ENABLED),true)
  ifeq ($(OS),Windows_NT)
    GCCEXE = $(shell where gcc.exe 2> NUL)
    ifeq ($(GCCEXE),)
      $(error gcc.exe not installed for ledger support, please install or set LEDGER_ENABLED=false)
    else
      build_tags += ledger
    endif
  else
    UNAME_S = $(shell uname -s)
    ifeq ($(UNAME_S),OpenBSD)
      $(warning OpenBSD detected, disabling ledger support (https://github.com/cosmos/cosmos-sdk/issues/1988))
    else
      GCC = $(shell command -v gcc 2> /dev/null)
      ifeq ($(GCC),)
        $(error gcc not installed for ledger support, please install or set LEDGER_ENABLED=false)
      else
        build_tags += ledger
      endif
    endif
  endif
endif

ifeq ($(WITH_CLEVELDB),yes)
  build_tags += gcc
endif
build_tags += $(BUILD_TAGS)
build_tags := $(strip $(build_tags))

whitespace :=
empty = $(whitespace) $(whitespace)
comma := ,
build_tags_comma_sep := $(subst $(empty),$(comma),$(build_tags))

# process linker flags

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=tacchain \
		  -X github.com/cosmos/cosmos-sdk/version.AppName=tacchaind \
		  -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
		  -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
		  -X "github.com/cosmos/cosmos-sdk/version.BuildTags=$(build_tags_comma_sep)"

ifeq ($(WITH_CLEVELDB),yes)
  ldflags += -X github.com/cosmos/cosmos-sdk/types.DBBackend=cleveldb
endif
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

BUILD_FLAGS := -tags "$(build_tags_comma_sep)" -ldflags '$(ldflags)' -trimpath

###############################################################################
###                                  Build                                  ###
###############################################################################

all: install test

install: go.sum
	go install -mod=readonly $(BUILD_FLAGS) ./cmd/tacchaind

build: go.sum
ifeq ($(OS),Windows_NT)
	$(error tacchaind server not supported. Use "make build-windows-client" for client)
	exit 1
else
	go build -mod=readonly $(BUILD_FLAGS) -o build/tacchaind ./cmd/tacchaind
endif

build-windows-client: go.sum
	GOOS=windows GOARCH=amd64 go build -mod=readonly $(BUILD_FLAGS) -o build/tacchaind.exe ./cmd/tacchaind

go.sum: go.mod
	@echo "--> Ensure dependencies have not been modified"
	@go mod verify

clean:
	rm -rf build/

###############################################################################
###                                 Tests                                   ###
###############################################################################

test: test-unit test-race test-e2e test-localnet-params test-localnet-evm test-ledger test-solidity

test-unit:
	@VERSION=$(VERSION) go test -mod=readonly -tags='ledger test_ledger_mock' -v $(shell go list ./... | grep -v "tests")

test-race:
	@VERSION=$(VERSION) go test -mod=readonly -race -tags='ledger test_ledger_mock' ./...

test-e2e:
	@VERSION=$(VERSION) go test -mod=readonly -tags='ledger test_ledger_mock' -v ./tests/e2e/...

test-cover:
	@go test -mod=readonly -timeout 30m -race -coverprofile=coverage.txt -covermode=atomic -tags='ledger test_ledger_mock' ./...

test-benchmark:
	@go test -mod=readonly -bench=. ./...

test-localnet-params:
	./tests/localnet/test-params.sh

test-localnet-evm:
	./tests/localnet/test-evm.sh

test-ledger:
	@VERSION=$(VERSION) go test -mod=readonly -tags='ledger test_ledger_mock' -v ./tests/ledger/...

test-solidity:
	./tests/solidity/run-solidity-tests.sh

###############################################################################
###                                Networks                                 ###
###############################################################################

TACCHAIND := $(shell which tacchaind)

localnet: install localnet-init localnet-start
testnet: install testnet-init

localnet-init:
	./contrib/localnet/init.sh

localnet-init-multi-node:
	./contrib/localnet/init-multi-node.sh

localnet-start:
	./contrib/localnet/start.sh

