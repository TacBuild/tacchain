package app

import (
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	v166 "github.com/TacBuild/tacchain/app/upgrades/v1.6.6"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
)

func TestV166Upgrade(t *testing.T) {
	const evidenceAddress = "0x0000000000000000000000000000000000000807"

	for _, tc := range []struct {
		name        string
		addEvidence bool
	}{
		{name: "remove legacy evidence address", addEvidence: true},
		{name: "evidence address already absent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := NewTacChainAppWithCustomOptions(t, false, SetupOptions{
				Logger:  log.NewTestLogger(t),
				DB:      dbm.NewMemDB(),
				AppOpts: simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
			})
			ctx := app.NewContext(false).WithBlockHeight(10).WithHeaderInfo(header.Info{Height: 10})
			require.True(t, app.UpgradeKeeper.HasHandler(v166.UpgradeName))

			params := app.EVMKeeper.GetParams(ctx)
			params.EVMChannels = []string{"channel-7"}
			expected := params
			expected.ActiveStaticPrecompiles = append([]string(nil), params.ActiveStaticPrecompiles...)
			if tc.addEvidence {
				params.ActiveStaticPrecompiles = append(params.ActiveStaticPrecompiles, evidenceAddress)
			}
			require.NoError(t, app.EVMKeeper.SetParams(ctx, params))

			versions, err := app.UpgradeKeeper.GetModuleVersionMap(ctx)
			require.NoError(t, err)

			plan := upgradetypes.Plan{Name: v166.UpgradeName, Height: ctx.BlockHeight()}
			require.NoError(t, app.UpgradeKeeper.ApplyUpgrade(ctx, plan))
			require.Equal(t, expected, app.EVMKeeper.GetParams(ctx))
			require.NotContains(t, app.EVMKeeper.GetParams(ctx).ActiveStaticPrecompiles, evidenceAddress)

			updatedVersions, err := app.UpgradeKeeper.GetModuleVersionMap(ctx)
			require.NoError(t, err)
			require.Equal(t, versions, updatedVersions)
			doneHeight, err := app.UpgradeKeeper.GetDoneHeight(ctx, v166.UpgradeName)
			require.NoError(t, err)
			require.Equal(t, ctx.BlockHeight(), doneHeight)
		})
	}
}
