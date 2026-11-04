package cxx

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/fantasim/canonlang/internal/testkit/cxx/config"
)

// slots is the process-wide semaphore: one token per compiler or go tool process running.
var slots = sync.OnceValue(func() chan struct{} { return make(chan struct{}, slotCount()) })

// slotCount is config.Slots' override when set, else one slot per cpusPerSlot CPUs, from one to maxSlots.
func slotCount() int {
	if n, ok := config.Slots(); ok {
		return n
	}
	return min(maxSlots, max(1, runtime.NumCPU()/cpusPerSlot))
}

// Hold takes a slot, blocking until one is free, and returns the function that gives it back:
// `defer Hold()()` holds one for the rest of the caller, a panic included.
func Hold() (release func()) {
	s := slots()
	s <- struct{}{}
	return func() { <-s }
}

// Combined runs cmd and returns its stdout and stderr together, once a slot is free.
func Combined(cmd *exec.Cmd) ([]byte, error) {
	defer Hold()()
	out, err := cmd.CombinedOutput()
	return out, wrap(cmd, err)
}

// Output runs cmd and returns its stdout alone, once a slot is free.
func Output(cmd *exec.Cmd) ([]byte, error) {
	defer Hold()()
	out, err := cmd.Output()
	return out, wrap(cmd, err)
}

// Compile is Combined with a Timeout kill that starts once the slot is held, not in the queue
// as an exec.CommandContext made before the call would. Like CombinedOutput, it fails when
// cmd already has a Stdout or a Stderr.
func Compile(cmd *exec.Cmd) ([]byte, error) {
	if cmd.Stdout != nil || cmd.Stderr != nil {
		return nil, wrap(cmd, errOutputSet)
	}
	defer Hold()()
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return nil, wrap(cmd, err)
	}
	timer := time.AfterFunc(Timeout, func() { _ = cmd.Process.Kill() })
	defer timer.Stop()
	err := cmd.Wait()
	return out.Bytes(), wrap(cmd, err)
}

// wrap names the command in err, which stays reachable through errors.Is and errors.As; nil stays nil.
func wrap(cmd *exec.Cmd, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", cmd.Path, err)
}
