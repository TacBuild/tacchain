package v160

// evm_params_migration.go contains KV store migrations required when upgrading
// the EVM stack from cosmos/evm @ b1c973f (evmos-based, used in tacchain v1.0.4)
// to cosmos/evm v0.6.0.
//
// Two independent changes are handled:
//
//  1. x/vm Params proto re-encoding (migrateEVMParamsStore)
//     Proto field numbers shifted between the two versions:
//
//     Old field 1  (bytes)  evm_denom                 → New field 1  (unchanged)
//     Old field 4  (varint) extra_eips                → New field 4  (unchanged)
//     Old field 5  (bytes)  chain_config              → REMOVED (reserved)
//     Old field 6  (varint) allow_unprotected_txs     → REMOVED (reserved)
//     Old field 8  (bytes)  evm_channels              → New field 7
//     Old field 9  (bytes)  access_control            → New field 8
//     Old field 10 (bytes)  active_static_precompiles → New field 9
//                                                        New field 10 (varint) history_serve_window — zero default
//                                                        New field 11 (bytes)  extended_denom_options — nil default
//
//     Decoding the old bytes with the new schema without re-encoding would
//     silently corrupt evm_channels, access_control and active_static_precompiles.
//
//  2. x/erc20 precompile address migration (migrateERC20Precompiles)
//     Two legacy keys held the lists as concatenated 42-byte hex strings:
//       []byte("NativePrecompiles")  → new per-address KV at KeyPrefixNativePrecompiles  ({0x06})
//       []byte("DynamicPrecompiles") → new per-address KV at KeyPrefixDynamicPrecompiles ({0x07})
//     Both must be migrated; missing the dynamic list breaks all single-token-representation
//     ERC20 wrappers registered via x/erc20 RegisterERC20 (e.g. IBC tokens, token-factory tokens).

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmvmtypes "github.com/cosmos/evm/x/vm/types"
)

// migrateEVMParamsStore reads the raw x/vm Params bytes from the KV store,
// re-encodes them with the corrected field numbers, and writes them back.
// It must be called before RunMigrations so that the EVM module initialises
// with consistent data.
func migrateEVMParamsStore(ctx sdk.Context, ak *upgrades.AppKeepers) error {
	storeKey := ak.GetStoreKey(evmvmtypes.StoreKey)
	if storeKey == nil {
		return fmt.Errorf("evm store key not found")
	}
	store := ctx.KVStore(storeKey)

	raw := store.Get(evmvmtypes.KeyPrefixParams)
	if raw == nil {
		return fmt.Errorf("evm params not found in store")
	}

	newRaw, err := reencodeEVMParams(raw)
	if err != nil {
		return fmt.Errorf("failed to re-encode EVM params: %w", err)
	}

	store.Set(evmvmtypes.KeyPrefixParams, newRaw)
	return nil
}

// reencodeEVMParams parses the old Params wire bytes and emits them with the
// corrected field numbers.  We operate at the raw protobuf level to avoid
// importing the old generated code (which no longer exists in this module).
func reencodeEVMParams(old []byte) ([]byte, error) {
	type fieldPayload struct {
		wireType uint64
		data     []byte // for varint: re-encoded varint bytes; for LEN: raw content (no length prefix)
	}

	fields := map[uint32][]fieldPayload{}

	b := old
	for len(b) > 0 {
		// Read tag
		tagVal, n := decodeVarint(b)
		if n == 0 {
			return nil, fmt.Errorf("failed to decode tag")
		}
		b = b[n:]

		fieldNum := uint32(tagVal >> 3)
		wireType := tagVal & 0x7

		switch wireType {
		case 0: // varint
			val, n := decodeVarint(b)
			if n == 0 {
				return nil, fmt.Errorf("failed to decode varint for field %d", fieldNum)
			}
			b = b[n:]
			encoded := encodeVarint(val)
			fields[fieldNum] = append(fields[fieldNum], fieldPayload{wireType: 0, data: encoded})

		case 2: // length-delimited
			length, n := decodeVarint(b)
			if n == 0 {
				return nil, fmt.Errorf("failed to decode length for field %d", fieldNum)
			}
			b = b[n:]
			if uint64(len(b)) < length {
				return nil, fmt.Errorf("not enough bytes for field %d", fieldNum)
			}
			content := make([]byte, length)
			copy(content, b[:length])
			b = b[length:]
			fields[fieldNum] = append(fields[fieldNum], fieldPayload{wireType: 2, data: content})

		case 1: // 64-bit
			if len(b) < 8 {
				return nil, fmt.Errorf("not enough bytes for 64-bit field %d", fieldNum)
			}
			data := make([]byte, 8)
			copy(data, b[:8])
			b = b[8:]
			fields[fieldNum] = append(fields[fieldNum], fieldPayload{wireType: 1, data: data})

		case 5: // 32-bit
			if len(b) < 4 {
				return nil, fmt.Errorf("not enough bytes for 32-bit field %d", fieldNum)
			}
			data := make([]byte, 4)
			copy(data, b[:4])
			b = b[4:]
			fields[fieldNum] = append(fields[fieldNum], fieldPayload{wireType: 5, data: data})

		default:
			return nil, fmt.Errorf("unsupported wire type %d for field %d", wireType, fieldNum)
		}
	}

	// Old → New field number mapping.  Fields 5 (chain_config) and 6
	// (allow_unprotected_txs) are intentionally absent — they are dropped.
	oldToNew := map[uint32]uint32{
		1:  1, // evm_denom
		4:  4, // extra_eips
		8:  7, // evm_channels
		9:  8, // access_control
		10: 9, // active_static_precompiles
	}

	// Emit fields in ascending new-field-number order for deterministic output.
	emitOrder := []uint32{1, 4, 8, 9, 10}
	var out []byte
	for _, oldField := range emitOrder {
		newField, ok := oldToNew[oldField]
		if !ok {
			continue
		}
		for _, fp := range fields[oldField] {
			out = appendField(out, newField, fp.wireType, fp.data)
		}
	}

	return out, nil
}

// appendField appends one protobuf field (tag + value) to buf and returns the result.
func appendField(buf []byte, fieldNum uint32, wireType uint64, data []byte) []byte {
	tag := (uint64(fieldNum) << 3) | wireType
	buf = append(buf, encodeVarint(tag)...)
	switch wireType {
	case 0: // varint: data is already encoded varint bytes
		buf = append(buf, data...)
	case 2: // LEN: prepend length
		buf = append(buf, encodeVarint(uint64(len(data)))...)
		buf = append(buf, data...)
	case 1: // 64-bit
		buf = append(buf, data...)
	case 5: // 32-bit
		buf = append(buf, data...)
	}
	return buf
}

// migrateERC20Precompiles migrates x/erc20 precompile addresses (both native
// and dynamic lists) from the v0.1.4 storage format to the v0.6.0 format.
//
// v0.1.4 layout (single key per list, concatenated 42-byte hex strings):
//
//	store.Get([]byte("NativePrecompiles"))  → "0xAaaa...0xBbbb..."
//	store.Get([]byte("DynamicPrecompiles")) → "0xCccc...0xDddd..."
//
// v0.6.0 layout (per-address keys with a prefix byte):
//
//	{0x06}+hexAddr → 0x01 (native)
//	{0x07}+hexAddr → 0x01 (dynamic)
//
// After migration both old keys are deleted.  Missing the dynamic list would
// cause every dynamically-registered ERC20 wrapper (IBC tokens, token-factory
// tokens, etc.) to appear unregistered in the EVM, making them unusable from
// EVM tooling.
func migrateERC20Precompiles(ctx sdk.Context, ak *upgrades.AppKeepers) error {
	storeKey := ak.GetStoreKey(erc20types.StoreKey)
	if storeKey == nil {
		return fmt.Errorf("erc20 store key not found")
	}
	store := ctx.KVStore(storeKey)

	type entry struct {
		oldKey []byte
		enable func(sdk.Context, common.Address) error
		label  string
	}
	migrations := []entry{
		{
			oldKey: []byte("NativePrecompiles"),
			enable: ak.Erc20Keeper.EnableNativePrecompile,
			label:  "native",
		},
		{
			oldKey: []byte("DynamicPrecompiles"),
			enable: ak.Erc20Keeper.EnableDynamicPrecompile,
			label:  "dynamic",
		},
	}

	const addrLen = 42 // len("0xAbCd...") — 42 ASCII characters

	for _, m := range migrations {
		bz := store.Get(m.oldKey)
		if len(bz) == 0 {
			continue // empty list or already migrated
		}
		if len(bz)%addrLen != 0 {
			return fmt.Errorf("%s precompiles bytes length %d is not a multiple of %d",
				m.label, len(bz), addrLen)
		}
		for i := 0; i < len(bz); i += addrLen {
			hexAddr := string(bz[i : i+addrLen])
			addr := common.HexToAddress(hexAddr)
			if err := m.enable(ctx, addr); err != nil {
				return fmt.Errorf("failed to enable %s precompile %s: %w", m.label, hexAddr, err)
			}
		}
		store.Delete(m.oldKey)
	}

	return nil
}

// decodeVarint reads a protobuf varint from b and returns (value, bytesRead).
// Returns (0, 0) on error.
func decodeVarint(b []byte) (uint64, int) {
	var x uint64
	var s uint
	for i, c := range b {
		if i == 10 {
			return 0, 0
		}
		if c < 0x80 {
			if i == 9 && c > 1 {
				return 0, 0
			}
			return x | uint64(c)<<s, i + 1
		}
		x |= uint64(c&0x7f) << s
		s += 7
	}
	return 0, 0
}

// encodeVarint encodes v as a protobuf varint.
func encodeVarint(v uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	i := 0
	for v >= 0x80 {
		buf[i] = byte(v) | 0x80
		v >>= 7
		i++
	}
	buf[i] = byte(v)
	return buf[:i+1]
}
