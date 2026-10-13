package writelock

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"
)

// A helper process is this test binary run with helperRun, then argSep and its role: holdRole
// holds one package exclusively until its standard input closes; cycleRole takes its packages,
// in the order given, cycles times.
const (
	helperRun = "-test.run=^TestHelperProcess$"
	argSep    = "--"
	holdRole  = "hold"
	cycleRole = "cycle"
	heldMark  = "held"
	patience  = 30 * time.Second
	exitFail  = 2
)

// helperArgs are the arguments after argSep when this binary runs as a helper, nil otherwise.
func helperArgs() []string {
	i := slices.Index(os.Args, argSep)
	if i < 0 {
		return nil
	}
	return os.Args[i+1:]
}

// TestHelperProcess is a lock holder in a process of its own for the tests below; run any
// other way it does nothing.
func TestHelperProcess(t *testing.T) {
	args := helperArgs()
	if len(args) < 3 {
		t.Skip("a helper process only")
	}
	os.Exit(helper(args[0], args[1], args[2:]))
}

// helper plays role in the project at dir over pkgs; its exit code is 0 when all went well.
func helper(role, dir string, pkgs []string) int {
	reqs := make([]Request, len(pkgs))
	for i, p := range pkgs {
		reqs[i] = ex(p)
	}
	n := cycles
	if role == holdRole {
		n = 1
	}
	for range n {
		h, err := Acquire(context.Background(), dir, reqs)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitFail
		}
		if role == holdRole {
			fmt.Println(heldMark)
			_, _ = io.Copy(io.Discard, os.Stdin) // held until the test closes stdin or kills this process
		}
		if h.Release() != nil {
			return exitFail
		}
	}
	return 0
}

// startHelper runs a helper process; for holdRole it returns once the lock is held.
func startHelper(t *testing.T, role, dir string, pkgs ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{helperRun, argSep, role, dir}, pkgs...)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // the test waits for the process itself
		_ = stdin.Close()
		_ = cmd.Process.Kill()
	})
	if role == holdRole {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil || line != heldMark+lineEnd {
			t.Fatalf("helper: %q, %v", line, err)
		}
	}
	return cmd
}

// DECISIONS 352: another process's lock excludes this one's; killing the holder releases it.
func TestKilledHolderReleases(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	cmd := startHelper(t, holdRole, dir, pkgA)
	if err := within(dir, held, sh(pkgA)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%sheld by another process: %v", rule, err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait() // killed: its error says so
	if err := within(dir, patience, ex(pkgA)); err != nil {
		t.Errorf("%safter the holder was killed: %v", rule, err)
	}
}

// DECISIONS 352: processes locking overlapping sets given in opposite orders never deadlock.
func TestOppositeOrdersAcrossProcesses(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	cmds := []*exec.Cmd{
		startHelper(t, cycleRole, dir, pkgA, pkgB),
		startHelper(t, cycleRole, dir, pkgB, pkgA),
		startHelper(t, cycleRole, dir, pkgB),
	}
	done := make(chan error, len(cmds))
	for _, cmd := range cmds {
		go func() { done <- cmd.Wait() }()
	}
	deadline := time.After(patience)
	for range cmds {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("%s%v", rule, err)
			}
		case <-deadline:
			t.Fatalf("%sthe helpers did not finish in %v", rule, patience)
		}
	}
}
