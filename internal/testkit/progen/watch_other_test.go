//go:build !unix

package progen_test

import (
	"os/exec"
	"time"
)

// stopHung has no goroutine samples to ask for here: SIGUSR1 and SIGQUIT are unix-only signals,
// so this platform kills the hung child directly and returns once it has ended; the report's
// stuckFunctions then names none, same as an incomplete sample would.
func stopHung(cmd *exec.Cmd, exited <-chan struct{}, limit time.Duration) {
	_ = cmd.Process.Kill()
	select {
	case <-exited:
	case <-time.After(limit):
	}
}

// answerSamples is a no-op here: the SIGUSR1 goroutine sampler it would install is unix-only.
func answerSamples() {}
