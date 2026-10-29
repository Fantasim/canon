//go:build !linux

package gogen_test

import (
	"os"
	"testing"
)

// checkRSS skips: only Linux reports a peak resident set.
func checkRSS(t *testing.T, _ *os.ProcessState) {
	t.Helper()
	t.Skip("no peak resident set to read")
}
