package v163

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	appconfig "github.com/TacBuild/tacchain/app/config"
)

func init() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(appconfig.Bech32PrefixAccAddr, appconfig.Bech32PrefixAccPub)
	cfg.SetBech32PrefixForValidator(appconfig.Bech32PrefixValAddr, appconfig.Bech32PrefixValPub)
}

// The refund list is the whole of Task A1: if it does not add up, the escrow is
// left with a residue or the transfer is short.
func TestRefundsSumToTotal(t *testing.T) {
	want, ok := math.NewIntFromString(RefundTotal)
	require.True(t, ok, "RefundTotal must parse")

	got := math.ZeroInt()
	seen := make(map[common.Address]struct{}, len(Refunds))
	for i, r := range Refunds {
		require.True(t, common.IsHexAddress(r.To), "Refunds[%d]: %q is not an address", i, r.To)
		addr := common.HexToAddress(r.To)
		_, dup := seen[addr]
		require.False(t, dup, "Refunds[%d]: %s appears twice", i, r.To)
		seen[addr] = struct{}{}

		amt, ok := math.NewIntFromString(r.Amount)
		require.True(t, ok, "Refunds[%d]: bad amount %q", i, r.Amount)
		require.True(t, amt.IsPositive(), "Refunds[%d]: non-positive amount", i)
		got = got.Add(amt)
	}

	require.Equal(t, 19, len(Refunds), "the 19 in-flight-message recipients")
	require.True(t, want.Equal(got), "sum %s != RefundTotal %s", got, want)
}

// Each operator account must be its validator's own address under the account
// prefix - that identity is what makes the Task B sweep legitimate: it only ever
// touches accounts that belong to the six compromised validators.
func TestTOEOperatorMatchesValoper(t *testing.T) {
	require.Equal(t, 6, len(TOEValidators))

	for _, v := range TOEValidators {
		val, err := sdk.ValAddressFromBech32(v.Valoper)
		require.NoErrorf(t, err, "%s: bad valoper", v.Moniker)

		acc, err := sdk.AccAddressFromBech32(v.Operator)
		require.NoErrorf(t, err, "%s: bad operator", v.Moniker)

		require.Equalf(t, val.Bytes(), acc.Bytes(),
			"%s: operator %s is not valoper %s under the account prefix",
			v.Moniker, v.Operator, v.Valoper)
		require.Lenf(t, acc.Bytes(), common.AddressLength, "%s: not a 20-byte address", v.Moniker)
	}
}

func TestPayoutAddressesAreDistinctAndValid(t *testing.T) {
	for name, hexAddr := range map[string]string{
		"EscrowAddr":             EscrowAddr,
		"RedirectDest":           RedirectDest,
		"FoundationMultisigAddr": FoundationMultisigAddr,
		"NewOFTAddr":             NewOFTAddr,
	} {
		require.Truef(t, common.IsHexAddress(hexAddr), "%s: %q is not an address", name, hexAddr)
	}

	// The rescue destination must never collide with a compromised account, or
	// Task B would redirect the stake straight back to the attacker.
	dest := common.HexToAddress(RedirectDest)
	for _, v := range TOEValidators {
		acc, err := sdk.AccAddressFromBech32(v.Operator)
		require.NoError(t, err)
		require.NotEqualf(t, dest.Bytes(), acc.Bytes(),
			"RedirectDest equals the %s operator account", v.Moniker)
	}
	require.NotEqual(t, common.HexToAddress(EscrowAddr), dest)
}

func TestAmountConstantsParse(t *testing.T) {
	for name, s := range map[string]string{
		"RefundTotal":       RefundTotal,
		"NewOFTFunding":     NewOFTFunding,
		"SelfStakeExpected": SelfStakeExpected,
	} {
		v, ok := math.NewIntFromString(s)
		require.Truef(t, ok, "%s: %q does not parse", name, s)
		require.Truef(t, v.IsPositive(), "%s must be positive", name)
	}
}
