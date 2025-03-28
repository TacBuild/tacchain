// SPDX-License-Identifier: BUSL-1.1-or-later
// SPDX-FileCopyrightText: 2025 Web3 Technologies Inc. <https://asphere.xyz/>
// Copyright (c) 2025 Web3 Technologies Inc. All rights reserved.
// Use of this software is governed by the Business Source License included in the LICENSE file <https://github.com/Asphere-xyz/tacchain/blob/main/LICENSE>.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"cosmossdk.io/log"
	"cosmossdk.io/simapp/params"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/Asphere-xyz/tacchain/app"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"

	etherminthd "github.com/evmos/ethermint/crypto/hd"
	ethermintserverflags "github.com/evmos/ethermint/server/flags"

	rosettaCmd "github.com/cosmos/rosetta/cmd"
)

// NewRootCmd creates a new root command for tacchaind. It is called once in the
// main function.
func NewRootCmd() *cobra.Command {
	cfg := sdk.GetConfig()
	cfg.Seal()

	tempApp := app.NewTacChainApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		0,
		simtestutil.NewAppOptionsWithFlagHome(tempDir()),
		[]wasmkeeper.Option{},
	)

	encodingConfig := params.EncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.TxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}

	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithHomeDir(app.DefaultNodeHome).
		WithKeyringOptions(etherminthd.EthSecp256k1Option()).
		WithViper(app.AppName)

	rootCmd := &cobra.Command{
		Use:   "tacchaind",
		Short: "Tac Chain Daemon (server)",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			initClientCtx, err = config.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}

			// This needs to go after ReadFromClientConfig, as that function
			// sets the RPC client needed for SIGN_MODE_TEXTUAL. This sign mode
			// is only available if the client is online.
			if !initClientCtx.Offline {
				enabledSignModes := append(tx.DefaultSignModes, signing.SignMode_SIGN_MODE_TEXTUAL)

				txConfigOpts := tx.ConfigOptions{
					EnabledSignModes:           enabledSignModes,
					TextualCoinMetadataQueryFn: txmodule.NewGRPCCoinMetadataQueryFn(initClientCtx),
				}

				txConfig, err := tx.NewTxConfigWithOptions(
					initClientCtx.Codec,
					txConfigOpts,
				)
				if err != nil {
					return err
				}

				initClientCtx = initClientCtx.WithTxConfig(txConfig)
			}

			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			customAppTemplate, customAppConfig := initAppConfig()
			customCMTConfig := initCometBFTConfig()

			return server.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, customCMTConfig)
		},
	}

	initRootCmd(tempApp, encodingConfig, rootCmd, encodingConfig.TxConfig, tempApp.BasicModuleManager)

	// add keyring to autocli opts
	autoCliOpts := tempApp.AutoCliOpts()
	initClientCtx, _ = config.ReadDefaultValuesFromDefaultClientConfig(initClientCtx)
	autoCliOpts.Keyring, _ = keyring.NewAutoCLIKeyring(initClientCtx.Keyring)
	autoCliOpts.ClientCtx = initClientCtx

	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}

	rootCmd, err := ethermintserverflags.AddTxFlags(rootCmd)
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(rosettaCmd.RosettaCommand(encodingConfig.InterfaceRegistry, encodingConfig.Codec))

	return rootCmd
}
