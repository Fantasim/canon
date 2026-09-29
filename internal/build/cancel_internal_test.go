package build

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"golang.org/x/tools/txtar"
)

const (
	cancelCase = "testdata/incremental/cancel.txtar"
	cutPoints  = 200

	cancelPkg    = "a"
	cancelRoot   = "heavy"
	cancelSource = "a/a.canon"
	cancelJSON   = "size.json"
)

// cutCtx is a context whose Err turns Canceled after a number of calls, its Done closing then.
type cutCtx struct {
	context.Context
	mu    sync.Mutex
	left  int
	calls int
	done  chan struct{}
}

func newCut(left int) *cutCtx {
	return &cutCtx{Context: context.Background(), left: left, done: make(chan struct{})}
}

func (c *cutCtx) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.left > 0 {
		c.left--
		return nil
	}
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	return context.Canceled
}

func (c *cutCtx) Done() <-chan struct{} { return c.done }

// sweepRun is one way to run the cancel case: its outcome as text, and its error.
type sweepRun func(ctx context.Context, z *analyzer) (string, error)

// IMPLEMENTATION-PLAN §4.8, §8.5 (log-2026-09-29 M4 B2-r): cancelled anywhere, a run completes or reports the interrupt.
func TestCancelSweep(t *testing.T) {
	for _, c := range []struct {
		name, must string // must: what the whole run's outcome holds
		run        sweepRun
	}{
		{"check", "", checkOutcome}, {"build", "", buildOutcome}, {"test", "", testOutcome},
		{"analyze and cause", "", causeOutcome}, {"json sources", cancelJSON, jsonOn(osCopy(t, cancelCase))},
	} {
		run := c.run
		t.Run(c.name, func(t *testing.T) {
			whole := newCut(math.MaxInt)
			want, err := run(whole, archiveAnalyzer(t, cancelCase))
			if err != nil || !strings.Contains(want, c.must) {
				t.Fatalf("%v: %s", err, want)
			}
			t.Logf("%d context checks", whole.calls)
			for k := 0; k <= whole.calls; k += max(1, whole.calls/cutPoints) {
				got, err := cutRun(run, newCut(k), archiveAnalyzer(t, cancelCase))
				if err != nil && !errors.Is(err, context.Canceled) || errors.Is(err, ErrInternal) {
					t.Errorf("cut after %d of %d: %v", k, whole.calls, err)
				}
				if err == nil && got != want {
					t.Errorf("cut after %d of %d: an incomplete result without error:\n%s", k, whole.calls, got)
				}
			}
		})
	}
}

// cutRun is run under ctx, a panic reported as an error.
func cutRun(run sweepRun, ctx context.Context, z *analyzer) (out string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	return run(ctx, z)
}

// checkOutcome is a cold check's findings.
func checkOutcome(ctx context.Context, z *analyzer) (string, error) {
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		return "", err
	}
	res, err := p.Check(ctx, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(res.List, res.Summary), nil
}

// buildOutcome is a cold build's findings and outputs, writing nothing.
func buildOutcome(ctx context.Context, z *analyzer) (string, error) {
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		return "", err
	}
	res, err := p.Build(ctx, BuildOptions{Check: true})
	if err != nil {
		return "", err
	}
	out := fmt.Sprint(res.List, res.Summary, res.Stale)
	for _, o := range res.Outputs {
		out += fmt.Sprint(o.Path, o.Status, len(o.Content))
	}
	return out, nil
}

// testOutcome is canon test's static errors and cases.
func testOutcome(ctx context.Context, z *analyzer) (string, error) {
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		return "", err
	}
	res, err := p.Test(ctx, nil, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(res.Static.List, res.Tests), nil
}

// causeOutcome is a memoized analysis's findings, then what its cause re-run gives a root.
func causeOutcome(ctx context.Context, z *analyzer) (string, error) {
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		return "", err
	}
	a, err := p.WithCache(z.cache).Analyze(ctx, nil)
	if err != nil {
		return "", err
	}
	causes, err := a.Cause(ctx, eval.Root{Pkg: cancelPkg, Name: cancelRoot})
	if err != nil {
		return "", err
	}
	return fmt.Sprint(a.Result().List, causes, a.r.memoized), nil
}

// jsonOn is the JSON sources the loads of the case copied to dir read, with their numbers; on
// the OS's file system, which resolves them.
func jsonOn(dir string) sweepRun {
	return func(ctx context.Context, _ *analyzer) (string, error) {
		p, err := Open(project.OS(), dir, Options{})
		if err != nil {
			return "", err
		}
		srcs, err := p.JSONSources(ctx, []string{cancelSource})
		if err != nil {
			return "", err
		}
		out := fmt.Sprint(len(srcs))
		for _, s := range srcs {
			out += fmt.Sprint(s.Display, s.Numbers)
		}
		return out, nil
	}
}

// osCopy writes the archive's files under a new directory and returns it, slash-separated.
func osCopy(t *testing.T, file string) string {
	t.Helper()
	a, err := txtar.ParseFile(file)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, f := range a.Files {
		name := filepath.Join(dir, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(name), dirMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, f.Data, fileMode); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.ToSlash(dir)
}
