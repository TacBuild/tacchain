package v160

import (
	"encoding/json"
	"fmt"

	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingexported "github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

type RescueEntry struct {
	Old string `json:"old"`
	New string `json:"new"`
}

type planInfo struct {
	VestingMigration []RescueEntry `json:"vesting_migration,omitempty"`
}

func parseRescueEntries(info string) ([]RescueEntry, error) {
	if info == "" {
		return nil, fmt.Errorf("plan.info is empty: vesting_migration is required for v1.6.0")
	}

	var p planInfo
	if err := json.Unmarshal([]byte(info), &p); err != nil {
		return nil, fmt.Errorf("plan.info is not valid JSON: %w", err)
	}
	if len(p.VestingMigration) == 0 {
		return nil, fmt.Errorf("plan.info missing or empty 'vesting_migration' array")
	}

	seenOld := make(map[string]struct{}, len(p.VestingMigration))
	seenNew := make(map[string]struct{}, len(p.VestingMigration))
	for i, e := range p.VestingMigration {
		oldAddr, err := sdk.AccAddressFromBech32(e.Old)
		if err != nil {
			return nil, fmt.Errorf("rescues[%d].old (%q) is not a valid bech32 address: %w", i, e.Old, err)
		}
		newAddr, err := sdk.AccAddressFromBech32(e.New)
		if err != nil {
			return nil, fmt.Errorf("rescues[%d].new (%q) is not a valid bech32 address: %w", i, e.New, err)
		}
		if oldAddr.Equals(newAddr) {
			return nil, fmt.Errorf("rescues[%d]: old and new are the same address (%s)", i, e.Old)
		}

		oldKey := string(oldAddr)
		newKey := string(newAddr)
		if _, dup := seenOld[oldKey]; dup {
			return nil, fmt.Errorf("rescues: duplicate old address %s", e.Old)
		}
		if _, dup := seenNew[newKey]; dup {
			return nil, fmt.Errorf("rescues: duplicate new address %s", e.New)
		}
		if _, exists := seenNew[oldKey]; exists {
			return nil, fmt.Errorf("rescues[%d].old %s is also used as a new address", i, e.Old)
		}
		if _, exists := seenOld[newKey]; exists {
			return nil, fmt.Errorf("rescues[%d].new %s is also used as an old address", i, e.New)
		}

		seenOld[oldKey] = struct{}{}
		seenNew[newKey] = struct{}{}
	}

	return p.VestingMigration, nil
}

func preflightRescueEntries(ctx sdk.Context, ak *upgrades.AppKeepers, rescues []RescueEntry) error {
	for i, r := range rescues {
		oldAddr, err := sdk.AccAddressFromBech32(r.Old)
		if err != nil {
			return fmt.Errorf("rescues[%d].old (%q) is not a valid bech32 address: %w", i, r.Old, err)
		}
		newAddr, err := sdk.AccAddressFromBech32(r.New)
		if err != nil {
			return fmt.Errorf("rescues[%d].new (%q) is not a valid bech32 address: %w", i, r.New, err)
		}

		oldAcc := ak.AccountKeeper.GetAccount(ctx, oldAddr)
		if oldAcc == nil {
			return fmt.Errorf("rescues[%d].old %s: account not found", i, r.Old)
		}
		if _, ok := oldAcc.(*vestingtypes.PeriodicVestingAccount); !ok {
			return fmt.Errorf("rescues[%d].old %s: expected PeriodicVestingAccount, got %T", i, r.Old, oldAcc)
		}

		newAcc := ak.AccountKeeper.GetAccount(ctx, newAddr)
		if !isCleanupSafeRescueDestinationAccount(newAcc) {
			return fmt.Errorf("rescues[%d].new %s: expected empty account or unused BaseAccount/vesting account; got %T", i, r.New, newAcc)
		}
	}

	return nil
}

func rescueDestinationBaseAccount(ctx sdk.Context, ak *upgrades.AppKeepers, addr sdk.AccAddress) (*authtypes.BaseAccount, error) {
	acc := ak.AccountKeeper.GetAccount(ctx, addr)
	if acc == nil {
		baseAcc := authtypes.NewBaseAccountWithAddress(addr)
		return ak.AccountKeeper.NewAccount(ctx, baseAcc).(*authtypes.BaseAccount), nil
	}

	if !isCleanupSafeRescueDestinationAccount(acc) {
		return nil, fmt.Errorf("expected empty account or unused BaseAccount/vesting account; got %T", acc)
	}

	if baseAcc, ok := acc.(*authtypes.BaseAccount); ok {
		return baseAcc, nil
	}

	baseAcc := authtypes.NewBaseAccountWithAddress(addr)
	baseAcc.AccountNumber = acc.GetAccountNumber()
	baseAcc.Sequence = acc.GetSequence()

	ctx.Logger().Info("Rescue destination account cleanup",
		"address", addr.String(),
		"old_type", fmt.Sprintf("%T", acc),
		"account_number", baseAcc.AccountNumber,
	)

	return baseAcc, nil
}

func isCleanupSafeRescueDestinationAccount(acc sdk.AccountI) bool {
	if acc == nil {
		return true
	}
	if _, ok := acc.(*authtypes.BaseAccount); ok {
		return isUnusedRescueDestinationAccount(acc)
	}
	if _, ok := acc.(vestingexported.VestingAccount); !ok {
		return false
	}

	return isUnusedRescueDestinationAccount(acc)
}

func isUnusedRescueDestinationAccount(acc sdk.AccountI) bool {
	return acc.GetPubKey() == nil && acc.GetSequence() == 0
}
