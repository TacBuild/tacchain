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
	"fmt"

	proto "github.com/cosmos/gogoproto/proto"
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

// reencodeEVMParams decodes the old x/vm Params bytes (cosmos/evm @ b1c973f)
// into oldEVMParams (which carries the old field numbers) and re-encodes the
// result as evmvmtypes.Params (new field numbers).
//
// Fields dropped in the new schema (chain_config, allow_unprotected_txs) are
// silently ignored by proto.Unmarshal via the unknown-field mechanism.
// New fields that have no counterpart in the old schema (HistoryServeWindow,
// ExtendedDenomOptions) are left at their zero values — the EVM module
// migration will fill them in with defaults during RunMigrations.
func reencodeEVMParams(raw []byte) ([]byte, error) {
	var old oldEVMParams
	if err := proto.Unmarshal(raw, &old); err != nil {
		return nil, fmt.Errorf("unmarshal old EVM params: %w", err)
	}

	newParams := evmvmtypes.Params{
		EvmDenom:                old.EvmDenom,
		ExtraEIPs:               old.ExtraEIPs,
		EVMChannels:             old.EVMChannels,
		AccessControl:           old.AccessControl,
		ActiveStaticPrecompiles: old.ActiveStaticPrecompiles,
		// HistoryServeWindow and ExtendedDenomOptions stay zero;
		// RunMigrations sets the proper defaults.
	}

	out, err := proto.Marshal(&newParams)
	if err != nil {
		return nil, fmt.Errorf("marshal new EVM params: %w", err)
	}
	return out, nil
}

// migrateERC20Precompiles migrates x/erc20 precompile addresses (both native
// and dynamic lists) from the v0.2.0 storage format to the v0.6.0 format.
//
// v0.2.0 layout (single key per list, concatenated 42-byte hex strings):
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

	type parsedEntry struct {
		entry
		addrs []common.Address
	}

	seen := make(map[common.Address]string)
	parsed := make([]parsedEntry, 0, len(migrations))
	for _, m := range migrations {
		bz := store.Get(m.oldKey)
		if len(bz) == 0 {
			continue // empty list or already migrated
		}

		addrs, err := parseLegacyERC20PrecompileBlob(m.label, bz, seen)
		if err != nil {
			return err
		}
		parsed = append(parsed, parsedEntry{entry: m, addrs: addrs})
	}

	for _, m := range parsed {
		for _, addr := range m.addrs {
			if err := m.enable(ctx, addr); err != nil {
				return fmt.Errorf("failed to enable %s precompile %s: %w", m.label, addr.Hex(), err)
			}
		}
		store.Delete(m.oldKey)
	}

	return nil
}

const legacyERC20PrecompileAddrLen = 42 // len("0xAbCd...") — 42 ASCII characters

func parseLegacyERC20PrecompileBlob(
	label string,
	bz []byte,
	seen map[common.Address]string,
) ([]common.Address, error) {
	if len(bz)%legacyERC20PrecompileAddrLen != 0 {
		return nil, fmt.Errorf("%s precompiles bytes length %d is not a multiple of %d",
			label, len(bz), legacyERC20PrecompileAddrLen)
	}

	addrs := make([]common.Address, 0, len(bz)/legacyERC20PrecompileAddrLen)
	for i := 0; i < len(bz); i += legacyERC20PrecompileAddrLen {
		hexAddr := string(bz[i : i+legacyERC20PrecompileAddrLen])
		addr, err := parseLegacyERC20PrecompileAddress(label, hexAddr, i)
		if err != nil {
			return nil, err
		}

		if previous, ok := seen[addr]; ok {
			return nil, fmt.Errorf("duplicate %s precompile address %s at offset %d; already seen in %s",
				label, addr.Hex(), i, previous)
		}
		seen[addr] = fmt.Sprintf("%s offset %d", label, i)
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

func parseLegacyERC20PrecompileAddress(label, hexAddr string, offset int) (common.Address, error) {
	if !common.IsHexAddress(hexAddr) {
		return common.Address{}, fmt.Errorf("invalid %s precompile address %q at offset %d", label, hexAddr, offset)
	}

	addr := common.HexToAddress(hexAddr)
	if addr == (common.Address{}) {
		return common.Address{}, fmt.Errorf("invalid %s precompile zero address %q at offset %d", label, hexAddr, offset)
	}

	return addr, nil
}
