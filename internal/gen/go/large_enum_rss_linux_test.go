//go:build linux

package gogen_test

import (
	"os"
	"syscall"
	"testing"
)

// checkRSS fails when the compile of the generated packages, the largest process the build waited for, peaked above maxRSSKB (Linux reports KB).
func checkRSS(t *testing.T, state *os.ProcessState) {
	t.Helper()
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok {
		t.Skip("no peak resident set to read")
	}
	t.Logf("go build peak resident set: %d KB", usage.Maxrss)
	if usage.Maxrss > maxRSSKB {
		t.Errorf("go build peaked at %d KB, over %d KB", usage.Maxrss, maxRSSKB)
	}
}
