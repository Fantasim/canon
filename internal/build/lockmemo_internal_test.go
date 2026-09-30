package build

import (
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const locksCase = "testdata/incremental/locks.txtar"

// lockStep is one edit of TestIncrementalLocks and the lock codes its analysis must report.
type lockStep struct {
	name  string
	do    func(m roFS)
	codes []diag.Code
}

// LOCK.md §4.1, §4.2, §4.5, §5, IMPLEMENTATION-PLAN §7.6 NFR-02: warm lock findings are cold's.
func TestIncrementalLocks(t *testing.T) {
	z := archiveAnalyzer(t, locksCase)
	m := z.fs.base.(roFS)
	file := func(name string) string { return strings.TrimPrefix(path.Join(archiveRoot, name), "/") }
	edit := func(name, old, new string) func(roFS) {
		return func(m roFS) { m[file(name)] = srcFile(strings.Replace(string(m[file(name)].Data), old, new, 1)) }
	}
	put := func(name, text string) func(roFS) { return func(m roFS) { m[file(name)] = srcFile(text) } }
	e6001, e6002, e6005 := diag.E6001.Def().Code, diag.E6002.Def().Code, diag.E6005.Def().Code
	for _, st := range []lockStep{
		{"weight edited", edit("a/s/open.canon", "weight: 3", "weight: 4"), nil},
		{"entry added", put("a/s/late.canon", "package a\n\nentry statuses.late {\n  code: 4\n}\n"), nil},
		{"locked entry renamed", edit("a/s/done.canon", "statuses.done", "statuses.finished"), []diag.Code{e6001}},
		{"rename undone", edit("a/s/done.canon", "statuses.finished", "statuses.done"), nil},
		{"locked entry removed", func(m roFS) { delete(m, file("a/s/open.canon")) }, []diag.Code{e6001}},
		{"removed entry back", put("a/s/open.canon", "package a\n\nentry statuses.open {\n  code: 1\n}\n"), nil},
		{"entry retired", edit("a/s/done.canon", "entry statuses.done", "retired entry statuses.done"), nil},
		{"entry unretired", edit("a/s/old.canon", "retired entry statuses.old", "entry statuses.old"), []diag.Code{e6002}},
		{"unretired entry moved", edit("a/s/old.canon", "package a\n", "package a\n\n// moved down\n"), []diag.Code{e6002}},
		{"retired again", edit("a/s/old.canon", "entry statuses.old", "retired entry statuses.old"), nil},
		{"lock line removed on disk", edit("a/canon.lock", "table  a.statuses  open\n", ""), nil},
		{"lock line retired on disk", edit("a/canon.lock", "table  a.statuses  done\n", "table  a.statuses  done  retired\n"), nil},
		{"lock line unretired on disk", edit("a/canon.lock", "table  a.statuses  done  retired\n", "table  a.statuses  done\n"), nil},
		{"lock retiring a live entry", edit("a/canon.lock", "table  a.statuses  old  retired\n", "table  a.statuses  late  retired\ntable  a.statuses  old  retired\n"), []diag.Code{e6002}},
		{"lock broken on disk", edit("a/canon.lock", "# canon.lock v1\n", "# canon.lock v1\n<<<<<<< ours\n"), []diag.Code{e6005}},
		{"lock mended", edit("a/canon.lock", "<<<<<<< ours\n", ""), []diag.Code{e6002}},
		{"lock retirement undone", edit("a/canon.lock", "table  a.statuses  late  retired\n", ""), nil},
		{"b's locked key renamed", edit("b/b.canon", "sensitive {", "private {"), []diag.Code{e6001}},
		{"b's lock holds a gone key", edit("b/canon.lock", "table  b.flags  blocking\n", "table  b.flags  blocking\ntable  b.flags  hidden  retired\n"), []diag.Code{e6001, e6001}},
		{"b's key named back", edit("b/b.canon", "private {", "sensitive {"), []diag.Code{e6001}},
		{"b's lock mended", edit("b/canon.lock", "table  b.flags  hidden  retired\n", ""), nil},
		{"enum renumbered", edit("a/a.canon", "cold = 2", "cold = 3"), []diag.Code{e6002}},
		{"enum put back", edit("a/a.canon", "cold = 3", "cold = 2"), nil},
		{"stable value changed", edit("a/s/open.canon", "code: 1", "code: 9"), []diag.Code{e6002}},
		{"stable value put back", edit("a/s/open.canon", "code: 9", "code: 1"), nil},
	} {
		st.do(m)
		warm, cold := z.pair(t)
		same(t, st.name, warm, cold)
		if got := lockCodes(warm.Result().List); !slices.Equal(got, st.codes) {
			t.Errorf("%s: lock findings %v, want %v", st.name, got, st.codes)
		}
	}
	if kept := z.cache.gen.locks.orders; kept["a.statuses"] == nil || kept["b.flags"] == nil {
		t.Errorf("the cache kept no order of the stable tables: %v", kept)
	}
}

// lockFindingCodes are the codes of LOCK.md §11 an analysis reports.
var lockFindingCodes = []diag.Code{diag.E6001.Def().Code, diag.E6002.Def().Code, diag.E6003.Def().Code,
	diag.E6004.Def().Code, diag.E6005.Def().Code}

// lockCodes is the codes of the LOCK.md findings of list, in order.
func lockCodes(list []diag.Finding) []diag.Code {
	var out []diag.Code
	for _, f := range list {
		if slices.Contains(lockFindingCodes, f.Code) {
			out = append(out, f.Code)
		}
	}
	return out
}

// LOCK.md §2.4, IMPLEMENTATION-PLAN §7.6 NFR-02: an unchanged lock is read once, a bad one always.
func TestLockReadKept(t *testing.T) {
	z := archiveAnalyzer(t, locksCase)
	first, _ := z.pair(t)
	again, _ := z.pair(t)
	if len(first.r.locks) == 0 || first.r.locks[0].file != again.r.locks[0].file {
		t.Fatal("an unchanged canon.lock was read again")
	}
	m := z.fs.base.(roFS)
	bad := strings.TrimPrefix(path.Join(archiveRoot, "a/canon.lock"), "/")
	m[bad] = srcFile("# canon.lock v9\n")
	for range 2 {
		warm, cold := z.pair(t)
		same(t, "bad lock", warm, cold)
		if got := lockCodes(warm.Result().List); !slices.Equal(got, []diag.Code{diag.E6005.Def().Code}) {
			t.Errorf("bad lock: %v", got)
		}
	}
}
