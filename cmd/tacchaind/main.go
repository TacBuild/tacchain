package main

import (
	"os"

	"cosmossdk.io/log"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	sdk "github.com/cosmos/cosmos-sdk/types"

	appconfig "github.com/TacBuild/tacchain/app/config"
)

func main() {
	setupSDKConfig()

	rootCmd := NewRootCmd()

	if err := svrcmd.Execute(rootCmd, "", appconfig.DefaultNodeHome); err != nil {
		log.NewLogger(rootCmd.OutOrStderr()).Error("failure when running app", "err", err)
		os.Exit(1)
	}
}

func setupSDKConfig() {
	appconfig.SetupSDKConfig()
	sdk.GetConfig().Seal()
}
