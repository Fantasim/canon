package cxx

import (
	"bytes"
	"errors"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/testkit/cxx/config"
)

const (
	// oversubscribe is how many goroutines compete per slot.
	oversubscribe = 4
	// workTime is how long each goroutine of the bound test holds its slot.
	workTime = 5 * time.Millisecond
	// blockedWait is how long the blocked-command test waits to see a command stay queued.
	blockedWait = 100 * time.Millisecond
	// goBinary and goVersion are a command that exists wherever the tests run and prints one line.
	goBinary  = "go"
	goVersion = "version"
	// panicValue is what the panic test panics with.
	panicValue = "hold panic"
	// missingCommand names no executable.
	missingCommand = "canon-no-such-command"
)

// No more than cap(slots()) holders run at once, however many ask.
func TestHoldBoundsConcurrency(t *testing.T) {
	limit := cap(slots())
	var mu sync.Mutex
	var inFlight, peak int
	var wg sync.WaitGroup
	for range limit * oversubscribe {
		wg.Go(func() {
			release := Hold()
			defer release()
			mu.Lock()
			inFlight++
			peak = max(peak, inFlight)
			mu.Unlock()
			time.Sleep(workTime)
			mu.Lock()
			inFlight--
			mu.Unlock()
		})
	}
	wg.Wait()
	if peak > limit {
		t.Errorf("peak in flight %d, over the %d slots", peak, limit)
	}
	if peak < 1 {
		t.Errorf("peak in flight %d, want at least 1", peak)
	}
}

// slotCount is at least one, whatever the CPU count.
func TestSlotCountAtLeastOne(t *testing.T) {
	if n := slotCount(); n < 1 {
		t.Errorf("slotCount() = %d, want at least 1", n)
	}
}

// Combined, Output and Compile each wait for a slot: with every slot held, none returns.
func TestRunnersWaitForASlot(t *testing.T) {
	runners := map[string]func() error{
		"Combined": func() error { _, err := Combined(exec.Command(goBinary, goVersion)); return err },
		"Output":   func() error { _, err := Output(exec.Command(goBinary, goVersion)); return err },
		"Compile":  func() error { _, err := Compile(exec.Command(goBinary, goVersion)); return err },
	}
	for name, run := range runners {
		t.Run(name, func(t *testing.T) {
			var releases []func()
			var once sync.Once
			releaseAll := func() {
				once.Do(func() {
					for _, release := range releases {
						release()
					}
				})
			}
			t.Cleanup(releaseAll)
			for range cap(slots()) {
				releases = append(releases, Hold())
			}
			done := make(chan error, 1)
			go func() { done <- run() }()
			select {
			case err := <-done:
				t.Fatalf("returned with every slot held: %v", err)
			case <-time.After(blockedWait):
			}
			releaseAll()
			if err := <-done; err != nil {
				t.Errorf("after release: %v", err)
			}
		})
	}
}

// A command that fails to start gives its slot back: more failures than slots never deadlock.
func TestRunnersReleaseOnFailure(t *testing.T) {
	for range cap(slots()) + 1 {
		if _, err := Combined(exec.Command(missingCommand)); err == nil {
			t.Fatal("Combined: want an error")
		}
		if _, err := Output(exec.Command(missingCommand)); err == nil {
			t.Fatal("Output: want an error")
		}
		if _, err := Compile(exec.Command(missingCommand)); err == nil {
			t.Fatal("Compile: want an error")
		}
	}
}

// A holder that panics gives its slot back: Hold's release runs on the panic path.
func TestHoldReleasesOnPanic(t *testing.T) {
	func() {
		defer func() { _ = recover() }()
		defer Hold()()
		panic(panicValue)
	}()
	if held := len(slots()); held != 0 {
		t.Errorf("%d slots still held after a panic", held)
	}
}

// Compile refuses a command whose Stdout or Stderr is set, as CombinedOutput does.
func TestCompileRefusesSetOutput(t *testing.T) {
	var sink bytes.Buffer
	cmd := exec.Command(goBinary, goVersion)
	cmd.Stdout = &sink
	if _, err := Compile(cmd); !errors.Is(err, errOutputSet) {
		t.Errorf("Compile with Stdout set = %v, want errOutputSet", err)
	}
}

// Without an override, the slot count never passes maxSlots, whatever the CPU count.
func TestSlotCountCapped(t *testing.T) {
	t.Setenv(config.SlotsEnv, "")
	if n := slotCount(); n > maxSlots {
		t.Errorf("slotCount() = %d, over maxSlots %d", n, maxSlots)
	}
}

// Compile returns the command's combined output, and wraps a failure so errors.As still finds it.
func TestCompileOutputAndWrappedError(t *testing.T) {
	out, err := Compile(exec.Command(goBinary, goVersion))
	if err != nil || len(out) == 0 {
		t.Fatalf("Compile(go version) = %q, %v; want output and no error", out, err)
	}
	_, err = Compile(exec.Command(goBinary, missingCommand))
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("Compile(go %s) = %v; want an *exec.ExitError in the chain", missingCommand, err)
	}
}
