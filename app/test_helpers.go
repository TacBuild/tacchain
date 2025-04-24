// SPDX-License-Identifier: BUSL-1.1-or-later
// SPDX-FileCopyrightText: 2025 Web3 Technologies Inc. <https://asphere.xyz/>
// Copyright (c) 2025 Web3 Technologies Inc. All rights reserved.
// Use of this software is governed by the Business Source License included in the LICENSE file <https://github.com/Asphere-xyz/tacchain/blob/main/LICENSE>.
package app

import (
	"testing"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	bam "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

var emptyWasmOpts []wasmkeeper.Option

// SetupOptions defines arguments that are passed into `TacChainApp` constructor.
type SetupOptions struct {
	Logger   log.Logger
	DB       *dbm.MemDB
	AppOpts  servertypes.AppOptions
	WasmOpts []wasmkeeper.Option
}

// NewTacChainAppWithCustomOptions initializes a new TacChainApp with custom options.
func NewTacChainAppWithCustomOptions(t *testing.T, isCheckTx bool, invCheckPeriod uint, options SetupOptions) *TacChainApp {
	t.Helper()

	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)
	// create validator set with single validator
	validator := cmttypes.NewValidator(pubKey, 1)
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{validator})

	// generate genesis account
	senderPrivKey := secp256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100000000000000))),
	}

	app := NewTacChainApp(
		options.Logger,
		options.DB,
		nil,
		true,
		invCheckPeriod,
		options.AppOpts,
		options.WasmOpts,
		SetupEvmConfig,
		bam.SetChainID(DefaultChainID),
	)
	genesisState := app.DefaultGenesis()
	genesisState, err = simtestutil.GenesisStateWithValSet(app.AppCodec(), genesisState, valSet, []authtypes.GenesisAccount{acc}, balance)
	require.NoError(t, err)

	if !isCheckTx {
		// init chain must be called to stop deliverState from being nil
		stateBytes, err := cmtjson.MarshalIndent(genesisState, "", " ")
		require.NoError(t, err)

		// Initialize the chain
		_, err = app.InitChain(&abci.RequestInitChain{
			ChainId:         DefaultChainID,
			Validators:      []abci.ValidatorUpdate{},
			ConsensusParams: simtestutil.DefaultConsensusParams,
			AppStateBytes:   stateBytes,
		})
		require.NoError(t, err)
	}

	return app
}
