package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/cli"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"golang.org/x/tools/txtar"
)

const (
	stepMark    = "step"   // "step<n>/<path>" is written to the project after the n-th summary
	cyclesFile  = "cycles" // the number of summaries, or unsettled lines, a case ends after, then a quiet wait
	quietWait   = 800 * time.Millisecond
	watchWait   = 20 * time.Second
	watchPoll   = 10 * time.Millisecond
	exitWatched = 130
)

// summaries counts the summary lines of a text or JSON run, the initial run's then each cycle's, and the line of a cycle that ends unsettled.
var summaries = regexp.MustCompile(`(?m)^(\d+ errors?, |\{"summary":|-- outputs did not settle)`)

// syncBuffer is an output the watch writes while the test reads it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// stepFiles are the files a case's step n writes, by path relative to the project.
func stepFiles(a *txtar.Archive, n int) map[string]string {
	prefix := stepMark + strconv.Itoa(n) + "/"
	out := map[string]string{}
	for _, f := range a.Files {
		if rest, ok := strings.CutPrefix(f.Name, prefix); ok {
			out[rest] = string(f.Data)
		}
	}
	return out
}

// isStep tells a case file that belongs to a step, not to the project at first.
func isStep(name string) bool { return strings.HasPrefix(name, stepMark) }

// writeProject writes the files of a case's project to dir; steps and control files are not.
func writeProject(t *testing.T, a *txtar.Archive, dir string) {
	t.Helper()
	for _, f := range a.Files {
		if control(f.Name) || isStep(f.Name) || f.Name == cyclesFile {
			continue
		}
		writeFile(t, dir, f.Name, f.Data)
	}
}

func writeFile(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// waitFor polls until out holds n summary lines.
func waitFor(t *testing.T, out *syncBuffer, n int) {
	t.Helper()
	deadline := time.Now().Add(watchWait)
	for time.Now().Before(deadline) {
		if len(summaries.FindAllString(out.String(), -1)) >= n {
			return
		}
		time.Sleep(watchPoll)
	}
	t.Fatalf("no summary number %d in %s", n, watchWait)
}

// watched runs a case's command line with --watch until its last step's cycle is printed,
// then interrupts it, and returns what it printed.
func watched(t *testing.T, a *txtar.Archive) string {
	t.Helper()
	tmp := t.TempDir()
	writeProject(t, a, tmp)
	text, _ := archived(a, argsFile)
	args := strings.Fields(strings.ReplaceAll(text, tmpMark, filepath.ToSlash(tmp)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, errs syncBuffer
	done := make(chan int, 1)
	go func() {
		done <- cli.Main(ctx, args, cli.Env{Stdout: &out, Stderr: &errs, Dir: tmp})
	}()
	waitFor(t, &out, 1)
	for n := 1; len(stepFiles(a, n)) > 0; n++ {
		for name, data := range stepFiles(a, n) {
			writeFile(t, tmp, name, []byte(data))
		}
		waitFor(t, &out, n+1)
	}
	if n, ok := archived(a, cyclesFile); ok {
		want, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			t.Fatal(err)
		}
		waitFor(t, &out, want)
		time.Sleep(quietWait) // no cycle follows the last one
	}
	cancel()
	var code int
	select {
	case code = <-done:
	case <-time.After(watchWait):
		t.Fatal("the watch did not end after the interrupt")
	}
	return normalize(fmt.Sprintf("[exit %d]\n[stdout]\n%s[stderr]\n%s", code, out.String(), errs.String()), tmp)
}

// CLI.md §3.3, §3.4, §2.5, IMPLEMENTATION-PLAN.md §8.1, API.md W12, W13: each case runs --watch, applies its `step<n>/<path>` files one by one, waits for each cycle, interrupts (exit 130).
func TestWatch(t *testing.T) {
	golden.Run(t, "testdata/watch/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		return []byte(watched(t, c.Archive))
	})
}

// CLI.md §2.5: an interrupt with no change ends --watch with exit 130 after the first run alone.
func TestWatchInterrupted(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "project.canon", []byte("project a {\n  canon: \"0.1\"\n}\n"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, errs syncBuffer
	done := make(chan int, 1)
	go func() {
		done <- cli.Main(ctx, []string{"check", "--watch"}, cli.Env{Stdout: &out, Stderr: &errs, Dir: dir})
	}()
	waitFor(t, &out, 1)
	cancel()
	code := <-done
	if code != exitWatched || strings.Contains(out.String(), "files changed") || !strings.Contains(errs.String(), "canon: interrupted") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out.String(), errs.String())
	}
}
