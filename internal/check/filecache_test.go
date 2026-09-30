package check_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

const cacheWorkers = 16

// counter is a scan that counts its calls and answers with the call's number.
type counter struct{ calls atomic.Int32 }

func (c *counter) scan(*syntax.File) int32 { return c.calls.Add(1) }

func programOf(files ...*syntax.File) *check.Program {
	return &check.Program{Packages: []*check.Package{{Files: files}}}
}

// IMPLEMENTATION-PLAN.md §7.6: a hit reuses, a miss and a replaced tree scan, a nil cache keeps nothing.
func TestFileCacheOf(t *testing.T) {
	a, b, a2 := &syntax.File{}, &syntax.File{}, &syntax.File{}
	tests := []struct {
		name      string
		cache     *check.FileCache[int32]
		files     []*syntax.File
		wantScans int32
		wantSame  bool // the last two answers are the same value
	}{
		{"miss then hit", &check.FileCache[int32]{}, []*syntax.File{a, a}, 1, true},
		{"two files, two misses, one hit", &check.FileCache[int32]{}, []*syntax.File{a, b, b}, 2, true},
		{"a replaced tree is a miss", &check.FileCache[int32]{}, []*syntax.File{a, a2}, 2, false},
		{"a nil cache keeps nothing", nil, []*syntax.File{a, a}, 2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c counter
			var last, prev int32
			for _, f := range tt.files {
				prev, last = last, tt.cache.Of(f, c.scan)
			}
			if got := c.calls.Load(); got != tt.wantScans {
				t.Errorf("scans = %d, want %d", got, tt.wantScans)
			}
			if same := prev == last; same != tt.wantSame {
				t.Errorf("last two answers %d, %d: same = %v, want %v", prev, last, same, tt.wantSame)
			}
		})
	}
}

// IMPLEMENTATION-PLAN.md §7.6: KeepOnly forgets the files the program does not hold and keeps the rest.
func TestFileCacheKeepOnly(t *testing.T) {
	a, b := &syntax.File{}, &syntax.File{}
	tests := []struct {
		name         string
		prog         *check.Program
		wantA, wantB int32 // scans of a and b afterwards: 1 kept, 2 rescanned
	}{
		{"a dead file is dropped", programOf(a), 1, 2},
		{"a live file is kept", programOf(a, b), 1, 1},
		{"no program drops everything", nil, 2, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ca, cb counter
			cache := &check.FileCache[int32]{}
			cache.Of(a, ca.scan)
			cache.Of(b, cb.scan)
			cache.KeepOnly(tt.prog)
			cache.Of(a, ca.scan)
			cache.Of(b, cb.scan)
			if ca.calls.Load() != tt.wantA || cb.calls.Load() != tt.wantB {
				t.Errorf("scans a, b = %d, %d, want %d, %d", ca.calls.Load(), cb.calls.Load(), tt.wantA, tt.wantB)
			}
		})
	}
	var nilCache *check.FileCache[int32]
	nilCache.KeepOnly(programOf(a)) // a nil cache holds nothing: no panic
}

// IMPLEMENTATION-PLAN.md §7.6: concurrent Of is safe under -race and a stored answer is stable.
func TestFileCacheConcurrentOf(t *testing.T) {
	files := []*syntax.File{{}, {}, {}, {}}
	cache := &check.FileCache[int32]{}
	var c counter
	var wg sync.WaitGroup
	for range cacheWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, f := range files {
				cache.Of(f, c.scan)
				cache.KeepOnly(programOf(files...))
			}
		}()
	}
	wg.Wait()
	for _, f := range files {
		first := cache.Of(f, c.scan)
		if again := cache.Of(f, c.scan); again != first {
			t.Errorf("answer changed from %d to %d once stored", first, again)
		}
	}
	if n := c.calls.Load(); n < int32(len(files)) {
		t.Errorf("scans = %d, want at least one per file (%d)", n, len(files))
	}
}
