// SPDX-License-Identifier: BUSL-1.1-or-later
// SPDX-FileCopyrightText: 2025 Web3 Technologies Inc. <https://asphere.xyz/>
// Copyright (c) 2025 Web3 Technologies Inc. All rights reserved.
// Use of this software is governed by the Business Source License included in the LICENSE file <https://github.com/Asphere-xyz/tacchain/blob/main/LICENSE>.
package app

import (
	"fmt"
	"math/big"
	"os"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmdconfig "github.com/cosmos/evm/cmd/evmd/config"
	"github.com/cosmos/evm/evmd/eips"
	evmvmtypes "github.com/cosmos/evm/x/vm/types"
	evmvmcore "github.com/ethereum/go-ethereum/core/vm"
)

const (
	DisplayDenom  = "tac"
	BaseDenom     = "utac"
	BaseDenomUnit = 18

	// Bech32PrefixAccAddr defines the Bech32 prefix of an account's address.
	Bech32PrefixAccAddr = "tac"

	NodeDir        = ".tacchaind"
	AppName        = "TacChainApp"
	DefaultChainID = "tacchain_2391-1"
)

var (
	// Bech32PrefixAccPub defines the Bech32 prefix of an account's public key.
	Bech32PrefixAccPub = Bech32PrefixAccAddr + "pub"
	// Bech32PrefixValAddr defines the Bech32 prefix of a validator's operator address.
	Bech32PrefixValAddr = Bech32PrefixAccAddr + "valoper"
	// Bech32PrefixValPub defines the Bech32 prefix of a validator's operator public key.
	Bech32PrefixValPub = Bech32PrefixAccAddr + "valoperpub"
	// Bech32PrefixConsAddr defines the Bech32 prefix of a consensus node address.
	Bech32PrefixConsAddr = Bech32PrefixAccAddr + "valcons"
	// Bech32PrefixConsPub defines the Bech32 prefix of a consensus node public key.
	Bech32PrefixConsPub = Bech32PrefixAccAddr + "valconspub"

	DefaultNodeHome = os.ExpandEnv("$HOME/") + NodeDir

	// PowerReduction defines the default power reduction value for staking
	PowerReduction = sdkmath.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(BaseDenomUnit), nil))
)

func init() {
	registerDenoms()
	setAddressPrefixes()
}

var evmConfigSealed = false

func SetupEvmConfig(chainID string) error {
	if evmConfigSealed {
		return nil
	}

	baseDenom, err := sdk.GetBaseDenom()
	if err != nil {
		return fmt.Errorf("failed to get base denom: %s", err)
	}

	ethCfg := evmvmtypes.DefaultChainConfig(chainID)

	eips := map[int]func(*evmvmcore.JumpTable){
		0o000: eips.Enable0000,
		0o001: eips.Enable0001,
		0o002: eips.Enable0002,
	}
	err = evmvmtypes.NewEVMConfigurator().
		WithExtendedEips(eips).
		WithChainConfig(ethCfg).
		WithEVMCoinInfo(baseDenom, uint8(BaseDenomUnit)).
		Configure()
	if err != nil {
		return fmt.Errorf("failed to setup EVMConfigurator: %s", err)
	}

	evmConfigSealed = true
	return nil
}

// registerDenoms registers token denoms.
func registerDenoms() {
	sdk.DefaultBondDenom = BaseDenom
	sdk.DefaultPowerReduction = PowerReduction

	config := sdk.GetConfig()
	evmdconfig.SetBip44CoinType(config)

	if err := sdk.RegisterDenom(DisplayDenom, sdkmath.LegacyOneDec()); err != nil {
		panic(err)
	}

	if err := sdk.RegisterDenom(BaseDenom, sdkmath.LegacyNewDecWithPrec(1, BaseDenomUnit)); err != nil {
		panic(err)
	}
}

// setAddressPrefixes builds the Config with Bech32 addressPrefix and publKeyPrefix for accounts, validators, and consensus nodes and verifies that addreeses have correct format.
func setAddressPrefixes() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
}
