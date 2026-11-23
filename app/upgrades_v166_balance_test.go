package app

import (
	"context"
	"errors"
	"testing"
	"time"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	appconfig "github.com/TacBuild/tacchain/app/config"
	v166 "github.com/TacBuild/tacchain/app/upgrades/v1.6.6"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

const (
	v166TestChainID     = "tacchain_239-1"
	v166TestSource      = "tac1zw4llpxyus4j5yxrejgx6w9uth8r0xdqd6503j"
	v166TestDestination = "tac1glrzh24ka4ay6d5fgy7sz4car5d9qq3qyd4g2n"
)

func TestV166AccountBalanceMigration(t *testing.T) {
	for _, tc := range []struct {
		name               string
		chainID            string
		plainSource        bool
		rejectSend         bool
		destinationMissing bool
		destinationDeficit bool
		wantTransfer       bool
	}{
		{name: "periodic source", chainID: v166TestChainID, wantTransfer: true},
		{name: "base source", chainID: v166TestChainID, plainSource: true, wantTransfer: true},
		{name: "transfer failure", chainID: v166TestChainID, rejectSend: true},
		{name: "missing destination", chainID: v166TestChainID, destinationMissing: true},
		{name: "destination vesting deficit", chainID: v166TestChainID, destinationDeficit: true, wantTransfer: true},
		{name: "different chain", chainID: appconfig.DefaultChainID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := NewTacChainAppWithCustomOptions(t, false, SetupOptions{
				Logger:  log.NewTestLogger(t),
				DB:      dbm.NewMemDB(),
				AppOpts: simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
			})
			blockTime := time.Unix(100, 0)
			ctx := app.NewContext(false).WithChainID(tc.chainID).WithBlockHeight(10).WithBlockTime(blockTime).
				WithHeaderInfo(header.Info{ChainID: tc.chainID, Height: 10, Time: blockTime})
			source, err := sdk.AccAddressFromBech32(v166TestSource)
			require.NoError(t, err)
			destination, err := sdk.AccAddressFromBech32(v166TestDestination)
			require.NoError(t, err)

			amount := sdk.NewInt64Coin(appconfig.BaseDenom, 100)
			sourceFunds := sdk.NewCoins(amount, sdk.NewInt64Coin("uother", 7))
			destinationFunds := sdk.NewCoins()
			sourceBase := authtypes.NewBaseAccount(source, nil, 101, 7)
			var sourceAccount sdk.AccountI = sourceBase
			if !tc.plainSource {
				vesting, err := vestingtypes.NewPeriodicVestingAccount(sourceBase, sdk.NewCoins(amount), 50,
					vestingtypes.Periods{{Length: 100, Amount: sdk.NewCoins(amount)}})
				require.NoError(t, err)
				sourceAccount = vesting
			}
			app.AccountKeeper.SetAccount(ctx, sourceAccount)
			var destinationAccount sdk.AccountI
			if !tc.destinationMissing {
				locked := sdk.NewCoins(sdk.NewInt64Coin(appconfig.BaseDenom, 50))
				vesting, err := vestingtypes.NewPeriodicVestingAccount(
					authtypes.NewBaseAccount(destination, nil, 102, 1), locked, 50,
					vestingtypes.Periods{{Length: 100, Amount: locked}},
				)
				require.NoError(t, err)
				destinationAccount = vesting
				app.AccountKeeper.SetAccount(ctx, destinationAccount)
				balance := int64(90)
				if tc.destinationDeficit {
					balance = 30
				}
				destinationFunds = sdk.NewCoins(sdk.NewInt64Coin(appconfig.BaseDenom, balance))
			}
			require.NoError(t, app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sourceFunds.Add(destinationFunds...)))
			require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, source, sourceFunds))
			if !tc.destinationMissing {
				require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, destination, destinationFunds))
			}
			beforeSpendable := app.BankKeeper.SpendableCoins(ctx, destination).AmountOf(appconfig.BaseDenom)
			beforeSupply := app.BankKeeper.GetSupply(ctx, appconfig.BaseDenom)

			sendRejected := false
			if tc.rejectSend {
				app.BankKeeper.AppendSendRestriction(func(_ context.Context, from, to sdk.AccAddress, _ sdk.Coins) (sdk.AccAddress, error) {
					if from.Equals(source) && to.Equals(destination) {
						sendRejected = true
						return nil, errors.New("transfer rejected")
					}
					return to, nil
				})
			}

			plan := upgradetypes.Plan{Name: v166.UpgradeName, Height: ctx.BlockHeight()}
			require.NoError(t, app.UpgradeKeeper.ApplyUpgrade(ctx, plan))
			doneHeight, err := app.UpgradeKeeper.GetDoneHeight(ctx, v166.UpgradeName)
			require.NoError(t, err)
			require.Equal(t, ctx.BlockHeight(), doneHeight)
			require.Equal(t, tc.rejectSend, sendRejected)

			wantSourceAccount := sourceAccount
			wantSourceFunds := sourceFunds
			wantDestinationAccount := destinationAccount
			wantDestinationFunds := destinationFunds
			wantSpendable := beforeSpendable
			if tc.chainID == v166TestChainID && destinationAccount != nil {
				wantDestinationAccount = destinationAccount.(*vestingtypes.PeriodicVestingAccount).BaseAccount
				wantSpendable = destinationFunds.AmountOf(appconfig.BaseDenom)
			}
			if tc.wantTransfer {
				wantSourceAccount = sourceBase
				wantSourceFunds = sourceFunds.Sub(amount)
				wantDestinationFunds = destinationFunds.Add(amount)
				wantSpendable = wantSpendable.Add(amount.Amount)
			}

			require.Equal(t, wantSourceAccount, app.AccountKeeper.GetAccount(ctx, source))
			require.Equal(t, wantDestinationAccount, app.AccountKeeper.GetAccount(ctx, destination))
			require.Equal(t, wantSourceFunds, app.BankKeeper.GetAllBalances(ctx, source))
			require.Equal(t, wantDestinationFunds, app.BankKeeper.GetAllBalances(ctx, destination))
			require.Equal(t, wantSpendable, app.BankKeeper.SpendableCoins(ctx, destination).AmountOf(appconfig.BaseDenom))
			require.Equal(t, beforeSupply, app.BankKeeper.GetSupply(ctx, appconfig.BaseDenom))
		})
	}
}
