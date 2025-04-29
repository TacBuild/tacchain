// SPDX-License-Identifier: BUSL-1.1-or-later
// SPDX-FileCopyrightText: 2025 Web3 Technologies Inc. <https://asphere.xyz/>
// Copyright (c) 2025 Web3 Technologies Inc. All rights reserved.
// Use of this software is governed by the Business Source License included in the LICENSE file <https://github.com/Asphere-xyz/tacchain/blob/main/LICENSE>.
package app

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/math"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/stretchr/testify/require"
)

func TestTacParabolicInflationFormula(t *testing.T) {
	inflationMax, _ := math.LegacyNewDecFromStr("0.07")
	goalBonded, _ := math.LegacyNewDecFromStr("0.7")
	params := minttypes.Params{
		InflationMax: inflationMax,
		GoalBonded:   goalBonded,
	}

	testCases := []struct {
		name              string
		bondedRatio       string
		expectedInflation string
	}{
		{"Ideal staking rate (target)", "0.70", "0.070"},
		{"No staking", "0.0", "0.000"},
		{"Half of target", "0.35", "0.052"},
		{"Below target but close", "0.50", "0.064"},
		{"Above target but close", "0.90", "0.064"},
		{"Further above target", "1.0", "0.057"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bonded, _ := math.LegacyNewDecFromStr(tc.bondedRatio)

			inflation := TacParabolicInflationFormula(context.Background(), minttypes.Minter{}, params, bonded)

			// truncate to 3 decimal places
			truncatedInflation := fmt.Sprintf("%.3f", inflation.MustFloat64())

			require.Equal(t, tc.expectedInflation, truncatedInflation)
		})
	}
}

func TestTacLinearInflationFormula(t *testing.T) {
	inflationMin, _ := math.LegacyNewDecFromStr("0.02")
	inflationMax, _ := math.LegacyNewDecFromStr("0.07")
	goalBonded, _ := math.LegacyNewDecFromStr("0.7")
	params := minttypes.Params{
		InflationMin: inflationMin,
		InflationMax: inflationMax,
		GoalBonded:   goalBonded,
	}

	testCases := []struct {
		name              string
		bondedRatio       string
		expectedInflation string
	}{
		{"Ideal staking rate", "0.70", "0.070"},
		{"No staking", "0.0", "0.020"},
		{"Half of target", "0.35", "0.045"},
		{"Slightly above target", "0.80", "0.053"},
		{"More above target", "0.90", "0.037"},
		{"Full staking", "1.0", "0.020"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bonded, _ := math.LegacyNewDecFromStr(tc.bondedRatio)

			inflation := TacLinearInflationFormula(context.Background(), minttypes.Minter{}, params, bonded)

			// truncate to 3 decimal places
			truncatedInflation := fmt.Sprintf("%.3f", inflation.MustFloat64())

			require.Equal(t, tc.expectedInflation, truncatedInflation)
		})
	}
}
