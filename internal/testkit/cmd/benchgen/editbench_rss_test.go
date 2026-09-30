//go:build linux

package main

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// rssHelperEnv, when set, makes the test binary the RSS helper: it runs its arguments as a
// command and writes its own peak resident set in KiB and its wall time in ns to the file named.
const rssHelperEnv = "CANON_BENCHGEN_RSS_FILE"

// The helper's report: its file's mode, and the base and width of the number it holds.
const (
	rssFilePerm = 0o600
	decimal     = 10
	rssBits     = 64
	rssSep      = " "
)

// TestMain runs the tests, or, in the RSS helper, the one command it measures.
func TestMain(m *testing.M) {
	if out := os.Getenv(rssHelperEnv); out != "" {
		os.Exit(rssHelper(out, os.Args[1:]))
	}
	os.Exit(m.Run())
}

// rssHelper runs args and writes their peak RSS and wall time to out. Linux's ru_maxrss keeps the
// peak of the memory a process replaced at exec: started by the test process it would report the
// test process's, several GB with the benchmark open; started from this small one, only this one's.
func rssHelper(out string, args []string) int {
	if len(args) == 0 {
		return exitUsage
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	start := time.Now()
	err := cmd.Run()
	wall := time.Since(start)
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return exitFail
	}
	usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		return exitFail
	}
	report := strconv.FormatInt(usage.Maxrss, decimal) + rssSep + strconv.FormatInt(int64(wall), decimal)
	if os.WriteFile(out, []byte(report), rssFilePerm) != nil {
		return exitFail
	}
	return cmd.ProcessState.ExitCode()
}

// The RSS helper's own test: the test process holds rssHeld bytes, a command started through
// the helper reports less than rssSmall.
const (
	rssHeld  = 192 << 20
	rssSmall = 96 << 20
	rssPage  = 4096
	rssTrue  = "true"
)

// NFR-01 peak RSS (log-2026-09-29 M4 P14): a measure reports the command's own, never the test's.
func TestMeasuredRSSIsOwn(t *testing.T) {
	held := make([]byte, rssHeld)
	for i := 0; i < len(held); i += rssPage {
		held[i] = 1
	}
	path, err := exec.LookPath(rssTrue)
	if err != nil {
		t.Skip("no `true` command")
	}
	_, m, err := measured(t, exec.Command(path))
	if err != nil || m.rss <= 0 || m.rss >= rssSmall || m.wall <= 0 {
		t.Errorf("`true` peaked at %d bytes in %v (%v) while the test holds %d", m.rss, m.wall, err, len(held))
	}
	held[len(held)-1] = 1
}

// measurement is a command's own peak resident set in bytes and its wall time.
type measurement struct {
	rss  int64
	wall time.Duration
}

// measured is cmd run through the RSS helper: its combined output and its measurement.
func measured(t *testing.T, cmd *exec.Cmd) ([]byte, measurement, error) {
	t.Helper()
	report := t.TempDir() + "/rss"
	helper := exec.Command(os.Args[0], cmd.Args...)
	helper.Dir = cmd.Dir
	helper.Env = append(os.Environ(), rssHelperEnv+"="+report)
	out, err := helper.CombinedOutput()
	if err != nil {
		return out, measurement{}, err
	}
	data, rerr := os.ReadFile(report)
	if rerr != nil {
		t.Fatalf("the RSS helper wrote no report: %v\n%s", rerr, out)
	}
	kib, ns, _ := strings.Cut(strings.TrimSpace(string(data)), rssSep)
	rss, rerr := strconv.ParseInt(kib, decimal, rssBits)
	wall, werr := strconv.ParseInt(ns, decimal, rssBits)
	if rerr != nil || werr != nil {
		t.Fatalf("the RSS helper's report %q: %v %v", data, rerr, werr)
	}
	return out, measurement{rss: rss * rssUnit, wall: time.Duration(wall)}, nil
}
