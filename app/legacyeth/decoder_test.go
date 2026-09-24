package legacyeth

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// The fixture is the transaction from mainnet block 12,489,259 that surfaced the
// bug: a Safe execTransaction written under the pre-v1.6.0 layout, which the
// current decoder rejects on field 1.
func TestRewriteLegacyMainnetTx(t *testing.T) {
	raw, err := os.ReadFile("testdata/legacy_tx_12489259.b64")
	require.NoError(t, err)
	txBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	require.NoError(t, err)

	out, ok := rewrite(txBytes)
	require.True(t, ok, "legacy message not recognised")

	var decoded txtypes.Tx
	require.NoError(t, decoded.Unmarshal(out))
	require.Len(t, decoded.Body.Messages, 1)

	var msg evmtypes.MsgEthereumTx
	require.NoError(t, msg.Unmarshal(decoded.Body.Messages[0].Value))
	require.NotNil(t, msg.Raw.Transaction)

	// convertMsg drops any reconstruction whose hash does not match the one the
	// legacy message recorded; assert the value explicitly so a weakened
	// self-check cannot pass unnoticed.
	require.Equal(t,
		"0x84380b5ebf3d0a1fd00fb99d81e423cc4a9207572aafff12e4c3c2c394b83b4e",
		msg.Raw.Hash().Hex())
	require.Equal(t, uint64(1), msg.Raw.Nonce())
	require.Equal(t, uint64(172644), msg.Raw.Gas())
	require.Equal(t, "0x88B577E8eB8a0BEFF49eb4fAB2a21210Af35264B", msg.Raw.To().Hex())
}

// A current-layout transaction must not be touched: rewrite only fires when the
// standard decoder has already failed, and it must decline anything it does not
// recognise rather than guess.
func TestRewriteIgnoresCurrentLayout(t *testing.T) {
	msg := &evmtypes.MsgEthereumTx{From: []byte{0x01}}
	body, err := msg.Marshal()
	require.NoError(t, err)

	tx := txtypes.Tx{Body: &txtypes.TxBody{}}
	tx.Body.Messages = append(tx.Body.Messages, mustAny(t, msgEthereumTxURL, body))
	bz, err := tx.Marshal()
	require.NoError(t, err)

	_, ok := rewrite(bz)
	require.False(t, ok, "a current-layout message must be left alone")
}

func TestRewriteRejectsGarbage(t *testing.T) {
	_, ok := rewrite([]byte{0xff, 0xfe, 0xfd})
	require.False(t, ok)
}
