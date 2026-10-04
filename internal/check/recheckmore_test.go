package check_test

import (
	"context"
	"maps"
	"slices"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	axe       = "game/items/entries/IK1_WEAPON/II_WEA_AXE_ANGEL.canon"
	moonstone = "game/items/entries/IK1_GENERAL/II_GEN_MAT_MOONSTONE.canon"
)

// IMPLEMENTATION-PLAN §7.6 NFR-02: every example file, and edits of its entry files, against a cold Check.
func TestRecheckExamples(t *testing.T) {
	l := loadExamples(t)
	srcs := map[string]string{}
	for _, f := range l.files {
		srcs[f.Src.Path] = string(f.Src.Content)
	}
	w := newWorld(t, srcs)
	_, s, _ := w.session()
	s = stepEveryFile(w, s)
	for _, st := range []step{
		{axe, "cost: 300_000", "cost: 1", true},
		{axe, "cost: 1", `cost: "x"`, true},
		{axe, `cost: "x"`, "cost: 300_000", true},
		{axe, "attackMin: 310", "attackMin: attackMax", true},
		{moonstone, "\n}\n", "\n  bogus: 1\n}\n", true},
		{moonstone, "entry items.", "retired entry items.", false},
		{"game/items/item.canon", "package game.items", "package game.items\n", false},
	} {
		s = w.step(s, st)
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: two entry files of one package edited at once.
func TestRecheckTwoFiles(t *testing.T) {
	w := newWorld(t, dups)
	_, s, _ := w.session()
	a := w.edit("dup/a.canon", "entry rows.x { n: 1 }", "entry rows.x {\n  n: \"one\"\n}")
	c := w.edit("dup/c.canon", "label: \"c\"", "label: 3")
	prog, _, bags, ok := w.recheck(s, a, c)
	if !ok {
		t.Fatal("Recheck refused two body-only edits")
	}
	if got, want := canonical(w.fs, prog, bags), w.cold(); got != want {
		t.Fatalf("Recheck differs from Check:\n%s", firstDiff(got, want))
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: a kept finding reading the changed file (its import closes a cycle) refuses.
func TestRecheckRefusesAKeptFindingReadingTheFile(t *testing.T) {
	w := newWorld(t, map[string]string{
		"x/x.canon": "package x\n\nimport y\n\nlet q: Int = 1\n",
		"y/y.canon": "package y\n\nrecord R {\n  n: Int\n}\n\nlet rs: table R = {}\n",
		"y/a.canon": "package y\n\nimport x\n\nentry rs.a { n: 1 }\n",
	})
	_, s, _ := w.session()
	if _, _, _, ok := w.recheck(s, w.edit("y/a.canon", "n: 1", "n: 2")); ok {
		t.Error("Recheck kept an import cycle's finding located in the file it replaced")
	}
}

// DOCTRINE §5: files added to the file set in reverse path order, so FileIDs run against paths, check alike.
func TestCheckIgnoresFileIDs(t *testing.T) {
	l := loadExamples(t)
	examples := map[string]string{}
	for _, f := range l.files {
		examples[f.Src.Path] = string(f.Src.Content)
	}
	for _, srcs := range []map[string]string{examples, shop, dups} {
		order := slices.Sorted(maps.Keys(srcs))
		want := newWorldIn(t, srcs, order).cold()
		slices.Reverse(order)
		if got := newWorldIn(t, srcs, order).cold(); got != want {
			t.Errorf("FileIDs changed what Check found:\n%s", firstDiff(got, want))
		}
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: a file of annotated lets is a body-only candidate (package doc).
func TestRecheckAnnotatedLets(t *testing.T) {
	w := newWorld(t, map[string]string{
		"p/p.canon":    "package p\n\nlocal record Row {\n  n: Int\n}\n",
		"p/lets.canon": "package p\n\nlocal let rows: table Row = {\n  a { n: 1 }\n}\n\nlocal let limit: Int(0..) = 3\n",
	})
	_, s, _ := w.session()
	s = stepEveryFile(w, s)
	if !bodyOnly(w.files["p/lets.canon"]) {
		t.Fatal("the oracle refuses a file of annotated lets")
	}
	for _, st := range []step{
		{"p/lets.canon", "= 3", "= 4", true},
		{"p/lets.canon", "n: 1", "n: 2", true},
		{"p/lets.canon", "Int(0..)", "Int", false},
	} {
		s = w.step(s, st)
	}
}

// stepEveryFile re-parses every file of w unchanged, in path order, Recheck accepting exactly
// what bodyOnly does.
func stepEveryFile(w *world, s *check.Session) *check.Session {
	for _, path := range slices.Sorted(maps.Keys(w.src)) {
		s = w.step(s, step{path: path, ok: bodyOnly(w.files[path])})
	}
	return s
}

// bodyOnly is the test's own reading of a body-only candidate: a source file of `entry`
// declarations writing no type, and of annotated lets whose type holds no `where` and whose
// value writes no type (only the value is compared).
func bodyOnly(f *syntax.File) bool {
	if f.FileKind != syntax.FileSource || f.Layer != nil || f.Lang != nil {
		return false
	}
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *syntax.EntryDecl:
			if writesType(x, false) {
				return false
			}
		case *syntax.LetDecl:
			if x.Type == nil || writesType(x.Type, true) || writesType(x.Value, false) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// writesType reports a written type under n, or only a `where` predicate when wheres.
func writesType(n syntax.Node, wheres bool) bool {
	found := false
	if n == nil {
		return false
	}
	syntax.Inspect(n, func(m syntax.Node) bool {
		_, isWhere := m.(*syntax.WhereType)
		_, isType := m.(syntax.Type)
		found = found || isWhere || (isType && !wheres)
		return !found
	})
	return found
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: old-program readers race Recheck; one of concurrent Rechecks goes through.
func TestRecheckConcurrency(t *testing.T) {
	w := newWorld(t, shop)
	old, s, oldBags := w.session()
	want := canonical(w.fs, old, oldBags)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 10 {
				if got := canonical(w.fs, old, oldBags); got != want {
					t.Errorf("the old program changed under Recheck:\n%s", firstDiff(got, want))
					return
				}
			}
		})
	}
	edits := []*syntax.File{
		w.edit("shop/items/apple.canon", "price: 3", "price: 5"),
		w.edit("shop/items/apple.canon", "price: 3", "price: 6"),
		w.edit("shop/items/apple.canon", "price: 3", "price: 7"),
	}
	var through sync.Map
	var racers sync.WaitGroup
	for i, nf := range edits {
		bags := check.Bags{}
		w.parseInto(bags, nf)
		racers.Go(func() {
			if _, _, ok := s.Recheck(context.Background(), []*syntax.File{nf}, bags, eval.NewFolder(bags, eval.Options{})); ok {
				through.Store(i, true)
			}
		})
	}
	racers.Wait()
	wg.Wait()
	n := 0
	through.Range(func(any, any) bool { n++; return true })
	if n != 1 {
		t.Errorf("%d concurrent Rechecks of one session went through, want 1", n)
	}
}
