package v105

// Upgrade v1.0.5: Migration of a compromised PeriodicVestingAccount.
//
// A vesting account's private key has been compromised. This upgrade handler
// migrates the full vesting state (schedule, delegations, rewards) to a new
// safe address and tombstones the old account so the attacker can no longer use it.

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/Asphere-xyz/tacchain/app/upgrades"

	"github.com/cosmos/cosmos-sdk/types/module"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const UpgradeName = "v1.0.5"

// Old account address
const OldAddress = "tac1vngy6vmurl9f8ck06hz3ezuk6v2ka8dx0sl8jx"

// New safe address
const NewAddress = "tac1ac9ythwz84l600832qupex0n534t0a2t8xh6av"

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
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		logger := sdkCtx.Logger()

		logger.Info("Starting v1.0.5 upgrade")

		if err := migrateVestingAccount(sdkCtx, ak, OldAddress, NewAddress); err != nil {
			return nil, fmt.Errorf("vesting account migration failed: %w", err)
		}

		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
