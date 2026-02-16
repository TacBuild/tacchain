package app

import (
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/Asphere-xyz/tacchain/app/upgrades"
	v0010 "github.com/Asphere-xyz/tacchain/app/upgrades/v0.0.10"
	v0011 "github.com/Asphere-xyz/tacchain/app/upgrades/v0.0.11"
	v009 "github.com/Asphere-xyz/tacchain/app/upgrades/v0.0.9"
	v101 "github.com/Asphere-xyz/tacchain/app/upgrades/v1.0.1"
	v102 "github.com/Asphere-xyz/tacchain/app/upgrades/v1.0.2"
	v104 "github.com/Asphere-xyz/tacchain/app/upgrades/v1.0.4"
)

// Upgrades list of chain upgrades
var Upgrades = []upgrades.Upgrade{
	v009.Upgrade,
	v0010.Upgrade,
	v0011.Upgrade,
	v101.Upgrade,
	v102.Upgrade, // liquid stake
	v104.Upgrade, // ed25519 precompile
}

// RegisterUpgradeHandlers registers the chain upgrade handlers
func (app *TacChainApp) RegisterUpgradeHandlers() {
	keepers := upgrades.AppKeepers{
		AccountKeeper:         &app.AccountKeeper,
		ParamsKeeper:          &app.ParamsKeeper,
		ConsensusParamsKeeper: &app.ConsensusParamsKeeper,
		CapabilityKeeper:      app.CapabilityKeeper,
		IBCKeeper:             app.IBCKeeper,
		Codec:                 app.appCodec,
		GetStoreKey:           app.GetKey,
		LiquidStakeKeeper:     &app.LiquidStakeKeeper,
		BankKeeper:            app.BankKeeper,
		Erc20Keeper:           &app.Erc20Keeper,
		StakingKeeper:         app.StakingKeeper,
		EVMKeeper:             app.EVMKeeper,
	}
	app.GetStoreKeys()
	// register all upgrade handlers
	for _, upgrade := range Upgrades {
		app.UpgradeKeeper.SetUpgradeHandler(
			upgrade.UpgradeName,
			upgrade.CreateUpgradeHandler(
				app.ModuleManager,
				app.configurator,
				&keepers,
			),
		)
	}

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk %s", err))
	}

	if app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		return
	}

	// register store loader for current upgrade
	for _, upgrade := range Upgrades {
		if upgradeInfo.Name == upgrade.UpgradeName {
			app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &upgrade.StoreUpgrades))
			break
		}
	}
}
