//go:build unix

package progen_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime/pprof"
	"slices"
	"sync"
	"syscall"
	"time"
)

// stopHung asks a silent child for hangSamples goroutine samples (SIGUSR1), then for its dump
// (SIGQUIT), and kills it if it still runs a limit later; it returns once the child ended.
func stopHung(cmd *exec.Cmd, exited <-chan struct{}, limit time.Duration) {
	signals := append(slices.Repeat([]os.Signal{syscall.SIGUSR1}, hangSamples), syscall.SIGQUIT)
	for _, sig := range signals {
		if cmd.Process.Signal(sig) != nil {
			return
		}
		select {
		case <-exited:
			return
		case <-time.After(sampleEvery):
		}
	}
	select {
	case <-exited:
	case <-time.After(limit):
		_ = cmd.Process.Kill()
	}
}

var samplesOnce sync.Once

// answerSamples makes this child print a goroutine sample, between a sampleMark and a sampleEnd
// line, at each SIGUSR1 of its supervisor. Every child calls it first.
func answerSamples() {
	samplesOnce.Do(func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGUSR1)
		go func() {
			for range ch {
				var b bytes.Buffer
				_ = pprof.Lookup(goroutineProfile).WriteTo(&b, goroutineDump)
				fmt.Fprintf(os.Stderr, "%s\n%s%s\n", sampleMark, b.Bytes(), sampleEnd)
			}
		}()
	})
}
