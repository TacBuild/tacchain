package v163

import (
	"bytes"
	"testing"
	"time"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	coreaddress "cosmossdk.io/core/address"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtestutil "github.com/cosmos/cosmos-sdk/x/staking/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// rekeyUBD is the only hand-written state surgery in the migration, and the
// failure it can produce is the worst one available: if the {dest, validator}
// pair never lands in the completion-time queue slice, DequeueAllMatureUBDQueue
// never yields it, CompleteUnbonding is never called, and the stake stays locked
// in not_bonded_tokens_pool forever. The chain does not even notice - a
// CompleteUnbonding error is swallowed with `continue` in the staking EndBlocker.
//
// Every test below therefore ends by draining the mature queue and asserting the
// payout would actually fire, and fire for the destination only.

func newRekeyTestKeeper(t *testing.T) (sdk.Context, *stakingkeeper.Keeper, coreaddress.Codec) {
	t.Helper()

	key := storetypes.NewKVStoreKey(stakingtypes.StoreKey)
	storeService := runtime.NewKVStoreService(key)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx.WithBlockHeader(cmtproto.Header{Time: time.Now().UTC()})

	encCfg := moduletestutil.MakeTestEncodingConfig()
	ctrl := gomock.NewController(t)

	accCodec := address.NewBech32Codec("tac")
	accountKeeper := stakingtestutil.NewMockAccountKeeper(ctrl)
	accountKeeper.EXPECT().AddressCodec().Return(accCodec).AnyTimes()
	accountKeeper.EXPECT().
		GetModuleAddress(stakingtypes.BondedPoolName).
		Return(authtypes.NewModuleAddress(stakingtypes.BondedPoolName)).AnyTimes()
	accountKeeper.EXPECT().
		GetModuleAddress(stakingtypes.NotBondedPoolName).
		Return(authtypes.NewModuleAddress(stakingtypes.NotBondedPoolName)).AnyTimes()

	keeper := stakingkeeper.NewKeeper(
		encCfg.Codec,
		storeService,
		accountKeeper,
		stakingtestutil.NewMockBankKeeper(ctrl),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		address.NewBech32Codec("tacvaloper"),
		address.NewBech32Codec("tacvalcons"),
	)
	require.NoError(t, keeper.SetParams(ctx, stakingtypes.DefaultParams()))

	return ctx, keeper, accCodec
}

type rekeyFixture struct {
	ctx      sdk.Context
	sk       *stakingkeeper.Keeper
	accCodec coreaddress.Codec
	val      sdk.ValAddress
	valStr   string
	oldOwner sdk.AccAddress
	oldStr   string
	dest     sdk.AccAddress
	destStr  string
}

func newRekeyFixture(t *testing.T) rekeyFixture {
	t.Helper()
	ctx, sk, accCodec := newRekeyTestKeeper(t)

	val := sdk.ValAddress(bytes.Repeat([]byte{0x01}, 20))
	valStr, err := sk.ValidatorAddressCodec().BytesToString(val)
	require.NoError(t, err)

	oldOwner := sdk.AccAddress(bytes.Repeat([]byte{0x02}, 20))
	oldStr, err := accCodec.BytesToString(oldOwner)
	require.NoError(t, err)

	dest := sdk.AccAddress(bytes.Repeat([]byte{0x03}, 20))
	destStr, err := accCodec.BytesToString(dest)
	require.NoError(t, err)

	return rekeyFixture{ctx, sk, accCodec, val, valStr, oldOwner, oldStr, dest, destStr}
}

// seed writes an unbonding delegation owned by oldOwner with one entry per
// completion time, exactly as Undelegate would have.
func (f rekeyFixture) seed(t *testing.T, amounts []int64, times []time.Time) {
	t.Helper()
	require.Equal(t, len(amounts), len(times))
	for i := range amounts {
		ubd, err := f.sk.SetUnbondingDelegationEntry(
			f.ctx, f.oldOwner, f.val, int64(100+i), times[i], math.NewInt(amounts[i]))
		require.NoError(t, err)
		require.NoError(t, f.sk.InsertUBDQueue(f.ctx, ubd, times[i]))
	}
}

// assertPayoutTargets drains everything mature as of `at` and reports which
// (delegator, validator) pairs the staking EndBlocker would hand to
// CompleteUnbonding. Destructive - call it last.
func (f rekeyFixture) assertPayoutTargets(t *testing.T, at time.Time, wantDelegators ...string) {
	t.Helper()
	pairs, err := f.sk.DequeueAllMatureUBDQueue(f.ctx, at)
	require.NoError(t, err)

	got := make([]string, 0, len(pairs))
	for _, p := range pairs {
		require.Equal(t, f.valStr, p.ValidatorAddress)
		got = append(got, p.DelegatorAddress)
	}
	require.ElementsMatch(t, wantDelegators, got,
		"queue would pay the wrong set of delegators")
	require.NotContains(t, got, f.oldStr, "the compromised owner is still queued for payout")
}

func TestRekeyUBD_SingleEntry(t *testing.T) {
	f := newRekeyFixture(t)
	due := f.ctx.BlockTime().Add(24 * time.Hour)
	f.seed(t, []int64{5_000}, []time.Time{due})

	ubd, err := f.sk.GetUnbondingDelegation(f.ctx, f.oldOwner, f.val)
	require.NoError(t, err)

	moved, err := rekeyUBD(f.ctx, f.sk, f.accCodec, ubd, f.dest)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(5_000), moved)

	// The old record is gone and the new one carries the entry unchanged.
	_, err = f.sk.GetUnbondingDelegation(f.ctx, f.oldOwner, f.val)
	require.Error(t, err, "the record must not remain under the compromised owner")

	got, err := f.sk.GetUnbondingDelegation(f.ctx, f.dest, f.val)
	require.NoError(t, err)
	require.Len(t, got.Entries, 1)
	require.Equal(t, math.NewInt(5_000), got.Entries[0].Balance)
	require.True(t, due.Equal(got.Entries[0].CompletionTime),
		"completion time must not move: %s != %s", got.Entries[0].CompletionTime, due)

	// The UnbondingId index must point at the new key, not dangle at the old one.
	byID, err := f.sk.GetUnbondingDelegationByUnbondingID(f.ctx, got.Entries[0].UnbondingId)
	require.NoError(t, err)
	require.Equal(t, f.destStr, byID.DelegatorAddress)

	f.assertPayoutTargets(t, due.Add(time.Second), f.destStr)
}

// TOE 6's exact shape: one record, two entries, two different completion times.
// Each time has its own queue slice, so both have to be rewritten.
func TestRekeyUBD_TwoEntriesDifferentTimes(t *testing.T) {
	f := newRekeyFixture(t)
	first := f.ctx.BlockTime().Add(24 * time.Hour)
	second := first.Add(55 * time.Minute)
	f.seed(t, []int64{4_999_900, 1}, []time.Time{first, second})

	ubd, err := f.sk.GetUnbondingDelegation(f.ctx, f.oldOwner, f.val)
	require.NoError(t, err)
	require.Len(t, ubd.Entries, 2)

	moved, err := rekeyUBD(f.ctx, f.sk, f.accCodec, ubd, f.dest)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(4_999_901), moved)

	got, err := f.sk.GetUnbondingDelegation(f.ctx, f.dest, f.val)
	require.NoError(t, err)
	require.Len(t, got.Entries, 2)
	for _, e := range got.Entries {
		byID, err := f.sk.GetUnbondingDelegationByUnbondingID(f.ctx, e.UnbondingId)
		require.NoError(t, err)
		require.Equal(t, f.destStr, byID.DelegatorAddress)
	}

	// The earlier slice alone must already fire for dest.
	f.assertPayoutTargets(t, first.Add(time.Second), f.destStr)
	// And so must the later one, independently.
	f.assertPayoutTargets(t, second.Add(time.Second), f.destStr)
}

// Two entries sharing a completion time collapse to a single queue slice. The
// destination pair must be appended once, not twice.
func TestRekeyUBD_TwoEntriesSameTime(t *testing.T) {
	f := newRekeyFixture(t)
	due := f.ctx.BlockTime().Add(24 * time.Hour)
	f.seed(t, []int64{700, 300}, []time.Time{due, due})

	ubd, err := f.sk.GetUnbondingDelegation(f.ctx, f.oldOwner, f.val)
	require.NoError(t, err)

	moved, err := rekeyUBD(f.ctx, f.sk, f.accCodec, ubd, f.dest)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(1_000), moved)

	slice, err := f.sk.GetUBDQueueTimeSlice(f.ctx, due)
	require.NoError(t, err)
	require.Len(t, slice, 1, "the destination pair was appended more than once")

	f.assertPayoutTargets(t, due.Add(time.Second), f.destStr)
}

// Unrelated pairs sharing the same completion-time slice must survive untouched:
// the slice is rewritten wholesale, so a careless overwrite would silently strand
// somebody else's unbonding.
func TestRekeyUBD_PreservesUnrelatedQueueEntries(t *testing.T) {
	f := newRekeyFixture(t)
	due := f.ctx.BlockTime().Add(24 * time.Hour)
	f.seed(t, []int64{5_000}, []time.Time{due})

	bystander := sdk.AccAddress(bytes.Repeat([]byte{0x09}, 20))
	bystanderStr, err := f.accCodec.BytesToString(bystander)
	require.NoError(t, err)
	other, err := f.sk.SetUnbondingDelegationEntry(f.ctx, bystander, f.val, 100, due, math.NewInt(42))
	require.NoError(t, err)
	require.NoError(t, f.sk.InsertUBDQueue(f.ctx, other, due))

	ubd, err := f.sk.GetUnbondingDelegation(f.ctx, f.oldOwner, f.val)
	require.NoError(t, err)
	_, err = rekeyUBD(f.ctx, f.sk, f.accCodec, ubd, f.dest)
	require.NoError(t, err)

	f.assertPayoutTargets(t, due.Add(time.Second), f.destStr, bystanderStr)
}

// If the destination already holds an unbonding delegation against the same
// validator, the entries must merge. Overwriting would destroy whatever was
// already there; aborting would strand the rescue.
func TestRekeyUBD_MergesIntoExistingDestinationRecord(t *testing.T) {
	f := newRekeyFixture(t)
	mine := f.ctx.BlockTime().Add(12 * time.Hour)
	due := f.ctx.BlockTime().Add(24 * time.Hour)

	existing, err := f.sk.SetUnbondingDelegationEntry(f.ctx, f.dest, f.val, 50, mine, math.NewInt(11))
	require.NoError(t, err)
	require.NoError(t, f.sk.InsertUBDQueue(f.ctx, existing, mine))

	f.seed(t, []int64{5_000}, []time.Time{due})
	ubd, err := f.sk.GetUnbondingDelegation(f.ctx, f.oldOwner, f.val)
	require.NoError(t, err)

	_, err = rekeyUBD(f.ctx, f.sk, f.accCodec, ubd, f.dest)
	require.NoError(t, err)

	got, err := f.sk.GetUnbondingDelegation(f.ctx, f.dest, f.val)
	require.NoError(t, err)
	require.Len(t, got.Entries, 2, "the destination's pre-existing entry was lost")

	total := math.ZeroInt()
	for _, e := range got.Entries {
		total = total.Add(e.Balance)
	}
	require.Equal(t, math.NewInt(5_011), total)
}

// Re-keying to the address that already owns the record must be a no-op rather
// than a delete-and-reinsert that could drop the queue pair.
func TestRekeyUBD_SameOwnerIsNoop(t *testing.T) {
	f := newRekeyFixture(t)
	due := f.ctx.BlockTime().Add(24 * time.Hour)
	f.seed(t, []int64{5_000}, []time.Time{due})

	ubd, err := f.sk.GetUnbondingDelegation(f.ctx, f.oldOwner, f.val)
	require.NoError(t, err)

	moved, err := rekeyUBD(f.ctx, f.sk, f.accCodec, ubd, f.oldOwner)
	require.NoError(t, err)
	require.True(t, moved.IsZero())

	got, err := f.sk.GetUnbondingDelegation(f.ctx, f.oldOwner, f.val)
	require.NoError(t, err)
	require.Len(t, got.Entries, 1)

	pairs, err := f.sk.DequeueAllMatureUBDQueue(f.ctx, due.Add(time.Second))
	require.NoError(t, err)
	require.Len(t, pairs, 1, "the record must still be queued for payout")
	require.Equal(t, f.oldStr, pairs[0].DelegatorAddress)
}
