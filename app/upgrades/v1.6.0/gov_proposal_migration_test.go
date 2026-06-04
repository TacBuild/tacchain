package v160

import (
	"bytes"
	"testing"

	"cosmossdk.io/collections"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/gogoproto/proto"
	gogoany "github.com/cosmos/gogoproto/types/any"
	"github.com/stretchr/testify/require"

	"github.com/TacBuild/tacchain/app/upgrades"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	vm "github.com/cosmos/evm/x/vm"
	evmvmtypes "github.com/cosmos/evm/x/vm/types"
)

func TestMigrateHistoricalGovEVMParamProposalsRewritesOldPayload(t *testing.T) {
	ctx, ak, cdc := newGovProposalMigrationTest(t)

	proposal := govv1.Proposal{
		Id:       7,
		Messages: []*gogoany.Any{oldEVMMsgUpdateParamsAny(t)},
		Status:   govv1.StatusPassed,
		Title:    "old evm params",
	}
	writeRawGovProposal(t, ctx, ak, proposal)

	var decodedBefore govv1.Proposal
	err := cdc.Unmarshal(readRawGovProposal(t, ctx, ak, proposal.Id), &decodedBefore)
	require.Error(t, err)

	require.NoError(t, MigrateHistoricalGovEVMParamProposals(ctx, ak))

	var decodedAfter govv1.Proposal
	require.NoError(t, cdc.Unmarshal(readRawGovProposal(t, ctx, ak, proposal.Id), &decodedAfter))

	msgs, err := decodedAfter.GetMsgs()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	updateMsg, ok := msgs[0].(*evmvmtypes.MsgUpdateParams)
	require.True(t, ok)
	require.Equal(t, "gov-authority", updateMsg.Authority)
	require.Equal(t, "utac", updateMsg.Params.EvmDenom)
	require.Equal(t, []int64{3855}, updateMsg.Params.ExtraEIPs)
	require.Equal(t, []string{"channel-0"}, updateMsg.Params.EVMChannels)
	require.Equal(t, []string{"0x0000000000000000000000000000000000000800"}, updateMsg.Params.ActiveStaticPrecompiles)
	require.Equal(t, uint64(evmvmtypes.DefaultHistoryServeWindow), updateMsg.Params.HistoryServeWindow)
	require.Nil(t, updateMsg.Params.ExtendedDenomOptions)
}

func TestMigrateHistoricalGovEVMParamProposalsLeavesUnrelatedProposalUnchanged(t *testing.T) {
	ctx, ak, _ := newGovProposalMigrationTest(t)

	proposal := govv1.Proposal{
		Id: 1,
		Messages: []*gogoany.Any{
			{
				TypeUrl: "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
				Value:   []byte{0x0a, 0x03, 'v', '1', '6'},
			},
		},
		Status: govv1.StatusPassed,
		Title:  "software upgrade only",
	}
	writeRawGovProposal(t, ctx, ak, proposal)
	before := readRawGovProposal(t, ctx, ak, proposal.Id)

	require.NoError(t, MigrateHistoricalGovEVMParamProposals(ctx, ak))

	require.Equal(t, before, readRawGovProposal(t, ctx, ak, proposal.Id))
}

func TestMigrateHistoricalGovEVMParamProposalsLeavesCurrentPayloadUnchanged(t *testing.T) {
	ctx, ak, _ := newGovProposalMigrationTest(t)

	currentAny, err := codectypes.NewAnyWithValue(&evmvmtypes.MsgUpdateParams{
		Authority: "gov-authority",
		Params: evmvmtypes.Params{
			EvmDenom:                "utac",
			ExtraEIPs:               []int64{3855},
			EVMChannels:             []string{"channel-0"},
			AccessControl:           testAccessControl(),
			ActiveStaticPrecompiles: []string{"0x0000000000000000000000000000000000000800"},
			HistoryServeWindow:      evmvmtypes.DefaultHistoryServeWindow,
		},
	})
	require.NoError(t, err)

	proposal := govv1.Proposal{
		Id:       2,
		Messages: []*gogoany.Any{currentAny},
		Status:   govv1.StatusPassed,
		Title:    "current evm params",
	}
	writeRawGovProposal(t, ctx, ak, proposal)
	before := readRawGovProposal(t, ctx, ak, proposal.Id)

	require.NoError(t, MigrateHistoricalGovEVMParamProposals(ctx, ak))

	require.Equal(t, before, readRawGovProposal(t, ctx, ak, proposal.Id))
}

func TestMigrateHistoricalGovEVMParamProposalsIsIdempotent(t *testing.T) {
	ctx, ak, cdc := newGovProposalMigrationTest(t)

	proposal := govv1.Proposal{
		Id:       3,
		Messages: []*gogoany.Any{oldEVMMsgUpdateParamsAny(t)},
		Status:   govv1.StatusPassed,
		Title:    "old evm params",
	}
	writeRawGovProposal(t, ctx, ak, proposal)

	require.NoError(t, MigrateHistoricalGovEVMParamProposals(ctx, ak))
	afterFirstRun := readRawGovProposal(t, ctx, ak, proposal.Id)

	require.NoError(t, MigrateHistoricalGovEVMParamProposals(ctx, ak))
	require.Equal(t, afterFirstRun, readRawGovProposal(t, ctx, ak, proposal.Id))

	var decoded govv1.Proposal
	require.NoError(t, cdc.Unmarshal(afterFirstRun, &decoded))
}

func TestMigrateHistoricalGovEVMParamProposalsRequiresGovStore(t *testing.T) {
	ctx, ak, _ := newGovProposalMigrationTest(t)
	ak.GetStoreKey = func(string) *storetypes.KVStoreKey { return nil }

	err := MigrateHistoricalGovEVMParamProposals(ctx, ak)
	require.Error(t, err)
	require.Contains(t, err.Error(), "gov store key not found")
}

func newGovProposalMigrationTest(t *testing.T) (sdk.Context, *upgrades.AppKeepers, codec.Codec) {
	t.Helper()

	govKey := storetypes.NewKVStoreKey(govtypes.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, govKey, storetypes.NewTransientStoreKey("transient_test"))
	encCfg := moduletestutil.MakeTestEncodingConfig(vm.AppModuleBasic{})

	ak := &upgrades.AppKeepers{
		Codec: encCfg.Codec,
		GetStoreKey: func(storeKey string) *storetypes.KVStoreKey {
			if storeKey == govtypes.StoreKey {
				return govKey
			}
			return nil
		},
	}

	return testCtx.Ctx, ak, encCfg.Codec
}

func writeRawGovProposal(t *testing.T, ctx sdk.Context, ak *upgrades.AppKeepers, proposal govv1.Proposal) {
	t.Helper()

	bz, err := proto.Marshal(&proposal)
	require.NoError(t, err)
	ctx.KVStore(ak.GetStoreKey(govtypes.StoreKey)).Set(govProposalStoreKey(t, proposal.Id), bz)
}

func readRawGovProposal(t *testing.T, ctx sdk.Context, ak *upgrades.AppKeepers, id uint64) []byte {
	t.Helper()

	bz := ctx.KVStore(ak.GetStoreKey(govtypes.StoreKey)).Get(govProposalStoreKey(t, id))
	require.NotNil(t, bz)
	return bytes.Clone(bz)
}

func govProposalStoreKey(t *testing.T, id uint64) []byte {
	t.Helper()

	key := append([]byte(nil), govtypes.ProposalsKeyPrefix.Bytes()...)
	idKey := make([]byte, collections.Uint64Key.Size(id))
	_, err := collections.Uint64Key.Encode(idKey, id)
	require.NoError(t, err)
	return append(key, idKey...)
}

func oldEVMMsgUpdateParamsAny(t *testing.T) *gogoany.Any {
	t.Helper()

	msg := oldEVMMsgUpdateParams{
		Authority: "gov-authority",
		Params: oldEVMParams{
			EvmDenom:                "utac",
			ExtraEIPs:               []int64{3855},
			EVMChannels:             []string{"channel-0"},
			AccessControl:           testAccessControl(),
			ActiveStaticPrecompiles: []string{"0x0000000000000000000000000000000000000800"},
		},
	}
	bz, err := proto.Marshal(&msg)
	require.NoError(t, err)

	hasOldWire, err := msgUpdateParamsHasOldEVMParamsWire(bz)
	require.NoError(t, err)
	require.True(t, hasOldWire)

	var current evmvmtypes.MsgUpdateParams
	require.Error(t, proto.Unmarshal(bz, &current))

	return &gogoany.Any{
		TypeUrl: evmMsgUpdateParamsTypeURL,
		Value:   bz,
	}
}

func testAccessControl() evmvmtypes.AccessControl {
	return evmvmtypes.AccessControl{
		Create: evmvmtypes.AccessControlType{AccessType: evmvmtypes.AccessTypePermissionless},
		Call:   evmvmtypes.AccessControlType{AccessType: evmvmtypes.AccessTypePermissionless},
	}
}
