package stock

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

// golangci-lint takes this lock with gofrs/flock, flock(2) LOCK_EX on $TMPDIR/<name>
// (pkg/commands/run.go, acquireFileLock); lockTestDeadline stands in for lintDeadline.
const (
	golangciLockFile = "golangci-lint.lock"
	argNoConfig      = "--no-config"
	lockTestDeadline = 3 * time.Second
)

// lockedModule is a one-package module in a t.TempDir, while this test holds golangci-lint's
// own lock for its whole run, so any golangci-lint started here waits on it.
func lockedModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"go.mod": "module locked\n\ngo 1.25\n", "x.go": "package locked\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), testFilePerm); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TMPDIR", t.TempDir())
	f, err := os.OpenFile(filepath.Join(os.TempDir(), golangciLockFile), os.O_CREATE|os.O_RDWR, testFilePerm)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("hold %s: %v", f.Name(), err)
	}
	return dir
}

// killedAtDeadline runs run (its failure text, "" on success) against a held lock and proves
// it was stopped by the deadline itself: not before it (the child really waited on the
// lock), not long after it (the wait is bounded), and saying so.
func killedAtDeadline(t *testing.T, run func() string) {
	t.Helper()
	done := make(chan string, 1)
	start := time.Now()
	go func() { done <- run() }()
	select {
	case reason := <-done:
		elapsed := time.Since(start)
		if elapsed < lockTestDeadline || elapsed > lockTestDeadline+testMargin {
			t.Fatalf("returned after %s, want the %s deadline to stop it: %q", elapsed, lockTestDeadline, reason)
		}
		if !strings.Contains(reason, context.DeadlineExceeded.Error()+" "+msgKilled) {
			t.Fatalf("failure %q, want it to name the deadline %s", reason, msgKilled)
		}
	case <-time.After(lockTestDeadline + testMargin):
		t.Fatalf("still waiting on the lock %s past the %s deadline", testMargin, lockTestDeadline)
	}
}

// golangciBin is the pinned golangci-lint; these tests run in the gate whenever it resolves
// (the audit's own stock lane needs it there too).
func golangciBin(t *testing.T) string {
	t.Helper()
	bin, err := resolveTool(toolchainDir(t), toolGolangci)
	if err != nil {
		t.Skipf("pinned golangci-lint does not resolve: %v", err)
	}
	return bin
}

// TestGolangciLockDeadline proves runTool's deadline covers golangci-lint's lock wait
// (log-2026-09-24, audit gate review calls: a hard deadline covering the lock wait).
func TestGolangciLockDeadline(t *testing.T) {
	bin := golangciBin(t)
	dir := lockedModule(t)
	killedAtDeadline(t, func() string {
		ctx, cancel := context.WithTimeout(context.Background(), lockTestDeadline)
		defer cancel()
		_, err := runTool(ctx, bin, dir, nil, argRun, argNoConfig, argAllowSerialRunners, argOutputJSONPath, valStdout, targetAll)
		if err == nil {
			return ""
		}
		return err.Error()
	})
}

// TestRunGolangciLockDeadline proves the lane itself kills golangci-lint at the deadline it
// is given and reports the lane unmeasured (a Skip, so check fails) rather than hanging.
func TestRunGolangciLockDeadline(t *testing.T) {
	golangciBin(t)
	ctx := realContext(t, lockedModule(t), toolchainDir(t))
	ctx.Enabled = map[string]bool{ruleFnComplexity: true}
	killedAtDeadline(t, func() string {
		if _, skip := runGolangci(ctx, lockTestDeadline); skip != nil {
			return skip.Reason
		}
		return ""
	})
}

// realContext is the lane context main would build for root: the real thresholds, and a
// cache base of the test's own.
func realContext(t *testing.T, root, toolchain string) *lane.Context {
	t.Helper()
	r, err := repo.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	limits, err := threshold.Load(filepath.Join(toolchain, "..", threshold.FileName))
	if err != nil {
		t.Fatal(err)
	}
	return &lane.Context{Repo: r, Toolchain: toolchain, Limits: limits, CacheBase: t.TempDir()}
}
