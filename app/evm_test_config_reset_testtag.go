//go:build test

package app

import (
	"testing"

	evmvmtypes "github.com/cosmos/evm/x/vm/types"
)

func resetEVMTestConfig(t *testing.T, evmChainID uint64) {
	t.Helper()
	configurator := evmvmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	if err := evmvmtypes.SetChainConfig(evmvmtypes.DefaultChainConfig(evmChainID)); err != nil {
		t.Fatalf("reset EVM test chain config: %v", err)
	}
}
