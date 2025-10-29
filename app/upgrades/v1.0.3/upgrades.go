package v103

import (
	"context"
	"slices"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Asphere-xyz/tacchain/app/upgrades"
	"github.com/cosmos/cosmos-sdk/types/module"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

const UpgradeName = "v1.0.3"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        storetypes.StoreUpgrades{},
}

func CreateUpgradeHandler(
	mm upgrades.ModuleManager,
	configurator module.Configurator,
	ak *upgrades.AppKeepers,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		vm, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return vm, err
		}

		sdkCtx := sdk.UnwrapSDKContext(ctx)

		evmParams := ak.EVMKeeper.GetParams(sdkCtx)
		target := evmtypes.Ed25519PrecompileAddress
		if !slices.Contains(evmParams.ActiveStaticPrecompiles, target) {
			evmParams.ActiveStaticPrecompiles = append(evmParams.ActiveStaticPrecompiles, target)
			if err := ak.EVMKeeper.SetParams(sdkCtx, evmParams); err != nil {
				return vm, err
			}
		}
		return vm, nil
	}
}
