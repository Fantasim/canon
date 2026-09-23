package main

import (
	"strings"
	"testing"
)

// CLI.md exit codes: a command canon does not know is a usage error, exit 2.
func TestEveryInvocationIsAUsageErrorBeforeM1(t *testing.T) {
	var stderr strings.Builder
	if code := run(&stderr); code != exitUsage || stderr.String() != noCommand {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
}
