package liquidstake

import (
	"time"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Asphere-xyz/tacchain/x/liquidstake/keeper"
	"github.com/Asphere-xyz/tacchain/x/liquidstake/types"
)

func BeginBlock(ctx sdk.Context, k keeper.Keeper) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyBeginBlocker)

	if !k.GetParams(ctx).ModulePaused {
		// return value of UpdateLiquidValidatorSet is useful only in testing
		_ = k.UpdateLiquidValidatorSet(ctx, false)
	}
}
