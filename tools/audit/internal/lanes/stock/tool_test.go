package stock

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testDeadline = 200 * time.Millisecond
	testMargin   = 2 * time.Second
)

// TestRunToolRespectsDeadline proves runTool kills a child that outlives ctx instead of
// waiting for it, and says so: golangci-lint's own --allow-serial-runners lock wait is
// unbounded, so this is what bounds it (log-2026-09-24, audit gate review calls).
func TestRunToolRespectsDeadline(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep binary on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	start := time.Now()
	_, err = runTool(ctx, sleep, t.TempDir(), nil, "30")
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), msgKilled) {
		t.Fatalf("runTool = %v, want a %v error naming %s", err, context.DeadlineExceeded, msgKilled)
	}
	if elapsed > testDeadline+testMargin {
		t.Errorf("runTool took %s, want it bounded near the %s deadline", elapsed, testDeadline)
	}
}

// TestRunToolChildSeesAuditCache proves the child reads the audit's own GOLANGCI_LINT_CACHE
// even when the caller set another (log-2026-09-24: a caller's cache is ignored).
func TestRunToolChildSeesAuditCache(t *testing.T) {
	envBin, err := exec.LookPath("env")
	if err != nil {
		t.Skip("no env binary on PATH")
	}
	t.Setenv(golangciCacheEnv, "caller-set")
	dir := filepath.Join(t.TempDir(), "cache")
	extra, err := golangciExtraEnv(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runTool(context.Background(), envBin, t.TempDir(), extra)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	for l := range strings.SplitSeq(string(out), "\n") {
		if strings.HasPrefix(l, golangciCacheEnv+"=") {
			seen = append(seen, l)
		}
	}
	if want := golangciCacheEnv + "=" + dir; len(seen) != 1 || seen[0] != want {
		t.Fatalf("child env has %v, want exactly [%s]", seen, want)
	}
}
