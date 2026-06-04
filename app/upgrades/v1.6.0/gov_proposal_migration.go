package v160

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/gogoproto/proto"
	gogoany "github.com/cosmos/gogoproto/types/any"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	evmvmtypes "github.com/cosmos/evm/x/vm/types"
)

const evmMsgUpdateParamsTypeURL = "/cosmos.evm.vm.v1.MsgUpdateParams"

// MigrateHistoricalGovEVMParamProposals rewrites old x/vm MsgUpdateParams
// payloads embedded in stored x/gov proposals without unpacking proposal
// interfaces. This repairs historical proposals that were submitted before the
// cosmos/evm Params protobuf field numbers changed.
func MigrateHistoricalGovEVMParamProposals(ctx sdk.Context, ak *upgrades.AppKeepers) error {
	storeKey := ak.GetStoreKey(govtypes.StoreKey)
	if storeKey == nil {
		return fmt.Errorf("gov store key not found")
	}
	store := ctx.KVStore(storeKey)

	var scannedProposals, scannedMessages, rewrittenMessages uint64

	iterator := storetypes.KVStorePrefixIterator(store, govtypes.ProposalsKeyPrefix.Bytes())
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		scannedProposals++

		var proposal govv1.Proposal
		if err := proto.Unmarshal(iterator.Value(), &proposal); err != nil {
			return fmt.Errorf("unmarshal gov proposal at key %x: %w", iterator.Key(), err)
		}

		proposalRewritten := false
		for i, msg := range proposal.Messages {
			scannedMessages++

			rewritten, err := migrateHistoricalGovEVMParamProposalMessage(msg)
			if err != nil {
				return fmt.Errorf("proposal %d message %d: %w", proposal.Id, i, err)
			}
			if rewritten {
				proposalRewritten = true
				rewrittenMessages++
			}
		}

		if !proposalRewritten {
			continue
		}

		bz, err := proto.Marshal(&proposal)
		if err != nil {
			return fmt.Errorf("marshal repaired gov proposal %d: %w", proposal.Id, err)
		}
		store.Set(iterator.Key(), bz)
	}

	ctx.Logger().Info(
		"Historical gov proposal EVM params migration complete",
		"proposals_scanned", scannedProposals,
		"messages_scanned", scannedMessages,
		"messages_rewritten", rewrittenMessages,
	)

	return nil
}

func migrateHistoricalGovEVMParamProposalMessage(msg *gogoany.Any) (bool, error) {
	if msg == nil || msg.TypeUrl != evmMsgUpdateParamsTypeURL {
		return false, nil
	}

	var current evmvmtypes.MsgUpdateParams
	currentErr := proto.Unmarshal(msg.Value, &current)
	hasOldWire, wireErr := msgUpdateParamsHasOldEVMParamsWire(msg.Value)
	if wireErr != nil {
		return false, fmt.Errorf("inspect MsgUpdateParams wire layout: %w", wireErr)
	}
	if currentErr == nil && !hasOldWire {
		return false, nil
	}

	var old oldEVMMsgUpdateParams
	if err := proto.Unmarshal(msg.Value, &old); err != nil {
		if currentErr != nil {
			return false, fmt.Errorf("unmarshal current MsgUpdateParams: %w; unmarshal old MsgUpdateParams: %v", currentErr, err)
		}
		return false, fmt.Errorf("unmarshal old MsgUpdateParams: %w", err)
	}

	newMsg := evmvmtypes.MsgUpdateParams{
		Authority: old.Authority,
		Params: evmvmtypes.Params{
			EvmDenom:                old.Params.EvmDenom,
			ExtraEIPs:               old.Params.ExtraEIPs,
			EVMChannels:             old.Params.EVMChannels,
			AccessControl:           old.Params.AccessControl,
			ActiveStaticPrecompiles: old.Params.ActiveStaticPrecompiles,
			HistoryServeWindow:      evmvmtypes.DefaultHistoryServeWindow,
			ExtendedDenomOptions:    nil,
		},
	}

	bz, err := proto.Marshal(&newMsg)
	if err != nil {
		return false, fmt.Errorf("marshal current MsgUpdateParams: %w", err)
	}
	msg.Value = bz

	return true, nil
}

func msgUpdateParamsHasOldEVMParamsWire(bz []byte) (bool, error) {
	for len(bz) > 0 {
		num, typ, tagLen := protowire.ConsumeTag(bz)
		if tagLen < 0 {
			return false, protowire.ParseError(tagLen)
		}
		bz = bz[tagLen:]

		if num == 2 {
			if typ != protowire.BytesType {
				return false, fmt.Errorf("unexpected wire type %d for MsgUpdateParams.params", typ)
			}
			paramsBz, valueLen := protowire.ConsumeBytes(bz)
			if valueLen < 0 {
				return false, protowire.ParseError(valueLen)
			}
			return evmParamsHasOldLayoutWire(paramsBz), nil
		}

		valueLen := protowire.ConsumeFieldValue(num, typ, bz)
		if valueLen < 0 {
			return false, protowire.ParseError(valueLen)
		}
		bz = bz[valueLen:]
	}

	return false, nil
}

func evmParamsHasOldLayoutWire(bz []byte) bool {
	for len(bz) > 0 {
		num, typ, tagLen := protowire.ConsumeTag(bz)
		if tagLen < 0 {
			return false
		}
		bz = bz[tagLen:]

		if num == 10 && typ == protowire.BytesType {
			return true
		}

		valueLen := protowire.ConsumeFieldValue(num, typ, bz)
		if valueLen < 0 {
			return false
		}
		bz = bz[valueLen:]
	}

	return false
}
