// Package recovery holds the Aug-2026 incident state-migration.
//
// The exploit drained the bonded_tokens_pool bank balance to ~0 while the bonded
// validators still hold their tokens -> the pool is insolvent (bank(pool) !=
// sum of bonded validator.tokens). This migration is SUPPLY-NEUTRAL: it mints the
// pool deficit and burns the SAME total across a set of accounts (ToBurn), so the
// total supply is unchanged. It then moves the remainder of any partially-drained
// account to its destination (Transfer); a transfer does not change supply.
//
// GATING: the migration runs ONLY on a chain-id present in ParamsByChainID, and
// ONLY at that entry's Height. On any other network the PreBlocker skips it
// (returns nil) instead of touching state or halting. All amounts are hardcoded
// absolute values, audited against the halt state; the guards assert the invariants
// before and after and abort (deterministic halt) on any mismatch.
package recovery

import (
	"context"
	"fmt"
	"slices"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// Transfer moves Amount (base units) from a source account to To. Supply-neutral.
type Transfer struct {
	To     string
	Amount string
}

// Params is the per-network recovery configuration. Absolute base-unit (utac)
// amounts, audited against that network's halt state. Runtime invariants:
//   - PoolRestore == sum(ToBurn amounts)                       (supply-neutral: mint == burn)
//   - PoolBefore + PoolRestore == PoolTargetAfter == sum of bonded validator.tokens
type Params struct {
	Height          int64               // block at which PreBlocker runs Migrate (halt height + 1)
	PoolBefore      string              // expected bonded_pool bank balance BEFORE (drained)
	PoolTargetAfter string              // expected == sum of bonded validator.tokens
	PoolRestore     string              // minted into bonded_pool (== deficit); must == sum(ToBurn)
	ToBurn          map[string]string   // address -> amount burned from it
	Transfer        map[string]Transfer // from-address -> {To, Amount}; moves the remainder
}

// ParamsByChainID: only the listed chains ever run the migration. The map lookup
// in the PreBlocker is the chain-id gate (absent key -> skip).
var ParamsByChainID = map[string]Params{
	// MAINNET recovery (Aug-2026 incident). ARMED at the recovery height 24671476
	"tacchain_239-1": {
		Height:          24671476, // recovery block = last committed 24671475 + 1
		PoolBefore:      "1",
		PoolTargetAfter: "2985651403404712731337326750",
		PoolRestore:     "2985651403404712731337326749",
		ToBurn: map[string]string{
			"tac1ajc2l9myf5kz33vrd8rxxqr6rdmuwlyyj69wks": "65100988589194488679326677",
			"tac1zgvugz06hckz00gdrft9mthdn0vlyuw76rfwvj": "1662322352703987721367144079",
			"tac1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqkfj7lh": "292374143198882909207027939",
			"tac1l78ukdclvydl8e0glatuj5vc30ga8m5xs78wlx": "3736999225285000000000000",
			"tac14aurf5xtlp2z695m8q8hdrvgnslmqmvxqs4lnq": "65233628611981199281858474",
			"tac173zcgurfpzkca6vgvpff6waqynvmu83rff3mkf": "50000000186061070724208656",
			"tac1txr4j9407r5kcyw7z6ppy7jw4frwrhufy58mqg": "299999972524288078755499732",
			"tac1n5hkyteynj8vn43cwy5wl96hajl32m9zek4p8k": "399899972495530457098839762",
			"tac15yjhxu4pte4872stmew5favzx0q8hx77rs7g6w": "146983345869501806223421430",
		},
		Transfer: map[string]Transfer{
			"tac1zgvugz06hckz00gdrft9mthdn0vlyuw76rfwvj": {To: "tac15yjhxu4pte4872stmew5favzx0q8hx77rs7g6w", Amount: "777956288465320278632855921"},
		},
	},
}

// Modules for mint/burn (verified against app.go maccPerms):
// mint has {Minter} (only), gov has {Burner}.
const (
	minterModule = "mint"
	burnerModule = "gov"
)

// Keepers is the minimal keeper set, satisfied by the app's real keepers.
type Keepers struct {
	Account AccountKeeper
	Bank    BankKeeper
	Staking StakingKeeper
}
type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
}
type BankKeeper interface {
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetSupply(ctx context.Context, denom string) sdk.Coin
	MintCoins(ctx context.Context, module string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, module string, amt sdk.Coins) error
	SendCoins(ctx context.Context, from, to sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx context.Context, from, to string, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, from sdk.AccAddress, to string, amt sdk.Coins) error
}
type StakingKeeper interface {
	BondDenom(ctx context.Context) (string, error)
	GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error)
}

// Migrate applies the supply-neutral recovery for the already-selected params p.
// All-or-nothing: any returned error must make the PreBlocker panic.
func Migrate(ctx sdk.Context, k Keepers, p Params) error {
	logger := ctx.Logger()
	denom, err := k.Staking.BondDenom(ctx)
	if err != nil {
		return fmt.Errorf("bond denom: %w", err)
	}
	toInt := func(s string) math.Int {
		v, ok := math.NewIntFromString(s)
		if !ok {
			panic("recovery: bad int literal " + s)
		}
		return v
	}

	poolAddr := k.Account.GetModuleAddress(stakingtypes.BondedPoolName)
	burnAddrs := sortedKeys(p.ToBurn)   // deterministic iteration for consensus
	xferAddrs := sortedKeys(p.Transfer) // deterministic iteration for consensus

	// supply-neutral by construction: mint == total burned. Also validate addresses.
	burnTotal := math.ZeroInt()
	for _, a := range burnAddrs {
		mustAddr(a)
		burnTotal = burnTotal.Add(toInt(p.ToBurn[a]))
	}
	for _, a := range xferAddrs {
		mustAddr(a)
		mustAddr(p.Transfer[a].To)
	}
	if minted := toInt(p.PoolRestore); !minted.Equal(burnTotal) {
		return fmt.Errorf("params not supply-neutral: mint %s != burn %s", minted, burnTotal)
	}

	supplyBefore := k.Bank.GetSupply(ctx, denom).Amount

	// PREFLIGHT: the hardcoded target must match the live bonded set.
	sumBonded, err := sumBondedValidatorTokens(ctx, k)
	if err != nil {
		return err
	}
	if want := toInt(p.PoolTargetAfter); !sumBonded.Equal(want) {
		diff := sumBonded.Sub(want)
		logger.Error("recovery INCONSISTENT: sum bonded != poolTargetAfter",
			"sum_bonded", sumBonded.String(), "want", want.String(), "diff", diff.String())
		return fmt.Errorf("bonded sum %s != poolTargetAfter %s", sumBonded, want)
	}

	// PRECONDITIONS: pool drained as audited; every source can cover what leaves it
	// (its burn plus, if it is also a transfer source, the transferred remainder).
	if err := eq("bonded_pool before", bal(ctx, k, poolAddr, denom), toInt(p.PoolBefore)); err != nil {
		return err
	}
	for _, a := range burnAddrs {
		need := toInt(p.ToBurn[a])
		if t, ok := p.Transfer[a]; ok {
			need = need.Add(toInt(t.Amount))
		}
		if err := gte("source "+a, bal(ctx, k, mustAddr(a), denom), need); err != nil {
			return err
		}
	}
	for _, a := range xferAddrs {
		if _, ok := p.ToBurn[a]; ok {
			continue // already covered above (burn + transfer)
		}
		if err := gte("transfer "+a, bal(ctx, k, mustAddr(a), denom), toInt(p.Transfer[a].Amount)); err != nil {
			return err
		}
	}

	// APPLY: restore the pool, burn every ToBurn, then move every Transfer remainder.
	if err := mintTo(ctx, k, stakingtypes.BondedPoolName, denom, toInt(p.PoolRestore)); err != nil {
		return err
	}
	for _, a := range burnAddrs {
		if err := burnFrom(ctx, k, mustAddr(a), denom, toInt(p.ToBurn[a]), "burn:"+a); err != nil {
			return err
		}
	}
	// Snapshot each transfer recipient AFTER the burns (a recipient may itself be a
	// burn source): its expected end balance is (post-burn balance + amount received).
	recvBefore := make(map[string]math.Int, len(xferAddrs))
	recvTotal := make(map[string]math.Int, len(xferAddrs))
	for _, a := range xferAddrs {
		to := p.Transfer[a].To
		if _, ok := recvBefore[to]; !ok {
			recvBefore[to] = bal(ctx, k, mustAddr(to), denom)
			recvTotal[to] = math.ZeroInt()
		}
		recvTotal[to] = recvTotal[to].Add(toInt(p.Transfer[a].Amount))
	}
	for _, a := range xferAddrs {
		t := p.Transfer[a]
		c := sdk.NewCoins(sdk.NewCoin(denom, toInt(t.Amount)))
		if err := k.Bank.SendCoins(ctx, mustAddr(a), mustAddr(t.To), c); err != nil {
			return fmt.Errorf("transfer %s -> %s: %w", a, t.To, err)
		}
	}

	// POSTCONDITIONS: pool solvent, each recipient credited, supply unchanged.
	if err := eq("bank(pool) == sum bonded", bal(ctx, k, poolAddr, denom), sumBonded); err != nil {
		return err
	}
	for _, to := range sortedKeys(recvBefore) {
		want := recvBefore[to].Add(recvTotal[to])
		if err := eq("recipient "+to, bal(ctx, k, mustAddr(to), denom), want); err != nil {
			return err
		}
	}
	if supplyAfter := k.Bank.GetSupply(ctx, denom).Amount; !supplyAfter.Equal(supplyBefore) {
		return fmt.Errorf("supply changed: before %s after %s (mint must equal burn)", supplyBefore, supplyAfter)
	}
	logger.Info("recovery migration OK", "chain_id", ctx.ChainID(), "height", ctx.BlockHeight(),
		"pool_restored", p.PoolRestore, "burned", burnTotal.String(), "supply_delta", "0")
	return nil
}

// ---- helpers ----
func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}
func bal(ctx sdk.Context, k Keepers, a sdk.AccAddress, denom string) math.Int {
	return k.Bank.GetBalance(ctx, a, denom).Amount
}
func eq(tag string, got, want math.Int) error {
	if !got.Equal(want) {
		return fmt.Errorf("guard %q: got %s want %s", tag, got, want)
	}
	return nil
}
func gte(tag string, got, min math.Int) error {
	if got.LT(min) {
		return fmt.Errorf("guard %q: got %s < required %s", tag, got, min)
	}
	return nil
}
func mustAddr(b string) sdk.AccAddress {
	a, err := sdk.AccAddressFromBech32(b)
	if err != nil {
		panic(fmt.Sprintf("recovery: bad addr %q: %v", b, err))
	}
	return a
}
func mintTo(ctx sdk.Context, k Keepers, module, denom string, amt math.Int) error {
	c := sdk.NewCoins(sdk.NewCoin(denom, amt))
	if err := k.Bank.MintCoins(ctx, minterModule, c); err != nil {
		return fmt.Errorf("mint: %w", err)
	}
	return k.Bank.SendCoinsFromModuleToModule(ctx, minterModule, module, c)
}
func burnFrom(ctx sdk.Context, k Keepers, addr sdk.AccAddress, denom string, amt math.Int, tag string) error {
	c := sdk.NewCoins(sdk.NewCoin(denom, amt))
	if err := k.Bank.SendCoinsFromAccountToModule(ctx, addr, burnerModule, c); err != nil {
		return fmt.Errorf("%s send-to-burn: %w", tag, err)
	}
	if err := k.Bank.BurnCoins(ctx, burnerModule, c); err != nil {
		return fmt.Errorf("%s burn: %w", tag, err)
	}
	return nil
}

// sumBondedValidatorTokens = sum of tokens of all BONDED validators - the amount
// the bonded pool must hold (staking ModuleAccountInvariant). Independent of the
// drained pool bank balance.
func sumBondedValidatorTokens(ctx sdk.Context, k Keepers) (math.Int, error) {
	vals, err := k.Staking.GetAllValidators(ctx)
	if err != nil {
		return math.ZeroInt(), err
	}
	sum := math.ZeroInt()
	for _, v := range vals {
		if v.IsBonded() {
			sum = sum.Add(v.GetTokens())
		}
	}
	return sum, nil
}
