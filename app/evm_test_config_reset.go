//go:build !test

package app

import "testing"

func resetEVMTestConfig(t *testing.T, _ uint64) {
	t.Helper()
}
