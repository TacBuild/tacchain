package v166

import (
	"errors"
	"fmt"

	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

var vestingUnlockAddresses = [...]string{
	"tac14gurzf75agujcznmv7k5xnz5z9jgcctza5gynh",
	"tac1td8q7adv0x6ltxs8630tves5t9y49fzfs8cj9d",
	"tac18f83zsqe8up5vge8l8gpr3tnnp2sfvheqxn83f",
	accountMigrationDestination,
}

func runVestingUnlock(ctx sdk.Context, ak *upgrades.AppKeepers) {
	for _, address := range vestingUnlockAddresses {
		if err := unlockVestingAccount(ctx, ak, address); err != nil {
			ctx.Logger().Error("Vesting account unlock failed", "address", address, "error", err)
		}
	}
}

func unlockVestingAccount(ctx sdk.Context, ak *upgrades.AppKeepers, address string) error {
	addr, err := ak.AccountKeeper.AddressCodec().StringToBytes(address)
	if err != nil {
		return err
	}
	switch account := ak.AccountKeeper.GetAccount(ctx, addr).(type) {
	case *authtypes.BaseAccount:
		return nil
	case *vestingtypes.PeriodicVestingAccount:
		if account.BaseVestingAccount == nil || account.BaseAccount == nil {
			return errors.New("embedded base account not found")
		}
		ak.AccountKeeper.SetAccount(ctx, account.BaseAccount)
		return nil
	default:
		return fmt.Errorf("unexpected account type %T", account)
	}
}
