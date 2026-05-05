package v160

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type RescueEntry struct {
	Old string `json:"old"`
	New string `json:"new"`
}

type VestingMigration struct {
	Rescues []RescueEntry `json:"rescues"`
}

type planInfo struct {
	VestingMigration *VestingMigration `json:"vesting_migration,omitempty"`
}

func parseRescueEntries(info string) ([]RescueEntry, error) {
	if info == "" {
		return nil, fmt.Errorf("plan.info is empty: vesting_migration.rescues is required for v1.6.0")
	}

	var p planInfo
	if err := json.Unmarshal([]byte(info), &p); err != nil {
		return nil, fmt.Errorf("plan.info is not valid JSON: %w", err)
	}
	if p.VestingMigration == nil {
		return nil, fmt.Errorf("plan.info missing 'vesting_migration' object")
	}
	if len(p.VestingMigration.Rescues) == 0 {
		return nil, fmt.Errorf("plan.info 'vesting_migration.rescues' is empty")
	}

	seenOld := make(map[string]struct{}, len(p.VestingMigration.Rescues))
	seenNew := make(map[string]struct{}, len(p.VestingMigration.Rescues))
	for i, e := range p.VestingMigration.Rescues {
		if _, err := sdk.AccAddressFromBech32(e.Old); err != nil {
			return nil, fmt.Errorf("rescues[%d].old (%q) is not a valid bech32 address: %w", i, e.Old, err)
		}
		if _, err := sdk.AccAddressFromBech32(e.New); err != nil {
			return nil, fmt.Errorf("rescues[%d].new (%q) is not a valid bech32 address: %w", i, e.New, err)
		}
		if e.Old == e.New {
			return nil, fmt.Errorf("rescues[%d]: old and new are the same address (%s)", i, e.Old)
		}
		if _, dup := seenOld[e.Old]; dup {
			return nil, fmt.Errorf("rescues: duplicate old address %s", e.Old)
		}
		if _, dup := seenNew[e.New]; dup {
			return nil, fmt.Errorf("rescues: duplicate new address %s", e.New)
		}
		seenOld[e.Old] = struct{}{}
		seenNew[e.New] = struct{}{}
	}

	return p.VestingMigration.Rescues, nil
}
