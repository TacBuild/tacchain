// Package legacyeth restores the ability to read Ethereum transactions that were
// written before the v1.6.0 migration.
//
// Those transactions carry the type URL /cosmos.evm.vm.v1.MsgEthereumTx, which is
// still current, but the message body uses the pre-refactor field layout:
//
//	old MsgEthereumTx                 new MsgEthereumTx
//	  1 Any    data (TxData)            5 bytes      from
//	  2 double size (deprecated)        6 EthereumTx raw
//	  3 string hash
//	  4 string deprecated_from
//	  5 bytes  from
//
// The SDK decoder resolves the type and then rejects field 1 as unknown, so the
// transaction is silently dropped from every read path that decodes it:
// eth_getBlockByNumber returns an empty transactions array, eth_getTransactionByHash
// returns null, and the derived roots and bloom are computed as if the block were
// empty — while eth_getLogs, which reads the event index and needs no decoding,
// still returns the logs. The result is a block that reports gas used by zero
// transactions, which no indexer can reconcile.
//
// This package rewrites such a message into the current layout before handing it
// back to the standard decoder. It is a read-path shim: consensus never decodes
// these bytes again, and the rewritten form is only ever produced in memory.
package legacyeth

import (
	"bytes"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

const (
	msgEthereumTxURL = "/cosmos.evm.vm.v1.MsgEthereumTx"
	legacyTxURL      = "/cosmos.evm.vm.v1.LegacyTx"
	accessListTxURL  = "/cosmos.evm.vm.v1.AccessListTx"
	dynamicFeeTxURL  = "/cosmos.evm.vm.v1.DynamicFeeTx"
)

// Decoder wraps inner so that a transaction the current layout cannot parse is
// retried through the legacy rewrite. On any failure the ORIGINAL error is
// returned, so a genuinely malformed transaction still reports what is wrong with
// it rather than a confusing message from the shim.
func Decoder(inner sdk.TxDecoder) sdk.TxDecoder {
	return func(txBytes []byte) (sdk.Tx, error) {
		tx, err := inner(txBytes)
		if err == nil {
			return tx, nil
		}
		rewritten, ok := rewrite(txBytes)
		if !ok {
			return nil, err
		}
		converted, cerr := inner(rewritten)
		if cerr != nil {
			return nil, err
		}
		return converted, nil
	}
}

// rewrite converts every legacy MsgEthereumTx in the envelope. It reports false
// if nothing was converted, so the caller can fall through to the original error.
//
// Only the message bodies are touched; the envelope is re-marshalled around them.
// That does not preserve the original bytes, which is fine here — signatures are
// verified by the ante handler on the write path, never by an RPC read.
func rewrite(txBytes []byte) ([]byte, bool) {
	var raw txtypes.Tx
	if err := raw.Unmarshal(txBytes); err != nil || raw.Body == nil {
		return nil, false
	}

	changed := false
	for _, msg := range raw.Body.Messages {
		if msg == nil || msg.TypeUrl != msgEthereumTxURL {
			continue
		}
		converted, err := convertMsg(msg.Value)
		if err != nil {
			continue
		}
		msg.Value = converted
		changed = true
	}
	if !changed {
		return nil, false
	}

	out, err := raw.Marshal()
	if err != nil {
		return nil, false
	}
	return out, true
}

// convertMsg rebuilds one MsgEthereumTx body from the legacy layout.
func convertMsg(body []byte) ([]byte, error) {
	var (
		inner    []byte
		innerURL string
		wantHash string
		from     []byte
	)

	err := walk(body, func(field int, wire int, val []byte, _ uint64) error {
		switch {
		case field == 1 && wire == 2: // Any data
			url, v, err := unpackAny(val)
			if err != nil {
				return err
			}
			innerURL, inner = url, v
		case field == 3 && wire == 2: // hash, hex string
			wantHash = string(val)
		case field == 5 && wire == 2: // from, already the current layout
			from = append([]byte(nil), val...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if inner == nil {
		return nil, fmt.Errorf("not a legacy MsgEthereumTx: no data field")
	}

	ethTx, err := buildEthTx(innerURL, inner)
	if err != nil {
		return nil, err
	}

	// Self-check: the legacy message carries the hash it was indexed under. If the
	// reconstruction does not reproduce it, the conversion is wrong and it is far
	// better to drop the transaction than to serve a fabricated one.
	if wantHash != "" {
		got := ethTx.Hash().Hex()
		if !bytes.EqualFold([]byte(got), []byte(wantHash)) {
			return nil, fmt.Errorf("hash mismatch: rebuilt %s, recorded %s", got, wantHash)
		}
	}

	msg := &evmtypes.MsgEthereumTx{}
	msg.FromEthereumTx(ethTx)
	msg.From = from
	return msg.Marshal()
}

// txConfig delegates everything to the wrapped config and only substitutes the
// decoder. Every read path in the EVM JSON-RPC backend goes through
// ClientCtx.TxConfig.TxDecoder(), so this one substitution covers
// eth_getBlockByNumber, eth_getTransactionByHash, eth_getTransactionReceipt,
// eth_getBlockReceipts and the tracers alike.
type txConfig struct {
	client.TxConfig
}

func (c txConfig) TxDecoder() sdk.TxDecoder {
	return Decoder(c.TxConfig.TxDecoder())
}

// WrapTxConfig returns cfg with a decoder that understands pre-v1.6.0 Ethereum
// transactions.
func WrapTxConfig(cfg client.TxConfig) client.TxConfig {
	return txConfig{TxConfig: cfg}
}
