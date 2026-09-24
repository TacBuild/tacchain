package legacyeth

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// legacyFields holds the union of the three pre-v1.6.0 TxData shapes. The field
// numbers differ between them, so each parser fills what it has and buildEthTx
// assembles the go-ethereum transaction.
type legacyFields struct {
	chainID   *big.Int
	nonce     uint64
	gasPrice  *big.Int
	gasTipCap *big.Int
	gasFeeCap *big.Int
	gas       uint64
	to        *common.Address
	value     *big.Int
	data      []byte
	accesses  ethtypes.AccessList
	v, r, s   *big.Int
}

// buildEthTx reconstructs the signed Ethereum transaction from the legacy TxData.
func buildEthTx(url string, body []byte) (*ethtypes.Transaction, error) {
	f := legacyFields{}
	var err error

	switch url {
	case legacyTxURL:
		err = parseLegacyTx(body, &f)
		if err != nil {
			return nil, err
		}
		return ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce:    f.nonce,
			GasPrice: orZero(f.gasPrice),
			Gas:      f.gas,
			To:       f.to,
			Value:    orZero(f.value),
			Data:     f.data,
			V:        orZero(f.v),
			R:        orZero(f.r),
			S:        orZero(f.s),
		}), nil

	case accessListTxURL:
		err = parseAccessListTx(body, &f)
		if err != nil {
			return nil, err
		}
		return ethtypes.NewTx(&ethtypes.AccessListTx{
			ChainID:    orZero(f.chainID),
			Nonce:      f.nonce,
			GasPrice:   orZero(f.gasPrice),
			Gas:        f.gas,
			To:         f.to,
			Value:      orZero(f.value),
			Data:       f.data,
			AccessList: f.accesses,
			V:          orZero(f.v),
			R:          orZero(f.r),
			S:          orZero(f.s),
		}), nil

	case dynamicFeeTxURL:
		err = parseDynamicFeeTx(body, &f)
		if err != nil {
			return nil, err
		}
		return ethtypes.NewTx(&ethtypes.DynamicFeeTx{
			ChainID:    orZero(f.chainID),
			Nonce:      f.nonce,
			GasTipCap:  orZero(f.gasTipCap),
			GasFeeCap:  orZero(f.gasFeeCap),
			Gas:        f.gas,
			To:         f.to,
			Value:      orZero(f.value),
			Data:       f.data,
			AccessList: f.accesses,
			V:          orZero(f.v),
			R:          orZero(f.r),
			S:          orZero(f.s),
		}), nil

	default:
		return nil, fmt.Errorf("unsupported legacy tx data %q", url)
	}
}

// LegacyTx: 1 nonce, 2 gas_price, 3 gas, 4 to, 5 value, 6 data, 7 v, 8 r, 9 s.
func parseLegacyTx(b []byte, f *legacyFields) error {
	return walk(b, func(field, wire int, val []byte, num uint64) error {
		switch field {
		case 1:
			f.nonce = num
		case 2:
			f.gasPrice = decInt(val)
		case 3:
			f.gas = num
		case 4:
			f.to = decAddr(val)
		case 5:
			f.value = decInt(val)
		case 6:
			f.data = append([]byte(nil), val...)
		case 7:
			f.v = new(big.Int).SetBytes(val)
		case 8:
			f.r = new(big.Int).SetBytes(val)
		case 9:
			f.s = new(big.Int).SetBytes(val)
		}
		return nil
	})
}

// AccessListTx: 1 chain_id, 2 nonce, 3 gas_price, 4 gas, 5 to, 6 value, 7 data,
// 8 accesses, 9 v, 10 r, 11 s.
func parseAccessListTx(b []byte, f *legacyFields) error {
	return walk(b, func(field, wire int, val []byte, num uint64) error {
		switch field {
		case 1:
			f.chainID = decInt(val)
		case 2:
			f.nonce = num
		case 3:
			f.gasPrice = decInt(val)
		case 4:
			f.gas = num
		case 5:
			f.to = decAddr(val)
		case 6:
			f.value = decInt(val)
		case 7:
			f.data = append([]byte(nil), val...)
		case 8:
			tup, err := parseAccessTuple(val)
			if err != nil {
				return err
			}
			f.accesses = append(f.accesses, tup)
		case 9:
			f.v = new(big.Int).SetBytes(val)
		case 10:
			f.r = new(big.Int).SetBytes(val)
		case 11:
			f.s = new(big.Int).SetBytes(val)
		}
		return nil
	})
}

// DynamicFeeTx: 1 chain_id, 2 nonce, 3 gas_tip_cap, 4 gas_fee_cap, 5 gas, 6 to,
// 7 value, 8 data, 9 accesses, 10 v, 11 r, 12 s.
func parseDynamicFeeTx(b []byte, f *legacyFields) error {
	return walk(b, func(field, wire int, val []byte, num uint64) error {
		switch field {
		case 1:
			f.chainID = decInt(val)
		case 2:
			f.nonce = num
		case 3:
			f.gasTipCap = decInt(val)
		case 4:
			f.gasFeeCap = decInt(val)
		case 5:
			f.gas = num
		case 6:
			f.to = decAddr(val)
		case 7:
			f.value = decInt(val)
		case 8:
			f.data = append([]byte(nil), val...)
		case 9:
			tup, err := parseAccessTuple(val)
			if err != nil {
				return err
			}
			f.accesses = append(f.accesses, tup)
		case 10:
			f.v = new(big.Int).SetBytes(val)
		case 11:
			f.r = new(big.Int).SetBytes(val)
		case 12:
			f.s = new(big.Int).SetBytes(val)
		}
		return nil
	})
}

// AccessTuple: 1 address (hex string), 2 storage_keys (repeated hex string).
func parseAccessTuple(b []byte) (ethtypes.AccessTuple, error) {
	var t ethtypes.AccessTuple
	err := walk(b, func(field, wire int, val []byte, _ uint64) error {
		switch field {
		case 1:
			if a := decAddr(val); a != nil {
				t.Address = *a
			}
		case 2:
			t.StorageKeys = append(t.StorageKeys, common.HexToHash(string(val)))
		}
		return nil
	})
	return t, err
}

func orZero(v *big.Int) *big.Int {
	if v == nil {
		return new(big.Int)
	}
	return v
}

// decInt reads a cosmossdk.io/math.Int, which is serialised as its decimal string.
func decInt(val []byte) *big.Int {
	if len(val) == 0 {
		return new(big.Int)
	}
	v, ok := new(big.Int).SetString(string(val), 10)
	if !ok {
		return new(big.Int)
	}
	return v
}

// decAddr reads the hex-formatted recipient. An empty string means contract
// creation, which must stay nil rather than becoming the zero address.
func decAddr(val []byte) *common.Address {
	s := string(val)
	if s == "" {
		return nil
	}
	a := common.HexToAddress(s)
	return &a
}
