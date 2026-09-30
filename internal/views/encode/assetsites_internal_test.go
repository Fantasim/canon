package encode

import (
	"runtime"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	siteText = "package a\n\nrecord Top {\n  icon: asset(\"icons\", ext: [png])\n  more: asset(\"sounds\")\n}\n"
	siteWant = 2                // the asset types siteText writes
	gcWait   = 5 * time.Second  // the bound on the wait for a cleanup; only a leak pays it
	gcPause  = time.Millisecond // the wait between two collections
)

// parseSite parses siteText into a file of its own.
func parseSite(t *testing.T) *syntax.File {
	t.Helper()
	set := &source.FileSet{}
	src, err := set.Add("a/a.canon", "/a/a.canon", []byte(siteText))
	if err != nil {
		t.Fatal(err)
	}
	return syntax.Parse(src, syntax.FileSource, diag.NewBag(set, "a"))
}

// held is the number of files c remembers.
func (c *siteCache) held() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.files)
}

// IMPLEMENTATION-PLAN 7.6: a file is walked once. A second ask returns the very nodes of the
// first, in the same backing array, so nothing was scanned again; a file parsed anew is walked on
// its own.
func TestSiteCacheWalksAFileOnce(t *testing.T) {
	var c siteCache
	f := parseSite(t)
	first := c.typesOf(f)
	if len(first) != siteWant {
		t.Fatalf("found %d asset types, want %d", len(first), siteWant)
	}
	again := c.typesOf(f)
	if len(again) != siteWant || &again[0] != &first[0] {
		t.Error("the second ask walked the file again")
	}
	other := c.typesOf(parseSite(t))
	if len(other) != siteWant || other[0] == first[0] {
		t.Error("a file parsed anew took the nodes of another")
	}
}

// IMPLEMENTATION-PLAN 7.6: the cache holds a file weakly; once the file is gone its entry goes.
func TestSiteCacheForgetsACollectedFile(t *testing.T) {
	var c siteCache
	keep := parseSite(t)
	defer runtime.KeepAlive(keep)
	c.typesOf(keep)
	base := c.held()
	func() { c.typesOf(parseSite(t)) }()
	if c.held() != base+1 {
		t.Fatalf("holds %d files, want %d", c.held(), base+1)
	}
	for deadline := time.Now().Add(gcWait); time.Now().Before(deadline); time.Sleep(gcPause) {
		runtime.GC()
		if c.held() == base {
			return
		}
	}
	t.Errorf("holds %d files after %v of collections, want %d", c.held(), gcWait, base)
}
