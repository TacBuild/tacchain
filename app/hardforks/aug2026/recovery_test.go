package recovery

import (
	"context"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const testDenom = "utac"

// Local-test addresses (fake; not the real audited accounts).
const (
	tAttacker = "tac1p0wnk86ere6zxajp4ykflfr5ap3lmla6t0k22n"
	tEscrow   = "tac1e3qva29tw932mh3mq382ufk7rlpyvvt6hn5nu2"
	tAdmin    = "tac1k4lcd0amymuqlh2lay66k87ezygj8rmzenm0m2"
	tTre1     = "tac15xedv3w78gfx5rcpk7knxlmfg8lff7wemsgp2v"
	tTre2     = "tac1eggmmczew7ekxyt8q2yx9032zuuhdjs3yd8s2h"
)

func init() {
	// The migration parses/prints "tac"-prefixed addresses; the app sets this at
	// startup, so the unit test must set it too (SDK default is "cosmos").
	sdk.GetConfig().SetBech32PrefixForAccount("tac", "tacpub")
}

func bi(s string) math.Int { v, _ := math.NewIntFromString(s); return v }

// ---- minimal in-memory mocks ----

type mockAccount struct{}

func (mockAccount) GetModuleAddress(name string) sdk.AccAddress {
	return authtypes.NewModuleAddress(name)
}

type mockBank struct {
	bal    map[string]math.Int
	supply math.Int
}

func newMockBank() *mockBank { return &mockBank{bal: map[string]math.Int{}, supply: math.ZeroInt()} }

func (m *mockBank) get(k string) math.Int {
	if v, ok := m.bal[k]; ok {
		return v
	}
	return math.ZeroInt()
}
func (m *mockBank) GetBalance(_ context.Context, addr sdk.AccAddress, _ string) sdk.Coin {
	return sdk.NewCoin(testDenom, m.get(addr.String()))
}
func (m *mockBank) GetSupply(_ context.Context, _ string) sdk.Coin {
	return sdk.NewCoin(testDenom, m.supply)
}
func (m *mockBank) MintCoins(_ context.Context, mod string, amt sdk.Coins) error {
	k := authtypes.NewModuleAddress(mod).String()
	a := amt.AmountOf(testDenom)
	m.bal[k] = m.get(k).Add(a)
	m.supply = m.supply.Add(a)
	return nil
}
func (m *mockBank) BurnCoins(_ context.Context, mod string, amt sdk.Coins) error {
	k := authtypes.NewModuleAddress(mod).String()
	a := amt.AmountOf(testDenom)
	m.bal[k] = m.get(k).Sub(a)
	m.supply = m.supply.Sub(a)
	return nil
}
func (m *mockBank) SendCoinsFromModuleToModule(_ context.Context, from, to string, amt sdk.Coins) error {
	fk, tk := authtypes.NewModuleAddress(from).String(), authtypes.NewModuleAddress(to).String()
	a := amt.AmountOf(testDenom)
	m.bal[fk] = m.get(fk).Sub(a)
	m.bal[tk] = m.get(tk).Add(a)
	return nil
}
func (m *mockBank) SendCoinsFromAccountToModule(_ context.Context, from sdk.AccAddress, to string, amt sdk.Coins) error {
	fk, tk := from.String(), authtypes.NewModuleAddress(to).String()
	a := amt.AmountOf(testDenom)
	m.bal[fk] = m.get(fk).Sub(a)
	m.bal[tk] = m.get(tk).Add(a)
	return nil
}
func (m *mockBank) SendCoins(_ context.Context, from, to sdk.AccAddress, amt sdk.Coins) error {
	a := amt.AmountOf(testDenom)
	m.bal[from.String()] = m.get(from.String()).Sub(a)
	m.bal[to.String()] = m.get(to.String()).Add(a)
	return nil
}

type mockStaking struct{ bonded math.Int }

func (mockStaking) BondDenom(context.Context) (string, error) { return testDenom, nil }
func (s mockStaking) GetAllValidators(context.Context) ([]stakingtypes.Validator, error) {
	return []stakingtypes.Validator{{Status: stakingtypes.Bonded, Tokens: s.bonded}}, nil
}

func testCtx() sdk.Context {
	return sdk.Context{}.WithLogger(log.NewNopLogger()).WithChainID("tacchain_2391337-1").WithBlockHeight(1164)
}

// localParams — self-contained local-exploit values for the guard tests, independent
// of ParamsByChainID (mainnet entry ships; the local entry is not present there).
// Shape: burn attacker + escrow-share + two treasury accounts; sweep the escrow
// remainder to a standalone admin.
func localParams() Params {
	return Params{
		Height:          1164,
		PoolBefore:      "0",
		PoolTargetAfter: "10000000000000000000000001",
		PoolRestore:     "10000000000000000000000001",
		ToBurn: map[string]string{
			tAttacker: "999999699999999999999999",
			tEscrow:   "2000000000000000000000000", // partial: the rest is transferred out
			tTre1:     "4000000000000000000000000",
			tTre2:     "3000000300000000000000002",
		},
		Transfer: map[string]Transfer{
			tEscrow: {To: tAdmin, Amount: "500000000000000000000000"},
		},
	}
}

// localParamsB — variant with a dead-address burn folded into ToBurn (tTre2 plays 0x00)
// AND a transfer recipient (tTre1) that is itself a ToBurn source (the mainnet shape).
func localParamsB() Params {
	return Params{
		Height:          1164,
		PoolBefore:      "0",
		PoolTargetAfter: "11000000000000000000000000",
		PoolRestore:     "11000000000000000000000000",
		ToBurn: map[string]string{
			tAttacker: "1000000000000000000000000",
			tEscrow:   "2000000000000000000000000", // partial: the rest is transferred out
			tTre2:     "3000000000000000000000000", // dead-address (0x00) role, now a plain burn
			tAdmin:    "1000000000000000000000000",
			tTre1:     "4000000000000000000000000", // also the transfer recipient (overlap)
		},
		Transfer: map[string]Transfer{
			tEscrow: {To: tTre1, Amount: "500000000000000000000000"},
		},
	}
}

func seed(bank *mockBank, p Params) {
	add := func(addr string, amt math.Int) {
		bank.bal[addr] = bank.get(addr).Add(amt)
		bank.supply = bank.supply.Add(amt)
	}
	add(authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(), bi(p.PoolBefore))
	for a, amt := range p.ToBurn {
		add(a, bi(amt))
	}
	for a, t := range p.Transfer {
		add(a, bi(t.Amount)) // stacked on top of the source's burn balance
	}
}

func keepers(bank *mockBank, bonded math.Int) Keepers {
	return Keepers{Account: mockAccount{}, Bank: bank, Staking: mockStaking{bonded: bonded}}
}

// ---- tests ----

func TestMigrate_HappyPath(t *testing.T) {
	p := localParams()
	bank := newMockBank()
	seed(bank, p)
	supplyBefore := bank.supply
	if err := Migrate(testCtx(), keepers(bank, bi(p.PoolTargetAfter)), p); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got := bank.get(authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String()); !got.Equal(bi(p.PoolTargetAfter)) {
		t.Fatalf("pool = %s, want %s (== sum bonded)", got, p.PoolTargetAfter)
	}
	// every burn source ends at 0 (seeded exactly its burn; the escrow's rest is transferred out)
	for a := range p.ToBurn {
		if got := bank.get(a); !got.IsZero() {
			t.Fatalf("burn source %s = %s, want 0", a, got)
		}
	}
	// the standalone recipient received the swept remainder
	for _, tr := range p.Transfer {
		if got := bank.get(tr.To); !got.Equal(bi(tr.Amount)) {
			t.Fatalf("recipient %s = %s, want %s", tr.To, got, tr.Amount)
		}
	}
	if !bank.supply.Equal(supplyBefore) {
		t.Fatalf("supply changed %s -> %s (must be neutral)", supplyBefore, bank.supply)
	}
}

func TestMigrate_VariantB_ZeroBurnAndRecipientOverlap(t *testing.T) {
	p := localParamsB()
	bank := newMockBank()
	seed(bank, p)
	// tTre1 is a burn source AND the transfer recipient: give it a residual above its
	// burn amount, so after the burn it keeps the residual and then receives the sweep.
	residual := bi("250000000000000000000000")
	bank.bal[tTre1] = bank.get(tTre1).Add(residual)
	bank.supply = bank.supply.Add(residual)
	supplyBefore := bank.supply

	if err := Migrate(testCtx(), keepers(bank, bi(p.PoolTargetAfter)), p); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	// the dead-address role (tTre2) is fully burned
	if got := bank.get(tTre2); !got.IsZero() {
		t.Fatalf("dead-address = %s, want 0 (burned)", got)
	}
	// escrow source fully drained
	if got := bank.get(tEscrow); !got.IsZero() {
		t.Fatalf("escrow = %s, want 0 (fully drained)", got)
	}
	// recipient: burned its share, kept the residual, then received the sweep
	if want := residual.Add(bi(p.Transfer[tEscrow].Amount)); !bank.get(tTre1).Equal(want) {
		t.Fatalf("recipient = %s, want %s (residual + sweep)", bank.get(tTre1), want)
	}
	if !bank.supply.Equal(supplyBefore) {
		t.Fatalf("supply changed %s -> %s (must be neutral)", supplyBefore, bank.supply)
	}
}

func TestMigrate_InsufficientBalance_Aborts(t *testing.T) {
	p := localParams()
	bank := newMockBank()
	seed(bank, p)
	bank.bal[tAttacker] = bi(p.ToBurn[tAttacker]).Sub(math.OneInt()) // source short by 1 utac
	if err := Migrate(testCtx(), keepers(bank, bi(p.PoolTargetAfter)), p); err == nil {
		t.Fatal("expected insufficient-balance abort, got nil")
	}
}

func TestMigrate_BondedSumMismatch_Aborts(t *testing.T) {
	p := localParams()
	bank := newMockBank()
	seed(bank, p)
	// live bonded set != hardcoded PoolTargetAfter
	if err := Migrate(testCtx(), keepers(bank, bi(p.PoolTargetAfter).Add(math.OneInt())), p); err == nil {
		t.Fatal("expected preflight abort, got nil")
	}
}

func TestMigrate_NotSupplyNeutral_Aborts(t *testing.T) {
	p := localParams()
	p.PoolRestore = bi(p.PoolRestore).Add(math.OneInt()).String() // mint 1 more than burned
	bank := newMockBank()
	seed(bank, p)
	if err := Migrate(testCtx(), keepers(bank, bi(p.PoolTargetAfter)), p); err == nil {
		t.Fatal("expected supply-neutral abort, got nil")
	}
}

func TestChainIDGate(t *testing.T) {
	// Mainnet entry ships; the local-test entries are not present.
	if _, ok := ParamsByChainID["tacchain_239-1"]; !ok {
		t.Fatal("mainnet chain-id must be present (migration eligible)")
	}
	// The gate is the map lookup: any chain-id not listed is skipped by the PreBlocker.
	for _, cid := range []string{"tacchain_2391337-1", "tacchain_2391-1", "some-other-chain"} {
		if _, ok := ParamsByChainID[cid]; ok {
			t.Fatalf("chain-id %q must be ABSENT in this build (gate must skip it)", cid)
		}
	}
}
