// SPDX-License-Identifier: BUSL-1.1-or-later
// SPDX-FileCopyrightText: 2025 Web3 Technologies Inc. <https://asphere.xyz/>
// Copyright (c) 2025 Web3 Technologies Inc. All rights reserved.
// Use of this software is governed by the Business Source License included in the LICENSE file <https://github.com/Asphere-xyz/tacchain/blob/main/LICENSE>.
package main

import (
	"errors"
	"io"
	"os"

	cmtcli "github.com/cometbft/cometbft/libs/cli"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"cosmossdk.io/log"
	confixcmd "cosmossdk.io/tools/confix/cmd"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/pruning"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/snapshot"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"

	"github.com/Asphere-xyz/tacchain/app"

	evmclient "github.com/cosmos/evm/client"
	evmserver "github.com/cosmos/evm/server"
	evmsrvflags "github.com/cosmos/evm/server/flags"
)

func initRootCmd(appInstance *app.TacChainApp, rootCmd *cobra.Command) {
	cfg := sdk.GetConfig()
	cfg.Seal()

	rootCmd.AddCommand(
		evmclient.ValidateChainID(genutilcli.InitCmd(appInstance.BasicModuleManager, app.DefaultNodeHome)),
		genutilcli.Commands(appInstance.TxConfig(), appInstance.BasicModuleManager, app.DefaultNodeHome),
		cmtcli.NewCompletionCmd(rootCmd, true),
		debug.Cmd(),
		confixcmd.ConfigCommand(),
		pruning.Cmd(newApp, app.DefaultNodeHome),
		snapshot.Cmd(newApp),
	)

	// add Cosmos EVM' flavored TM commands to start server, etc.
	evmserver.AddCommands(
		rootCmd,
		evmserver.NewDefaultStartOptions(newApp, app.DefaultNodeHome),
		appExport,
		addModuleInitFlags,
	)

	// add Cosmos EVM key commands
	rootCmd.AddCommand(
		evmclient.KeyCommands(app.DefaultNodeHome, true),
	)

	// add keybase, auxiliary RPC, query, genesis, and tx child commands
	rootCmd.AddCommand(
		server.StatusCommand(),
		queryCommand(),
		txCommand(),
	)

	// add general tx flags to the root command
	var err error
	_, err = evmsrvflags.AddTxFlags(rootCmd)
	if err != nil {
		panic(err)
	}
}

func addModuleInitFlags(cmd *cobra.Command) {
	crisis.AddModuleInitFlags(cmd)
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.QueryEventForTxCmd(),
		rpc.ValidatorCommand(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
		server.QueryBlockCmd(),
		server.QueryBlockResultsCmd(),
	)

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		authcmd.GetSimulateCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

// newApp creates the application
func newApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	appOpts servertypes.AppOptions,
) servertypes.Application {
	baseappOptions := server.DefaultBaseappOptions(appOpts)

	return app.NewTacChainApp(
		logger,
		db,
		traceStore,
		true,
		cast.ToUint(appOpts.Get(server.FlagInvCheckPeriod)),
		appOpts,
		app.SetupEvmConfig,
		baseappOptions...,
	)
}

// appExport creates a new TacChain app (optionally at a given height) and exports state.
func appExport(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	height int64,
	forZeroHeight bool,
	jailAllowedAddrs []string,
	appOpts servertypes.AppOptions,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	var tacChainApp *app.TacChainApp
	// this check is necessary as we use the flag in x/upgrade.
	// we can exit more gracefully by checking the flag here.
	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, errors.New("application home is not set")
	}

	viperAppOpts, ok := appOpts.(*viper.Viper)
	if !ok {
		return servertypes.ExportedApp{}, errors.New("appOpts is not viper.Viper")
	}

	// overwrite the FlagInvCheckPeriod
	viperAppOpts.Set(server.FlagInvCheckPeriod, 1)
	appOpts = viperAppOpts

	tacChainApp = app.NewTacChainApp(
		logger,
		db,
		traceStore,
		height == -1,
		uint(1),
		appOpts,
		app.SetupEvmConfig,
		baseapp.SetChainID(cast.ToString(appOpts.Get(flags.FlagChainID))),
	)

	if height != -1 {
		if err := tacChainApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	}

	return tacChainApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}

func tempDir() string {
	dir, err := os.MkdirTemp("", ".tacchaind")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}

	return dir
}
