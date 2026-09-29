package eval

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// IMPLEMENTATION-PLAN §7.6: an entry notes the files of the defaults and where predicates it ran.
func TestMemoNotesCode(t *testing.T) {
	ctx := context.Background()
	fs := &source.FileSet{}
	srcs := [][2]string{
		{"a/t.canon", "/// A.\npackage a\n\n/// An item.\nrecord Item {\n  /// N.\n  n: Pos = 1\n  /// M.\n  m: Int = n + 1\n}\n\n/// Items.\nlet items: table Item = {}\n"},
		{"a/w.canon", "package a\n\n/// Positive.\ntype Pos = Int where it > 0\n"},
		{"a/e.canon", "package a\n\nentry items.one { n: 2 }\n"},
	}
	var files []*syntax.File
	for _, s := range srcs {
		src, err := fs.Add(s[0], "/"+s[0], []byte(s[1]))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, "")))
	}
	bags := check.Bags{}
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, NewFolder(bags, Options{}))
	ev := New(prog, nil, bags, Options{})
	m := NewMemo()
	ev.UseMemo(m, 1)
	if _, ok := ev.Force(ctx, Root{Pkg: "a", Name: "items"}); !ok || len(m.gens[0].entries) != 1 {
		t.Fatalf("items: forced %t, %d entries kept", ok, len(m.gens[0].entries))
	}
	for _, en := range m.gens[0].entries { //canon:unordered one entry
		for _, f := range files[:2] {
			if !slices.Contains(en.files, f) {
				t.Errorf("%s: not noted", f.Src.Path)
			}
		}
	}
}

// IMPLEMENTATION-PLAN §7.6: a memo keeps each epoch's store apart, less the declarations and files gone.
func TestMemoBegin(t *testing.T) {
	live, gone := &syntax.EntryDecl{}, &syntax.EntryDecl{}
	kept, dropped := &syntax.File{}, &syntax.File{}
	alive := liveness{
		decl: func(d *syntax.EntryDecl) bool { return d == live },
		file: func(f *syntax.File) bool { return f == kept },
	}
	m := NewMemo()
	u := &memoUse{m: m, gen: m.begin(1, alive)}
	u.store(memoKey{decl: live}, &memoEntry{size: memoNodeBytes})
	u.store(memoKey{decl: gone}, &memoEntry{size: memoNodeBytes})
	for _, f := range []*syntax.File{kept, dropped} {
		cachedFile(u, filesIndexed, f, func(*syntax.File) fileIndex { return fileIndex{} })
	}
	if u.gen = m.begin(1, alive); u.lookup(memoKey{decl: live}) == nil || u.lookup(memoKey{decl: gone}) != nil || u.gen.bytes != memoNodeBytes {
		t.Errorf("same epoch: want the live entry kept and the gone one dropped")
	}
	if _, ok := u.gen.indexed[dropped]; ok || len(u.gen.indexed) != 1 {
		t.Errorf("same epoch: want the live file's index kept and the gone one's dropped")
	}
	if u.gen = m.begin(2, alive); u.lookup(memoKey{decl: live}) != nil || len(u.gen.indexed) != 0 {
		t.Errorf("new epoch: want nothing kept")
	}
	if u.gen = m.begin(1, alive); u.lookup(memoKey{decl: live}) == nil {
		t.Errorf("first epoch again: want its store kept beside the second's")
	}
	for epoch := uint64(3); epoch < 3+memoEpochs; epoch++ {
		m.begin(epoch, alive)
	}
	if u.gen = m.begin(1, alive); u.lookup(memoKey{decl: live}) != nil || len(m.gens) != memoEpochs {
		t.Errorf("past memoEpochs newer epochs: want the first one's store dropped, %d kept", len(m.gens))
	}
	u.store(memoKey{decl: live}, &memoEntry{size: memoNodeBytes})
	if m.Forget(1); m.begin(1, alive) != nil || len(m.gens) != memoEpochs-1 {
		t.Errorf("forgotten epoch (log-2026-09-29 M4 P3-r): want its store dropped and never made again")
	}
}

// IMPLEMENTATION-PLAN §7.6: the stores together stay within memoAllBytes, the least recent emptied first.
func TestMemoBoundAllEpochs(t *testing.T) {
	m := NewMemo()
	var uses []*memoUse
	for epoch := range uint64(memoAllBytes/memoBytes + 1) {
		u := &memoUse{m: m, gen: m.begin(epoch, liveness{})}
		u.store(memoKey{decl: &syntax.EntryDecl{}}, &memoEntry{size: memoBytes})
		uses = append(uses, u)
	}
	if uses[0].gen.bytes != 0 || uses[len(uses)-1].gen.bytes != memoBytes || uses[1].gen.bytes != memoBytes {
		t.Errorf("past memoAllBytes: want the least recent store emptied, the others kept")
	}
}

// IMPLEMENTATION-PLAN §7.6 (log-2026-09-29 M4 P3-r): after a store is emptied the memo stays within memoAllBytes.
func TestMemoBoundAfterForget(t *testing.T) {
	m := NewMemo()
	sizes := []int{memoBytes, memoBytes - memoNodeBytes, memoNodeBytes}
	uses := make([]*memoUse, len(sizes))
	for i, size := range sizes {
		uses[i] = &memoUse{m: m, gen: m.begin(uint64(i), liveness{})}
		uses[i].store(memoKey{decl: &syntax.EntryDecl{}}, &memoEntry{size: size})
	}
	last := uses[len(uses)-1]
	last.store(memoKey{decl: &syntax.EntryDecl{}}, &memoEntry{size: memoBytes})
	total := 0
	for _, g := range m.gens {
		total += g.bytes
	}
	if total > memoAllBytes || uses[0].gen.bytes != 0 || last.gen.bytes != memoBytes {
		t.Errorf("%d bytes kept, %d in the least recent store: want at most %d, it emptied", total, uses[0].gen.bytes, memoAllBytes)
	}
	if last.store(memoKey{decl: &syntax.EntryDecl{}}, &memoEntry{size: memoBytes + 1}); last.stats.unkept != 1 || last.gen.bytes != memoBytes {
		t.Errorf("an entry past memoBytes: want it not kept, the store unchanged")
	}
}

// IMPLEMENTATION-PLAN §7.6: a memo past its byte bound forgets its entries and keeps recording.
func TestMemoBound(t *testing.T) {
	m := NewMemo()
	u := &memoUse{m: m, gen: m.begin(1, liveness{})}
	first, second := memoKey{decl: &syntax.EntryDecl{}}, memoKey{decl: &syntax.EntryDecl{}}
	u.store(first, &memoEntry{size: memoBytes})
	u.store(second, &memoEntry{size: memoNodeBytes})
	if u.lookup(first) != nil || u.lookup(second) == nil || u.stats.stored != 2 || u.gen.bytes != memoNodeBytes {
		t.Errorf("past the bound: want the first entry forgotten and the second kept")
	}
}

// IMPLEMENTATION-PLAN §7.6: graphs built alike share a fingerprint, graphs that differ do not.
func TestFingerprintOf(t *testing.T) {
	e := newEvaluator(nil, Options{})
	coll := &types.Collection{Name: "c"}
	graph := func(v int64, at source.Pos, shared bool, c *types.Collection) value.Value {
		p := &value.Prov{Span: source.Span{File: 1, Start: at}}
		x := &value.Int{V: v, T: types.IntType, P: p}
		y := x
		if !shared {
			y = &value.Int{V: v, T: types.IntType, P: p}
		}
		rec := &value.Record{Fields: []value.Value{x}, Set: []bool{true}, Ident: &value.Identity{Coll: c}, P: p}
		return &value.List{Elems: []value.Value{rec, y}, P: p}
	}
	base := e.fingerprintOf(graph(1, 0, true, coll))
	if again := e.fingerprintOf(graph(1, 0, true, coll)); !again.ok || again.fp != base.fp {
		t.Errorf("graphs built alike: fingerprints differ")
	}
	for name, v := range map[string]value.Value{
		"scalar":     graph(2, 0, true, coll),
		"provenance": graph(1, 1, true, coll),
		"sharing":    graph(1, 0, false, coll),
		"identity":   graph(1, 0, true, &types.Collection{Name: "c"}),
	} {
		if e.fingerprintOf(v).fp == base.fp {
			t.Errorf("%s differs: same fingerprint", name)
		}
	}
	if e.fingerprintOf(&value.List{Elems: []value.Value{&closure{}}}).ok {
		t.Errorf("a function value: want no fingerprint")
	}
}
