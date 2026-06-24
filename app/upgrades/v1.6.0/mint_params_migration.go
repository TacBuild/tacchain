package v160

// mint_params_migration.go corrects the x/mint blocks_per_year parameter.
//
// blocks_per_year was originally set to 15_768_000, which assumes a 2.0s block
// time (365 * 86400 / 2.0). The mint module mints exactly
// annual_provisions / blocks_per_year per block, regardless of wall-clock time,
// so with the real mainnet block time (~1.553s) the chain minted materially
// more tokens per year than the nominal inflation rate implied
// (actual emission ≈ nominal × 2.0 / block_time).
//
// TargetBlocksPerYear ≈ 365 * 86400 / 1.553, which re-aligns per-block emission
// with the nominal inflation rate. Only blocks_per_year is touched here; the
// inflation curve (inflation_max, inflation_min, goal_bonded,
// inflation_rate_change) is intentionally left unchanged.

import (
	"fmt"

	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TargetBlocksPerYear is the corrected x/mint blocks_per_year value for mainnet.
const TargetBlocksPerYear uint64 = 20_300_000

// migrateMintBlocksPerYear sets x/mint Params.BlocksPerYear to
// TargetBlocksPerYear, leaving every other mint parameter untouched. It is a
// no-op if the value is already at the target.
func migrateMintBlocksPerYear(ctx sdk.Context, ak *upgrades.AppKeepers) error {
	if ak.MintKeeper == nil {
		return fmt.Errorf("mint keeper not wired into AppKeepers")
	}

	params, err := ak.MintKeeper.Params.Get(ctx)
	if err != nil {
		return fmt.Errorf("get mint params: %w", err)
	}

	old := params.BlocksPerYear
	if old == TargetBlocksPerYear {
		ctx.Logger().Info("x/mint blocks_per_year already at target; skipping",
			"blocks_per_year", old)
		return nil
	}

	params.BlocksPerYear = TargetBlocksPerYear
	if err := params.Validate(); err != nil {
		return fmt.Errorf("validate mint params after blocks_per_year change: %w", err)
	}
	if err := ak.MintKeeper.Params.Set(ctx, params); err != nil {
		return fmt.Errorf("set mint params: %w", err)
	}

	ctx.Logger().Info("Corrected x/mint blocks_per_year",
		"old", old, "new", TargetBlocksPerYear)
	return nil
}
