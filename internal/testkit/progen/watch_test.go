package progen_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime/pprof"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// flagChildTimeout bounds how long a child may go without printing, that is, spend on one case:
// past it the child is stopped and its case reported as a hang, named as the decision log's
// "M1.5 round 2 calls" says. flagReplayTimeout is the same bound for one replayed archive.
var (
	flagChildTimeout  = flag.Duration("progen.childtimeout", childTimeout, "longest a child may spend on one case")
	flagReplayTimeout = flag.Duration("progen.replaytimeout", replayTimeout, "longest the replay of one kept counterexample may take")
)

// shrinkTried counts the shrink candidates this process tried, and lastBeat is when heartbeat
// last printed (Unix nanoseconds).
var shrinkTried, lastBeat atomic.Int64

// heartbeat, after each shrink candidate of a child, prints a line when the watcher's check
// interval passed since the last one: a shrink shows progress at every check, however slow its
// candidates, so only a case that itself outlasts the limit reads as a hang.
func heartbeat() {
	n := shrinkTried.Add(1)
	now := time.Now().UnixNano()
	if !*flagChild || now-lastBeat.Load() < int64(*flagChildTimeout/watchChecks) {
		return
	}
	lastBeat.Store(now)
	fmt.Fprintf(os.Stderr, "progen: shrinking, %d candidates tried\n", n)
}

// watched is a child's output, and when it last wrote.
type watched struct {
	mu   sync.Mutex
	out  bytes.Buffer
	last time.Time
}

func (w *watched) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.last = time.Now()
	return w.out.Write(p)
}

// idle is how long the child has printed nothing.
func (w *watched) idle() time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	return time.Since(w.last)
}

// runWatched runs cmd; once it has printed nothing for limit, hung, it is sampled and stopped
// (stopHung).
func runWatched(cmd *exec.Cmd, limit time.Duration) (out []byte, hung bool, err error) {
	w := &watched{last: time.Now()}
	cmd.Stdout, cmd.Stderr = w, w
	cmd.WaitDelay = waitDelay
	if err := cmd.Start(); err != nil {
		return nil, false, fmt.Errorf("start child: %w", err)
	}
	done, exited, stopped := make(chan error, 1), make(chan struct{}), make(chan struct{})
	go func() {
		err := cmd.Wait()
		close(exited)
		done <- err
	}()
	tick := time.NewTicker(max(limit/watchChecks, time.Millisecond))
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			if hung {
				<-stopped
			}
			w.mu.Lock()
			defer w.mu.Unlock()
			return bytes.Clone(w.out.Bytes()), hung, err
		case <-tick.C:
			if !hung && w.idle() > limit {
				hung = true
				go func() { stopHung(cmd, exited, limit); close(stopped) }()
			}
		}
	}
}

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

// hangVerdict is a child stopped for going silent past limit, named by the compiler functions
// its stuck goroutine never left (stuckFunctions); "hang" alone when none is the compiler's.
func hangVerdict(out []byte, limit time.Duration) verdict {
	text := fmt.Sprintf("%s: no progress for %v", kindHang, limit)
	sig := kindHang
	if names := stuckFunctions(string(out)); len(names) > 0 {
		sig += " in " + strings.Join(names, cycleSep)
	}
	return verdict{Kind: kindHang, Sig: sig, Text: text}
}

// stuckFunctions are the compiler functions the stuck goroutine never left, sorted: the frames
// every complete sample shares from the root down, so that a helper the loop reaches by two
// paths never names it (decision log "Owed (M1.5)"); with no complete sample, its SIGQUIT dump's.
func stuckFunctions(out string) []string {
	dumps := samples(out)
	if _, quit, ok := strings.Cut(out, quitMark); ok && len(dumps) == 0 {
		dumps = []string{quit}
	}
	var common []string
	elided := false
	for i, d := range dumps {
		g := stuckGoroutine(d)
		elided = elided || strings.Contains(g, "\n"+elidedMark)
		names := frameList(g, len(d))
		slices.Reverse(names)
		if i == 0 {
			common = names
			continue
		}
		common = sharedFrames(common, names, elided)
	}
	slices.Sort(common)
	return slices.Compact(common)
}

// sharedFrames are the frames of common, root first, that names shares: its prefix, or, once a
// stack was elided and has no root to align on, every frame names holds.
func sharedFrames(common, names []string, elided bool) []string {
	if elided {
		return slices.DeleteFunc(common, func(n string) bool { return !slices.Contains(names, n) })
	}
	n := 0
	for n < min(len(common), len(names)) && common[n] == names[n] {
		n++
	}
	return common[:n]
}

// samples are the complete goroutine samples of a child's output, in order.
func samples(out string) []string {
	var dumps []string
	for {
		_, rest, ok := strings.Cut(out, sampleMark+"\n")
		if !ok {
			return dumps
		}
		dump, after, complete := strings.Cut(rest, sampleEnd)
		if !complete {
			return dumps
		}
		dumps, out = append(dumps, dump), after
	}
}

// stuckGoroutine is the goroutine of a dump that runs compiler code, a running or runnable one
// before a waiting one; "" when there is none.
func stuckGoroutine(dump string) string {
	waiting := ""
	for _, g := range strings.Split(dump, "\n\n") {
		if len(frameList(g, 1)) == 0 {
			continue
		}
		head, _, _ := strings.Cut(g, "\n")
		if strings.Contains(head, stateRunning) || strings.Contains(head, stateRunnable) {
			return g
		}
		if waiting == "" {
			waiting = g
		}
	}
	return waiting
}
