package v166

import (
	"fmt"

	appconfig "github.com/TacBuild/tacchain/app/config"
	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
)

const (
	accountMigrationChainID     = "tacchain_239-1"
	accountMigrationSource      = "tac1zw4llpxyus4j5yxrejgx6w9uth8r0xdqd6503j"
	accountMigrationDestination = "tac1glrzh24ka4ay6d5fgy7sz4car5d9qq3qyd4g2n"
)

func runAccountMigration(ctx sdk.Context, ak *upgrades.AppKeepers) {
	logger := ctx.Logger()
	codec := ak.AccountKeeper.AddressCodec()
	sourceBytes, err := codec.StringToBytes(accountMigrationSource)
	if err != nil {
		logger.Error("Invalid source address", "error", err)
		return
	}
	destinationBytes, err := codec.StringToBytes(accountMigrationDestination)
	if err != nil {
		logger.Error("Invalid destination address", "error", err)
		return
	}
	source := sdk.AccAddress(sourceBytes)
	destination := sdk.AccAddress(destinationBytes)

	if err := migrateAccountBalance(ctx, ak, source, destination); err != nil {
		logger.Error("Account balance migration failed", "error", err)
		return
	}
	logger.Info("Account balance migration complete")

	if err := migrateAccountStaking(ctx, ak, source, destination); err != nil {
		logger.Error("Account staking migration failed", "error", err)
		return
	}
	logger.Info("Account staking migration complete")
}

func migrateAccountBalance(ctx sdk.Context, ak *upgrades.AppKeepers, source, destination sdk.AccAddress) error {
	cacheCtx, write := ctx.CacheContext()
	if ak.AccountKeeper.GetAccount(cacheCtx, destination) == nil {
		return fmt.Errorf("destination account not found")
	}

	switch account := ak.AccountKeeper.GetAccount(cacheCtx, source).(type) {
	case *vestingtypes.PeriodicVestingAccount:
		if account.BaseVestingAccount == nil || account.BaseAccount == nil {
			return fmt.Errorf("source base account not found")
		}
		for _, coin := range account.OriginalVesting {
			if coin.Denom != appconfig.BaseDenom {
				return fmt.Errorf("unexpected vesting denomination %s", coin.Denom)
			}
		}
		ak.AccountKeeper.SetAccount(cacheCtx, account.BaseAccount)
	case *authtypes.BaseAccount:
	default:
		return fmt.Errorf("unexpected source account type %T", account)
	}

	if err := transferAccountBalance(cacheCtx, ak.BankKeeper, source, destination); err != nil {
		return err
	}
	write()
	return nil
}

func transferAccountBalance(ctx sdk.Context, bank bankkeeper.Keeper, source, destination sdk.AccAddress) error {
	balance := bank.GetBalance(ctx, source, appconfig.BaseDenom)
	if balance.IsZero() {
		return nil
	}
	before := bank.SpendableCoins(ctx, destination).AmountOf(appconfig.BaseDenom)
	if err := bank.SendCoins(ctx, source, destination, sdk.NewCoins(balance)); err != nil {
		return fmt.Errorf("transfer account balance: %w", err)
	}
	after := bank.SpendableCoins(ctx, destination).AmountOf(appconfig.BaseDenom)
	if !after.Equal(before.Add(balance.Amount)) || !bank.GetBalance(ctx, source, appconfig.BaseDenom).IsZero() {
		return fmt.Errorf("unexpected balance after transfer")
	}
	return nil
}
