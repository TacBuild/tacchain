package legacyeth

import (
	"testing"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

func mustAny(t *testing.T, url string, value []byte) *codectypes.Any {
	t.Helper()
	return &codectypes.Any{TypeUrl: url, Value: value}
}
