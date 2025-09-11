package v102

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/Asphere-xyz/tacchain/app/upgrades"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	evmerc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/ethereum/go-ethereum/common"
)

// UpgradeName defines the on-chain upgrade name
const UpgradeName = "v1.0.2"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: storetypes.StoreUpgrades{
		Added:   []string{},
		Deleted: []string{},
	},
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

		// cosmos/evm v0.4.1 upgrade migration
		store := runtime.NewKVStoreService(ak.GetStoreKey(evmerc20types.StoreKey)).OpenKVStore(ctx)
		const addressLength = 42
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		sdkCtx.Logger().Info("Upgrade v1.0.2 started")

		// Migrate dynamic precompiles
		if oldData, _ := store.Get([]byte("DynamicPrecompiles")); len(oldData) > 0 {
			for i := 0; i < len(oldData); i += addressLength {
				address := common.HexToAddress(string(oldData[i : i+addressLength]))
				ak.ERC20Keeper.SetDynamicPrecompile(sdkCtx, address)
			}
			store.Delete([]byte("DynamicPrecompiles"))
		}

		// Migrate native precompiles
		if oldData, _ := store.Get([]byte("NativePrecompiles")); len(oldData) > 0 {
			for i := 0; i < len(oldData); i += addressLength {
				address := common.HexToAddress(string(oldData[i : i+addressLength]))
				ak.ERC20Keeper.SetNativePrecompile(sdkCtx, address)
			}
			store.Delete([]byte("NativePrecompiles"))
		}

		sdkCtx.Logger().Info("Upgrade v1.0.2 completed")
		return vm, nil
	}
}
