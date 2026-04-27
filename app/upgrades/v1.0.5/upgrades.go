package v105

// Upgrade v1.0.5 performs two independent migrations:
//
//  1. Vesting account rescue — a PeriodicVestingAccount's private key was
//     compromised. The handler moves the full vesting state (schedule,
//     delegations, unbondings, redelegations, rewards, balances) to a new
//     safe address and tombstones the old one.
//
//  2. EVM stack upgrade — tacchain migrates from cosmos/evm @ b1c973f
//     (evmos-based v0.1.4) to cosmos/evm v0.6.0.  The proto schema for
//     x/vm Params changed (field numbers shifted), x/erc20 native precompiles
//     moved to per-address KV keys, and a new EvmCoinInfo key was introduced.
//     All KV state is repaired before RunMigrations runs so that module
//     InitGenesis / BeginBlock code never sees stale bytes.

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmvmtypes "github.com/cosmos/evm/x/vm/types"
)

const UpgradeName = "v1.0.5"

// Old compromised account address.
const OldAddress = "tac1a0ef7a2ptywdq4s4034p3p88mnye29kpu4vx53"

// New safe address that receives the migrated vesting state.
const NewAddress = "tac1pnxs860d4zwpynamesfzejttms0w844xuxjd3n"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: storetypes.StoreUpgrades{
		// feeibc store key was removed when upgrading ibc-go v8 → v10.
		Deleted: []string{"feeibc"},
	},
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

		// ── Step 1: vesting account rescue ──────────────────────────────────
		logger.Info("Migrating compromised vesting account", "old", OldAddress, "new", NewAddress)
		if err := migrateVestingAccount(sdkCtx, ak, OldAddress, NewAddress); err != nil {
			return nil, fmt.Errorf("vesting account migration failed: %w", err)
		}

		// ── Step 2: EVM KV state repair (must happen before RunMigrations) ──
		//
		// The three sub-steps below fix incompatibilities between the old
		// evmos-based cosmos/evm @ b1c973f and the new cosmos/evm v0.6.0.
		// They all write directly to the KV store so that the module code
		// that runs inside RunMigrations (and later BeginBlock) reads
		// consistent data.

		// 2a. Re-encode x/vm Params.
		// Proto field numbers shifted: evm_channels 8→7, access_control 9→8,
		// active_static_precompiles 10→9.  Decoding old bytes with the new
		// schema without this step would produce garbage in those fields.
		logger.Info("Migrating x/vm Params proto schema")
		if err := migrateEVMParamsStore(sdkCtx, ak); err != nil {
			return nil, fmt.Errorf("x/vm params migration failed: %w", err)
		}

		// 2a2. Set history_serve_window to the default value (8192 / EIP-2935).
		// This is a new field in v0.6.0 that did not exist in v0.1.4, so it
		// stays 0 after the raw re-encoding above. Read → patch → write back.
		evmParams := ak.EVMKeeper.GetParams(sdkCtx)
		if evmParams.HistoryServeWindow == 0 {
			evmParams.HistoryServeWindow = evmvmtypes.DefaultHistoryServeWindow
			if err := ak.EVMKeeper.SetParams(sdkCtx, evmParams); err != nil {
				return nil, fmt.Errorf("x/vm SetParams (history_serve_window) failed: %w", err)
			}
			logger.Info("Set x/vm Params history_serve_window", "value", evmvmtypes.DefaultHistoryServeWindow)
		}

		// 2b. Initialise x/vm EvmCoinInfo.
		// This KV key ({0x05}) did not exist in v0.1.4.  The EVM module's
		// PreBlock calls SetGlobalConfigVariables which panics if the key is
		// absent.  InitEvmCoinInfo loads the data from x/bank denom metadata.
		logger.Info("Initialising x/vm EvmCoinInfo")
		if err := ak.EVMKeeper.InitEvmCoinInfo(sdkCtx); err != nil {
			return nil, fmt.Errorf("x/vm EvmCoinInfo init failed: %w", err)
		}

		// 2c. Set x/erc20 Params flag-keys.
		// In v0.6.0 params are stored as presence-flag keys, not protobuf.
		// EnableErc20 uses the same flag-key in both versions so it carries
		// over, but PermissionlessRegistration is a new key absent in v0.1.4.
		// Calling SetParams guarantees both keys are in the correct state.
		logger.Info("Setting x/erc20 Params")
		if err := ak.Erc20Keeper.SetParams(sdkCtx, erc20types.Params{
			EnableErc20:                true,
			PermissionlessRegistration: false,
		}); err != nil {
			return nil, fmt.Errorf("x/erc20 params migration failed: %w", err)
		}

		// 2d. Migrate x/erc20 native precompile addresses.
		// Old storage: one key "NativePrecompiles" → concat of 42-byte hex strings.
		// New storage: per-address keys {0x06}+hexAddr.
		// Without this step all native precompiles appear disabled after upgrade.
		logger.Info("Migrating x/erc20 native precompiles")
		if err := migrateERC20NativePrecompiles(sdkCtx, ak); err != nil {
			return nil, fmt.Errorf("x/erc20 native precompile migration failed: %w", err)
		}

		// ── Step 3: built-in SDK module migrations ───────────────────────────
		//
		// fromVM carries the old ConsensusVersions from the store:
		//   evm=9, erc20=4, feemarket=5  (evmos-based versions)
		// New cosmos/evm v0.6.0 resets all three to ConsensusVersion=1.
		// RunMigrations skips a module when toVersion <= 1, so downgrade is
		// safe (no-op).  We zero the entries explicitly to make the intent
		// clear and avoid any future surprises if that behaviour changes.
		fromVM["evm"] = 0
		fromVM["erc20"] = 0
		fromVM["feemarket"] = 0

		logger.Info("Running module migrations")
		vm, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, fmt.Errorf("RunMigrations failed: %w", err)
		}

		logger.Info("v1.0.5 upgrade complete")
		return vm, nil
	}
}
