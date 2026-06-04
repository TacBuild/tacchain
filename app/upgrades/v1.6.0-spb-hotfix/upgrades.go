package v160spbhotfix

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/TacBuild/tacchain/app/upgrades"
	v160 "github.com/TacBuild/tacchain/app/upgrades/v1.6.0"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

const UpgradeName = "v1.6.0-spb-hotfix"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        storetypes.StoreUpgrades{},
}

func CreateUpgradeHandler(
	_ upgrades.ModuleManager,
	_ module.Configurator,
	ak *upgrades.AppKeepers,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		logger := sdkCtx.Logger()

		logger.Info("Starting v1.6.0 SPB hotfix upgrade")
		if err := v160.MigrateHistoricalGovEVMParamProposals(sdkCtx, ak); err != nil {
			return nil, fmt.Errorf("historical gov EVM params proposal migration failed: %w", err)
		}

		logger.Info("v1.6.0 SPB hotfix upgrade complete")
		return fromVM, nil
	}
}
