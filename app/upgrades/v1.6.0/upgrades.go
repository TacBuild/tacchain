package v160

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	appconfig "github.com/TacBuild/tacchain/app/config"
	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmvmtypes "github.com/cosmos/evm/x/vm/types"
)

const UpgradeName = "v1.6.0"

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

		logger.Info("Starting v1.6.0 upgrade")

		// ── Step 1: vesting account rescue ──────────────────────────────────
		//
		// The rescue list (old → new pairs) is taken from the proposal's Plan.Info JSON.
		rescues, err := parseRescueEntries(plan.Info)
		if err != nil {
			return nil, fmt.Errorf("rescue config: %w", err)
		}
		logger.Info("Rescue plan parsed", "count", len(rescues))
		if err := preflightRescueEntries(sdkCtx, ak, rescues); err != nil {
			return nil, fmt.Errorf("rescue preflight: %w", err)
		}
		logger.Info("Rescue plan preflight passed", "count", len(rescues))
		for i, r := range rescues {
			logger.Info("Migrating compromised vesting account",
				"index", i, "old", r.Old, "new", r.New)
			if err := migrateVestingAccount(sdkCtx, ak, r.Old, r.New); err != nil {
				return nil, fmt.Errorf("vesting account migration failed for %s → %s: %w",
					r.Old, r.New, err)
			}
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

		// 2a1. Re-encode historical x/gov proposals that embed old x/vm
		// MsgUpdateParams payloads. Runtime x/vm state is fixed above, but gov
		// queries unpack proposal Any messages and would otherwise fail on the
		// old Params field 10 wire type.
		logger.Info("Migrating historical x/gov EVM MsgUpdateParams proposals")
		if err := MigrateHistoricalGovEVMParamProposals(sdkCtx, ak); err != nil {
			return nil, fmt.Errorf("historical gov EVM params proposal migration failed: %w", err)
		}

		// 2a2. Set history_serve_window to the default value (8192 / EIP-2935).
		// This is a new field in v0.6.0 that did not exist in v0.2.0, so it
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
		// This KV key ({0x05}) did not exist in v0.2.0.  The EVM module's
		// PreBlock calls SetGlobalConfigVariables which panics if the key is
		// absent.  InitEvmCoinInfo loads the data from x/bank denom metadata.
		if err := ensureEVMDenomMetadata(sdkCtx, ak, evmParams.EvmDenom); err != nil {
			return nil, fmt.Errorf("x/bank EVM denom metadata init failed: %w", err)
		}
		logger.Info("Initialising x/vm EvmCoinInfo")
		if err := ak.EVMKeeper.InitEvmCoinInfo(sdkCtx); err != nil {
			return nil, fmt.Errorf("x/vm EvmCoinInfo init failed: %w", err)
		}

		// 2c. Set x/erc20 Params flag-keys.
		// In v0.6.0 params are stored as presence-flag keys, not protobuf.
		// EnableErc20 uses the same flag-key in both versions so it carries
		// over, but PermissionlessRegistration is a new key absent in v0.2.0.
		// Calling SetParams guarantees both keys are in the correct state.
		logger.Info("Setting x/erc20 Params")
		if err := ak.Erc20Keeper.SetParams(sdkCtx, erc20types.Params{
			EnableErc20:                true,
			PermissionlessRegistration: false,
		}); err != nil {
			return nil, fmt.Errorf("x/erc20 params migration failed: %w", err)
		}

		// 2d. Migrate x/erc20 precompile addresses (native + dynamic).
		// Old storage: keys "NativePrecompiles" / "DynamicPrecompiles" → concat
		// of 42-byte hex strings.
		// New storage: per-address keys {0x06}+hexAddr (native) and
		// {0x07}+hexAddr (dynamic).
		// Without this step:
		//   * native precompiles (e.g. WTAC, staking, distribution, bank)
		//     appear disabled;
		//   * dynamic precompiles (every IBC / token-factory ERC20 wrapper
		//     registered via x/erc20 RegisterERC20) become invisible to EVM
		//     tooling, breaking all token transfers from the EVM side.
		logger.Info("Migrating x/erc20 precompiles (native + dynamic)")
		if err := migrateERC20Precompiles(sdkCtx, ak); err != nil {
			return nil, fmt.Errorf("x/erc20 precompile migration failed: %w", err)
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

		// ── Step 4: staking LSM accounting rebuild ─────────────────────────
		//
		// Rebuild derived staking LSM counters from delegation state rather
		// than asserting a narrow pre-upgrade shape. This keeps the upgrade
		// resilient to valid pre-upgrade LSM/validator-bond transactions.
		logger.Info("Rebuilding staking LSM accounting")
		if err := rebuildStakingLSMAccounting(sdkCtx, ak); err != nil {
			return nil, fmt.Errorf("staking LSM accounting rebuild failed: %w", err)
		}

		// ── Step 5: x/mint blocks_per_year correction ──────────────────────
		//
		// blocks_per_year was set assuming a 2.0s block time, but mainnet runs
		// at ~1.553s, so the chain over-mints relative to the nominal inflation
		// rate. Re-align per-block emission with the inflation rate. Runs after
		// RunMigrations so the mint module migration cannot overwrite it.
		logger.Info("Correcting x/mint blocks_per_year")
		if err := migrateMintBlocksPerYear(sdkCtx, ak); err != nil {
			return nil, fmt.Errorf("mint blocks_per_year correction failed: %w", err)
		}

		logger.Info("v1.6.0 upgrade complete")
		return vm, nil
	}
}

func ensureEVMDenomMetadata(ctx sdk.Context, ak *upgrades.AppKeepers, evmDenom string) error {
	if _, found := ak.BankKeeper.GetDenomMetaData(ctx, evmDenom); found {
		ctx.Logger().Info("x/bank EVM denom metadata already exists", "denom", evmDenom)
		return nil
	}

	metadata, found := appconfig.DefaultBankDenomMetadataFor(evmDenom)
	if !found {
		return fmt.Errorf("no default bank denom metadata for EVM denom %s", evmDenom)
	}
	if err := metadata.Validate(); err != nil {
		return fmt.Errorf("invalid EVM denom metadata for %s: %w", evmDenom, err)
	}

	ak.BankKeeper.SetDenomMetaData(ctx, metadata)
	ctx.Logger().Info("Set missing x/bank EVM denom metadata",
		"denom", evmDenom,
		"display", metadata.Display,
	)

	return nil
}
