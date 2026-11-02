//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// rssHelperEnv, when set, makes the test binary the RSS helper: it runs its arguments as a
// command and writes its own peak resident set in KiB, its wall time in ns and the signal that
// killed it (0 for none) to the file named.
const rssHelperEnv = "CANON_BENCHGEN_RSS_FILE"

// The helper's report: its file's mode, and the base and width of the number it holds.
const (
	rssFilePerm  = 0o600
	decimal      = 10
	rssBits      = 64
	rssSep       = " "
	rssFields    = 3 // KiB, ns, signal
	exitSignaled = 3 // the helper's exit when its command was killed by a signal
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
	var sig syscall.Signal
	if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		sig = ws.Signal()
	}
	report := strings.Join([]string{strconv.FormatInt(usage.Maxrss, decimal), strconv.FormatInt(int64(wall), decimal), strconv.Itoa(int(sig))}, rssSep)
	if os.WriteFile(out, []byte(report), rssFilePerm) != nil {
		return exitFail
	}
	if sig != 0 {
		return exitSignaled
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

// DECISIONS 441: a command killed by a signal is reported with its signal (the scale bench records
// an OOM kill), not as an anonymous failure; one that exits non-zero keeps no measurement.
func TestMeasuredSignal(t *testing.T) {
	_, m, err := measured(t, exec.Command("sh", "-c", "kill -9 $$"))
	if !errors.Is(err, errSignaled) || m.signal != syscall.SIGKILL || m.wall <= 0 || m.rss <= 0 {
		t.Errorf("a killed command: %+v, %v; want SIGKILL, a wall time, a peak and %v", m, err, errSignaled)
	}
	_, m, err = measured(t, exec.Command("sh", "-c", "exit 3"))
	if err == nil || errors.Is(err, errSignaled) || m != (measurement{}) {
		t.Errorf("a failing command: %+v, %v; want no measurement and an error that is not %v", m, err, errSignaled)
	}
}

// The measuring harness's errors: a command killed by a signal (an OOM kill), a report that is
// not three numbers.
var (
	errSignaled  = errors.New("the command was killed by a signal")
	errBadReport = errors.New("the RSS helper's report is malformed")
)

// measurement is a command's own peak resident set in bytes, its wall time and, when a signal
// killed it, that signal.
type measurement struct {
	rss    int64
	wall   time.Duration
	signal syscall.Signal
}

// measured is cmd run through the RSS helper: its combined output and its measurement. A command
// killed by a signal is returned with its measurement and an error wrapping errSignaled; one that
// exits non-zero has none (DECISIONS 441).
func measured(t *testing.T, cmd *exec.Cmd) ([]byte, measurement, error) {
	t.Helper()
	report := t.TempDir() + "/rss"
	helper := exec.Command(os.Args[0], cmd.Args...)
	helper.Dir = cmd.Dir
	helper.Env = append(os.Environ(), rssHelperEnv+"="+report)
	out, err := helper.CombinedOutput()
	m, rerr := readReport(report)
	switch {
	case err == nil && rerr != nil:
		t.Fatalf("the RSS helper's report: %v\n%s", rerr, out)
	case err == nil:
		return out, m, nil
	case rerr == nil && m.signal != 0:
		return out, m, fmt.Errorf("%v: %w", m.signal, errSignaled)
	}
	return out, measurement{}, err
}

// readReport is the measurement the RSS helper wrote to path.
func readReport(path string) (measurement, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return measurement{}, err
	}
	f := strings.Fields(string(data))
	if len(f) != rssFields {
		return measurement{}, fmt.Errorf("the RSS helper's report %q: %w", data, errBadReport)
	}
	kib, rerr := strconv.ParseInt(f[0], decimal, rssBits)
	ns, werr := strconv.ParseInt(f[1], decimal, rssBits)
	sig, serr := strconv.Atoi(f[2])
	if err := errors.Join(rerr, werr, serr); err != nil {
		return measurement{}, fmt.Errorf("the RSS helper's report %q: %w", data, err)
	}
	return measurement{rss: kib * rssUnit, wall: time.Duration(ns), signal: syscall.Signal(sig)}, nil
}
