package v166

import (
	"context"
	"fmt"
	"slices"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

const UpgradeName = "v1.6.6"

const legacyEvidencePrecompileAddress = "0x0000000000000000000000000000000000000807"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        storetypes.StoreUpgrades{},
}

// CreateUpgradeHandler runs module migrations and updates the active precompile list.
func CreateUpgradeHandler(
	mm upgrades.ModuleManager,
	configurator module.Configurator,
	ak *upgrades.AppKeepers,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		logger := sdkCtx.Logger().With("upgrade", UpgradeName)
		logger.Info("Starting upgrade")

		vm, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return vm, fmt.Errorf("module migrations failed: %w", err)
		}

		params := ak.EVMKeeper.GetParams(sdkCtx)
		if slices.Contains(params.ActiveStaticPrecompiles, legacyEvidencePrecompileAddress) {
			params.ActiveStaticPrecompiles = slices.DeleteFunc(params.ActiveStaticPrecompiles, func(address string) bool {
				return address == legacyEvidencePrecompileAddress
			})
			if err := ak.EVMKeeper.SetParams(sdkCtx, params); err != nil {
				return vm, fmt.Errorf("update EVM parameters: %w", err)
			}
		}

		if sdkCtx.ChainID() == accountMigrationChainID {
			runAccountMigration(sdkCtx.WithLogger(logger), ak)
		}

		logger.Info("Upgrade complete")
		return vm, nil
	}
}
