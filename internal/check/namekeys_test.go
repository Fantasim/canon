package check_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// nameKeysBase holds keyed lists with dynamic keys: a map literal keyed by a ref into one, at one
// level and through a ref chain (DECISIONS 456), and, as witnesses, each key written where a
// value is expected, which Keys records.
const nameKeysBase = `package a

local record S {
  name: String
  w: Int = 0
}

local record A {
  id: String
  w: Int = 0
  subs: [S] keyed by name = []
}

local record Bag {
  aa: [A] keyed by id
}

local record J {
  n: String
}

local record K {
  j: ref js
}

local let js: [J] keyed by n = [{ n: "second" }]

local let ks: [K] keyed by j = [{ j: second }]

local let aa: [A] keyed by id = [{ id: "first" }, { id: "second", subs: [{ name: "x" }] }]

local let b: Bag = { aa: [{ id: "second" }] }

local let m: {ref aa: Int} = { second: 1 }

local let mk: {ref ks: Int} = { second: 1 }

local let wr: ref aa = second

local let wk: ref ks = second

local let wb: A = b.aa.second

local let ws: S = aa.second.subs.x
`

// nameKeysLayer amends an entry by its `.k`: of the let, through a record field, and after an
// entry, in a nested keyed list.
const nameKeysLayer = `package a
layer dev

amend aa {
  second.w: 3
  second.subs.x.w: 4
}

amend b {
  aa.second.w: 5
}
`

// IMPLEMENTATION-PLAN §4.7, DECISIONS 470: bare-name keys are in NameKeys by their value form's Keys collection.
func TestNameKeysRecordBareKeys(t *testing.T) {
	prog, files, out := checkSet(t, [][2]string{{"a/a.canon", nameKeysBase}, {"a/dev.layer.canon", nameKeysLayer}})
	if !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	for _, f := range files {
		for _, g := range gaps(f, prog.Info, func(syntax.Decl) bool { return false }) {
			t.Error(g)
		}
	}
	base, layer := files[0], files[1]
	for _, tc := range []struct {
		name    string
		f       *syntax.File
		line    int
		col     int
		witness string
	}{
		{"map literal key (TYPES.md §4.1)", base, 34, 32, "wr"},
		{"map literal key through a ref chain (DECISIONS 456)", base, 36, 33, "wk"},
		{"amend `.k` on the let (TYPES.md §9.3)", layer, 5, 3, "wr"},
		{"amend `.k` through a record field", layer, 10, 6, "wb"},
		{"amend `.k` after an entry (TYPES.md §9.1)", layer, 6, 15, "ws"},
	} {
		id := identAt(t, tc.f, tc.line, tc.col)
		got, want := prog.Info.NameKeys[id], prog.Info.Keys[letValue(t, base, tc.witness)]
		if got == nil || got != want {
			t.Errorf("%s: NameKeys[%s] = %v, want %v (Keys of %s)", tc.name, id.Name, got, want, tc.witness)
		}
		if prog.Info.ObjectOf(id) != nil {
			t.Errorf("%s: %s names an object as well", tc.name, id.Name)
		}
	}
}

// identAt is the identifier of f starting at line:col.
func identAt(t *testing.T, f *syntax.File, line, col int) *syntax.Ident {
	t.Helper()
	for _, id := range nodesOf[*syntax.Ident](f) {
		if l, c := f.Src.Position(f.Span(id).Start); l == line && c == col {
			return id
		}
	}
	t.Fatalf("%s: no identifier at %d:%d", f.Src.Path, line, col)
	return nil
}

// letValue is the value expression of f's let named name.
func letValue(t *testing.T, f *syntax.File, name string) syntax.Expr {
	t.Helper()
	for _, d := range f.Decls {
		if let, ok := d.(*syntax.LetDecl); ok && let.Name.Name == name {
			return let.Value
		}
	}
	t.Fatalf("no let %s", name)
	return nil
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, DECISIONS 470: a Recheck records NameKeys as a cold Check does.
func TestRecheckKeepsNameKeys(t *testing.T) {
	w := newWorld(t, map[string]string{
		"a/a.canon":         nameKeysBase,
		"a/m.canon":         "package a\n\nlocal let m2: {ref aa: Int} = { second: 1 }\n",
		"a/dev.layer.canon": nameKeysLayer,
	})
	_, s, _ := w.session()
	for _, st := range []step{
		{"a/m.canon", "{ second: 1 }", "{ first: 1, second: 2 }", true},
		{"a/m.canon", "{ first: 1, second: 2 }", "{ second: 3 }", true},
	} {
		s = w.step(s, st)
	}
}

// exemptBase holds keyed lists no types.Collection names: a variant case's field's, a map's
// value, a list's element.
const exemptBase = `package a

local record S {
  name: String
  w: Int = 0
}

local variant V {
  c {
    items: [S] keyed by name
  }
  d
}

local record Bag {
  v: V
  m: {String: [S] keyed by name}
  ls: [[S] keyed by name]
  rm: {String: S}
}

local let bag: Bag = { v: c { items: [{ name: "x" }] }, m: { "k": [{ name: "x" }] }, ls: [[{ name: "x" }], [{ name: "x" }]], rm: { "k": { name: "y" } } }
`

// exemptLayer amends an entry of each by its `.x`, and a field `.w` right after a `[k]`, which is recorded.
const exemptLayer = `package a
layer dev

amend bag {
  v.items.x.w: 1
  m["k"].x.w: 2
  ls[#0].x.w: 3
  ls[1].x.w: 4
  rm["k"].w: 5
}
`

// IMPLEMENTATION-PLAN §4.7, DECISIONS 470: the known exemption, unrecorded and passed by the oracle; a fix flips it.
func TestNameKeysKnownExemption(t *testing.T) {
	prog, files, out := checkSet(t, [][2]string{{"a/a.canon", exemptBase}, {"a/dev.layer.canon", exemptLayer}})
	if !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	layer := files[1]
	for _, f := range files {
		for _, g := range gaps(f, prog.Info, func(syntax.Decl) bool { return false }) {
			t.Error(g)
		}
	}
	for _, tc := range []struct {
		name      string
		line, col int
	}{
		{"a variant case's field", 5, 11},
		{"a map's value", 6, 10},
		{"a list's element by position", 7, 10},
		{"a list's element by index", 8, 9},
	} {
		id := identAt(t, layer, tc.line, tc.col)
		if prog.Info.NameKeys[id] != nil || prog.Info.ObjectOf(id) != nil {
			t.Errorf("%s: %s is recorded now: drop it from uncollectedKeys and this test", tc.name, id.Name)
		}
	}
}

// optionalBase holds a keyed list in an optional record field, read after an entry as a value.
const optionalBase = `package a

local record S {
  name: String
  w: Int = 0
}

local record A {
  id: String
  oo: ([S] keyed by name)?
}

local let aa: [A] keyed by id = [{ id: "second", oo: [{ name: "x" }] }]

local let wo: S? = aa.second.oo?.x
`

// TYPES.md §9.1, DECISIONS 470: a keyed list an optional field holds is a record field's: `.x` is its key, as a value and in an amend line.
func TestNameKeysThroughAnOptionalField(t *testing.T) {
	layerText := "package a\nlayer dev\n\namend aa {\n  second.oo.x.w: 1\n}\n"
	prog, files, out := checkSet(t, [][2]string{{"a/a.canon", optionalBase}, {"a/dev.layer.canon", layerText}})
	if !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	for _, f := range files {
		for _, g := range gaps(f, prog.Info, func(syntax.Decl) bool { return false }) {
			t.Error(g)
		}
	}
	value, ok := letValue(t, files[0], "wo").(*syntax.SelectorExpr)
	if !ok {
		t.Fatal("wo's value is no selector")
	}
	want := prog.Info.Keys[value]
	if want == nil || want.Kind != types.CollField {
		t.Errorf("value form: Keys[aa.second.oo?.x] = %v, want the field collection A.oo", want)
	}
	if got := prog.Info.NameKeys[identAt(t, files[1], 5, 13)]; got == nil || got != want {
		t.Errorf("amend form: NameKeys[x] = %v, want %v", got, want)
	}
}

// IMPLEMENTATION-PLAN §4.7, DECISIONS 470: `.x` after `aa.second.oo!` is keyed as after `?.`, nothing left out of Info.
func TestNameKeysAfterAForce(t *testing.T) {
	src := optionalBase + "\nlocal let wf: S = aa.second.oo!.x\n"
	prog, files, out := checkSet(t, [][2]string{{"a/a.canon", src}})
	if !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	for _, g := range gaps(files[0], prog.Info, func(syntax.Decl) bool { return false }) {
		t.Error(g)
	}
	want := prog.Info.Keys[letValue(t, files[0], "wo")]
	value, ok := letValue(t, files[0], "wf").(*syntax.SelectorExpr)
	if !ok {
		t.Fatal("wf's value is no selector")
	}
	if got := prog.Info.Keys[value]; want == nil || got != want {
		t.Errorf("Keys[aa.second.oo!.x] = %v, want %v, as after ?.", got, want)
	}
}
