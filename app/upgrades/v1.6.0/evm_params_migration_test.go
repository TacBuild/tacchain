package v160

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestParseLegacyERC20PrecompileAddress(t *testing.T) {
	valid := "0x1111111111111111111111111111111111111111"
	addr, err := parseLegacyERC20PrecompileAddress("native", valid, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != common.HexToAddress(valid) {
		t.Fatalf("address mismatch: got %s, want %s", addr.Hex(), common.HexToAddress(valid).Hex())
	}

	cases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "malformed",
			input:   "0x" + strings.Repeat("0", 39) + "g",
			wantErr: "invalid native precompile address",
		},
		{
			name:    "zero",
			input:   "0x" + strings.Repeat("0", 40),
			wantErr: "zero address",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseLegacyERC20PrecompileAddress("native", tc.input, 42)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestParseLegacyERC20PrecompileBlob(t *testing.T) {
	addr1 := "0x1111111111111111111111111111111111111111"
	addr2 := "0x2222222222222222222222222222222222222222"

	seen := make(map[common.Address]string)
	addrs, err := parseLegacyERC20PrecompileBlob("native", []byte(addr1+addr2), seen)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(addrs) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(addrs))
	}

	_, err = parseLegacyERC20PrecompileBlob("dynamic", []byte(addr1), seen)
	if err == nil {
		t.Fatalf("expected duplicate error")
	}
	if !strings.Contains(err.Error(), "duplicate dynamic precompile address") {
		t.Fatalf("unexpected duplicate error: %v", err)
	}
}

func TestParseLegacyERC20PrecompileBlobRejectsBadLength(t *testing.T) {
	_, err := parseLegacyERC20PrecompileBlob("native", []byte("0x123"), make(map[common.Address]string))
	if err == nil {
		t.Fatalf("expected length error")
	}
	if !strings.Contains(err.Error(), "is not a multiple of") {
		t.Fatalf("unexpected error: %v", err)
	}
}
