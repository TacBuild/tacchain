package v160

import (
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func init() {
	sdk.GetConfig().SetBech32PrefixForAccount("tac", "tacpub")
}

func TestParseRescueEntries_OK(t *testing.T) {
	info := `{
		"binaries": {"linux/amd64": "https://example.com/bin"},
		"vesting_migration": [
			{"old": "tac12t0efd0ylr4mlz4n0rm367qpt09g6yxq0pqnkt", "new": "tac1gepr027cw2l606z8grrsagzznw9esfyvz7mrxu"},
			{"old": "tac1uutlmwr3xcplm468t4k52clxjvd7g9vjmy0d84", "new": "tac10g3lwvw32tj6m8mfd7ry5u2cmtt76eezp3alrp"}
		]
	}`
	got, err := parseRescueEntries(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rescues, got %d", len(got))
	}
	if got[0].Old != "tac12t0efd0ylr4mlz4n0rm367qpt09g6yxq0pqnkt" ||
		got[0].New != "tac1gepr027cw2l606z8grrsagzznw9esfyvz7mrxu" {
		t.Fatalf("rescue[0] mismatch: %+v", got[0])
	}
}

func TestParseRescueEntries_Errors(t *testing.T) {
	cases := []struct {
		name    string
		info    string
		wantErr string
	}{
		{name: "empty info", info: "", wantErr: "plan.info is empty"},
		{name: "invalid json", info: `{not json`, wantErr: "not valid JSON"},
		{name: "missing vesting_migration", info: `{"binaries":{}}`, wantErr: "missing or empty 'vesting_migration'"},
		{name: "empty rescues", info: `{"vesting_migration":[]}`, wantErr: "missing or empty"},
		{
			name:    "invalid old bech32",
			info:    `{"vesting_migration":[{"old":"not-bech32","new":"tac1gepr027cw2l606z8grrsagzznw9esfyvz7mrxu"}]}`,
			wantErr: "rescues[0].old",
		},
		{
			name:    "invalid new bech32",
			info:    `{"vesting_migration":[{"old":"tac12t0efd0ylr4mlz4n0rm367qpt09g6yxq0pqnkt","new":"x"}]}`,
			wantErr: "rescues[0].new",
		},
		{
			name:    "old==new",
			info:    `{"vesting_migration":[{"old":"tac12t0efd0ylr4mlz4n0rm367qpt09g6yxq0pqnkt","new":"tac12t0efd0ylr4mlz4n0rm367qpt09g6yxq0pqnkt"}]}`,
			wantErr: "old and new are the same",
		},
		{
			name: "duplicate old",
			info: `{"vesting_migration":[
				{"old":"tac12t0efd0ylr4mlz4n0rm367qpt09g6yxq0pqnkt","new":"tac1gepr027cw2l606z8grrsagzznw9esfyvz7mrxu"},
				{"old":"tac12t0efd0ylr4mlz4n0rm367qpt09g6yxq0pqnkt","new":"tac1uutlmwr3xcplm468t4k52clxjvd7g9vjmy0d84"}
			]}`,
			wantErr: "duplicate old",
		},
		{
			name: "duplicate new",
			info: `{"vesting_migration":[
				{"old":"tac12t0efd0ylr4mlz4n0rm367qpt09g6yxq0pqnkt","new":"tac1gepr027cw2l606z8grrsagzznw9esfyvz7mrxu"},
				{"old":"tac1uutlmwr3xcplm468t4k52clxjvd7g9vjmy0d84","new":"tac1gepr027cw2l606z8grrsagzznw9esfyvz7mrxu"}
			]}`,
			wantErr: "duplicate new",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseRescueEntries(tc.info)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}
