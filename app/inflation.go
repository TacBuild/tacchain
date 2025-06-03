package app

import (
	"context"

	"cosmossdk.io/math"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

// Parabolic formula
// inflation (X) = Max inflation * (1 - ((X - Xtarget)/Xtarget)^2), where
// X - actual staking rate
// Xtarget - Ideal staking rate (e.g. 70%)
// Max inflation = 7%
//
// this one gives a U-shape curve (an inversed parabola) with the maximum at Xtarget (70%)
//
// Example: given goalBonded 70% and maxInflation 7%:
// - if currently 70% of tokens are staked the inflation should be 7%
// - the further staked tokens go away from goalBonded in both directions, the less inflation is
// - if 0% tokens staked then 0% inflation and so on
// - if 100% of tokens are staked inflation comes around 5%
// - if staked tokens are 40% inflation is again around 5% cuz both are  30% away from the goalBonded
func TacParabolicInflationFormula(_ context.Context, _ minttypes.Minter, params minttypes.Params, bondedRatio math.LegacyDec) math.LegacyDec {
	// Calculate (x - x_target) / x_target
	delta := bondedRatio.Sub(params.GoalBonded).Quo(params.GoalBonded)

	// Square the result
	deltaSquared := delta.Mul(delta)

	// 1 - delta^2
	adjustment := math.LegacyOneDec().Sub(deltaSquared)

	// Final inflation: MaxInflation * adjustment
	inflation := params.InflationMax.Mul(adjustment)

	return inflation
}

// Linear formula
// min_inflation = 2%, max_infalion = 7%, goal_bonded = 70%
// if x <= goal_bonded:
//     min_inflation + (max_inflation - min_inflation) * (x / goal_bonded)
// else:
//     max_inflation - (max_inflation - min_inflation) * ((x - goal_bonded) / (100% - goal_bonded))

// Example:
// 1. goal_bonded -> max inflation = 7%
// 2. left or right from the goal_bonded inflation is linearly decreasing to minimum = 2% (in both cases)
// 3. E.g.
// Staking rate = 35% infl = 4.5%
// Staking rate  = 100% infl = 2%
func TacLinearInflationFormula(_ context.Context, _ minttypes.Minter, params minttypes.Params, bondedRatio math.LegacyDec) math.LegacyDec {
	// If bondedRatio <= goalBonded, use the linear increase equation
	if bondedRatio.LTE(params.GoalBonded) {
		// Calculate ratio: bondedRatio / goalBonded
		ratio := bondedRatio.Quo(params.GoalBonded)

		// Calculate inflation: min + (max - min) * ratio
		inflation := params.InflationMin.Add(params.InflationMax.Sub(params.InflationMin).Mul(ratio))
		return inflation
	}

	// Else if bondedRatio > goalBonded: use the linear decrease equation
	// Calculate ratio: (bondedRatio - goalBonded) / (1 - goalBonded)
	ratio := bondedRatio.Sub(params.GoalBonded).Quo(math.LegacyOneDec().Sub(params.GoalBonded))

	// Calculate inflation: max - (max - min) * ratio
	inflation := params.InflationMax.Sub(params.InflationMax.Sub(params.InflationMin).Mul(ratio))
	return inflation
}

func TacZeroInflation(_ context.Context, _ minttypes.Minter, params minttypes.Params, bondedRatio math.LegacyDec) math.LegacyDec {
	return math.LegacyZeroDec()
}
