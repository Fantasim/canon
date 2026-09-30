package load

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

const (
	keptHeader   = "#define A 1\n#define B (A << 2)\n#define M(x) (x)\n"
	editedHeader = "#define A 3\n#define B (A << 2)\n#define M(x) (x)\n"
	dupHeader    = "#define X 1\n#define X 2\n"
	headerPath   = "codes.h"
	headerAbs    = "/p/codes.h"
	twinPath     = "twin.h"
	twinAbs      = "/p/twin.h"
	headerPkg    = "p"
	editedA      = 3
	editedB      = 12
	madeTwin     = 2
	madeEdited   = 3
	dupReads     = 2
)

// headerSource adds text to set as the header every case reads.
func headerSource(t *testing.T, set *source.FileSet, text string) *source.File {
	t.Helper()
	src, err := set.Add(headerPath, headerAbs, []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// IMPLEMENTATION-PLAN §7.6, WIRE.md §6.8: a classification is kept while the source file is the same.
func TestHeadersKeptWhileSameFile(t *testing.T) {
	set := &source.FileSet{}
	req := Request{Pkg: headerPkg, Bag: diag.NewBag(set, headerPkg)}
	h := &Headers{}
	src := headerSource(t, set, keptHeader)
	first := h.classify(src, req)
	again := h.classify(src, req)
	if h.made != 1 || !slices.Equal(first.defs, again.defs) || !slices.Equal(first.skipped, again.skipped) {
		t.Fatalf("classified %d times; kept %v %v, then %v %v", h.made, first.defs, first.skipped, again.defs, again.skipped)
	}
	twin, err := set.Add(twinPath, twinAbs, []byte(keptHeader))
	if err != nil {
		t.Fatal(err)
	}
	c := h.classify(twin, req)
	if h.made != madeTwin || c.defs[0].at.File != twin.ID || c.skipped[0].at.File != twin.ID || twin.ID == src.ID {
		t.Errorf("another path of the same bytes: classified %d times, spans in files %d and %d, want %d and %d (not %d)",
			h.made, c.defs[0].at.File, c.skipped[0].at.File, madeTwin, twin.ID, src.ID)
	}
	edited := h.classify(headerSource(t, set, editedHeader), req)
	if h.made != madeEdited || edited.defs[0].val != editedA || edited.defs[1].val != editedB {
		t.Errorf("edited: classified %d times, defines %v; want %d times, A=%d and B=%d", h.made, edited.defs, madeEdited, editedA, editedB)
	}
	if fd := req.Bag.Findings(); len(fd) != 0 {
		t.Errorf("findings = %v, want none", fd)
	}
}

// WIRE.md §6.8, IMPLEMENTATION-PLAN §7.6: a header with an E7102 is never kept, so each read reports it.
func TestHeadersKeepNoFinding(t *testing.T) {
	set := &source.FileSet{}
	h := &Headers{}
	src := headerSource(t, set, dupHeader)
	for i := range dupReads {
		bag := diag.NewBag(set, headerPkg)
		if c := h.classify(src, Request{Pkg: headerPkg, Bag: bag}); c.ok {
			t.Fatalf("read %d: a duplicate of a different value classified as ok", i)
		}
		if fd := bag.Findings(); h.made != 0 || len(fd) != 1 || fd[0].Code != diag.E7102.Def().Code {
			t.Errorf("read %d: kept %d; findings %v, want nothing kept and one %s", i, h.made, fd, diag.E7102.Def().Code)
		}
	}
}

// IMPLEMENTATION-PLAN §7.6 (≡ cold), log-2026-09-29 M4 P18: a scratch request keeps nothing, yet takes what a real one kept.
func TestHeadersScratch(t *testing.T) {
	set := &source.FileSet{}
	src := headerSource(t, set, keptHeader)
	h := &Headers{}
	scratch := Request{Pkg: headerPkg, Bag: diag.NewBag(set, headerPkg), Scratch: true}
	h.classify(src, scratch)
	if h.made != 0 {
		t.Fatalf("a scratch request kept %d classifications", h.made)
	}
	kept := h.classify(src, Request{Pkg: headerPkg, Bag: diag.NewBag(set, headerPkg)})
	taken := h.classify(src, scratch)
	var none *Headers
	fresh := none.classify(src, scratch)
	if h.made != 1 || !slices.Equal(taken.defs, kept.defs) || !slices.Equal(fresh.defs, kept.defs) || !slices.Equal(fresh.skipped, kept.skipped) {
		t.Errorf("kept %d; scratch took %v, a nil Headers gave %v %v; want 1 kept and %v %v", h.made, taken.defs, fresh.defs, fresh.skipped, kept.defs, kept.skipped)
	}
}
