package v160

// old_evm_params.go declares a Go struct that mirrors the x/vm Params
// wire format from cosmos/evm @ b1c973f (the version used in tacchain v1.0.4).
//
// Field numbers differed from the current cosmos/evm v0.6.0 layout:
//
//   Old → New (current evmvmtypes.Params)
//   1  evm_denom                 → 1   (unchanged)
//   4  extra_eips                → 4   (unchanged)
//   5  chain_config              → dropped
//   6  allow_unprotected_txs     → dropped
//   8  evm_channels              → 7
//   9  access_control            → 8
//   10 active_static_precompiles → 9
//
// We keep this struct purely for proto.Unmarshal of the old KV bytes;
// it is never written to the store.

import (
	evmvmtypes "github.com/cosmos/evm/x/vm/types"
	proto "github.com/cosmos/gogoproto/proto"
)

// oldEVMParams mirrors the cosmos/evm @ b1c973f Params binary layout.
// Fields 5 (chain_config) and 6 (allow_unprotected_txs) are intentionally
// omitted — unknown fields are ignored by proto.Unmarshal.
type oldEVMParams struct {
	EvmDenom                string                   `protobuf:"bytes,1,opt,name=evm_denom,json=evmDenom,proto3"`
	ExtraEIPs               []int64                  `protobuf:"varint,4,rep,packed,name=extra_eips,json=extraEips,proto3"`
	EVMChannels             []string                 `protobuf:"bytes,8,rep,name=evm_channels,json=evmChannels,proto3"`
	AccessControl           evmvmtypes.AccessControl `protobuf:"bytes,9,opt,name=access_control,json=accessControl,proto3"`
	ActiveStaticPrecompiles []string                 `protobuf:"bytes,10,rep,name=active_static_precompiles,json=activeStaticPrecompiles,proto3"`
}

func (m *oldEVMParams) Reset()         { *m = oldEVMParams{} }
func (m *oldEVMParams) String() string { return proto.CompactTextString(m) }
func (m *oldEVMParams) ProtoMessage()  {}

// oldEVMMsgUpdateParams mirrors the old MsgUpdateParams payload embedded in
// historical x/gov proposals. The top-level message did not change, but its
// Params field did.
type oldEVMMsgUpdateParams struct {
	Authority string       `protobuf:"bytes,1,opt,name=authority,proto3"`
	Params    oldEVMParams `protobuf:"bytes,2,opt,name=params,proto3"`
}

func (m *oldEVMMsgUpdateParams) Reset()         { *m = oldEVMMsgUpdateParams{} }
func (m *oldEVMMsgUpdateParams) String() string { return proto.CompactTextString(m) }
func (m *oldEVMMsgUpdateParams) ProtoMessage()  {}
