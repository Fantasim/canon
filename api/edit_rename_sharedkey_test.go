package canon_test

import (
	"fmt"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// keyHead is S and A, whose pick is a ref into its own oo; {def} is pick's default.
const keyHead = "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\nrecord A {\n  id: String\n  oo: [S] keyed by k\n  pick: ref S{def}\n}\n\n"

// keyAA is aa, first holding x and y and picking y, second holding {k} and picking {ps}.
const keyAA = "let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\" }, { k: \"y\" }], pick: \"y\" },\n" +
	"  { id: \"second\", oo: [{ k: \"{k}\" }]{ps} },\n]\n"

// keyRow is a rename of second's x in keyHead and keyAA with marks and more after them: want are
// its refs, each `kind <line prefix>`, refused its refusal.
type keyRow struct {
	name, def, ps, more string
	want                []string
	refused             canon.Reason
}

// run renames second's x to y in r's source and checks it as sharedRow does.
func (r keyRow) run(t *testing.T) {
	marks := map[string]string{"{def}": r.def, "{ps}": r.ps}
	l := sharedLevel{path: "p:aa[second].oo[x]"}
	with := func(key string) map[string]string {
		return map[string]string{srcMain: l.fill(keyHead+keyAA+r.more, key, marks)}
	}
	before := with("x")
	var want []string
	for _, w := range r.want {
		kind, prefix, _ := strings.Cut(w, " ")
		want = append(want, fmt.Sprintf("%s %s:%d", kind, srcMain, lineOf(before[srcMain], prefix)))
	}
	sharedRow{l: l, before: before, after: with("y"), want: strings.Join(want, ", "), refused: r.refused}.run(t)
}

// API.md R7, E11, E22 (DECISIONS 410, 483): a ref copied from another instance keeps that
// instance's entry, whatever record it now sits in: second's pick copied from ff names ff's x,
// so it is no ref of second's x and the rename leaves it.
func TestEditRenameCopiedRefKeepsItsEntry(t *testing.T) {
	ff := "\nlet ff: A = { id: \"f\", oo: [{ k: \"x\" }, { k: \"y\" }], pick: \"x\" }\n"
	for _, r := range []keyRow{
		{name: "field copy", ps: ", pick: ff.pick", more: ff},
		{name: "copy through a function", ps: ", pick: same(ff).pick", more: ff + "\nfn same(r: A) -> A {\n  return r\n}\n"},
	} {
		t.Run(r.name, r.run)
	}
}

// API.md R7, E12, E22 (DECISIONS 483): a never-failing key test (hasKey, get, find) is a code ref
// when its receiver reaches second's instance statically, none when first's, and through a
// parameter or a default's own field the rename cannot judge it: refused `computed`.
func TestEditRenameKeyTestThatCannotFail(t *testing.T) {
	ps := ", pick: \"{k}\""
	for _, r := range []keyRow{
		{name: "static on second", ps: ps, more: "\nlet h: Bool = aa.second.oo.hasKey(\"{k}\")\n", want: []string{"value   { id: \"second\"", "code let h:"}},
		{name: "static on first", ps: ps, more: "\nlet h: Bool = aa.first.oo.hasKey(\"x\")\n", want: []string{"value   { id: \"second\""}},
		{name: "get through a lambda", ps: ps, more: "\nlet h: [Bool] = aa.map(a => a.oo.get(\"x\") != none)\n", want: []string{"value   { id: \"second\""}, refused: canon.ReasonComputed},
		{name: "hasKey through a lambda", ps: ps, more: "\nlet h: [Bool] = aa.map(a => a.oo.hasKey(\"x\"))\n", want: []string{"value   { id: \"second\""}, refused: canon.ReasonComputed},
		{name: "find through a function", ps: ps, more: "\nfn has(a: A) -> Bool {\n  return a.oo.find(\"x\") != none\n}\n", want: []string{"value   { id: \"second\""}, refused: canon.ReasonComputed},
		{name: "hasKey in a default", def: " = if oo.hasKey(\"x\") { \"y\" } else { \"y\" }", ps: ps, want: []string{"value   { id: \"second\""}, refused: canon.ReasonComputed},
	} {
		t.Run(r.name, r.run)
	}
}

// API.md R7, E12, E22 (DECISIONS 483): a key test is known by its receiver's type, a keyed list of
// the home field's records and key, wherever that list is held: a let holding second's list is a
// code ref, first's none; a lambda, parameter or local holding it refuses the rename.
func TestEditRenameKeyTestByType(t *testing.T) {
	ps, second := ", pick: \"{k}\"", "value   { id: \"second\""
	for _, r := range []keyRow{
		{name: "let holding second's list", ps: ps, more: "\nlet s2: [S] keyed by k = aa.second.oo\n\nlet h: Bool = s2.hasKey(\"{k}\")\n", want: []string{second, "code let h:"}},
		{name: "let holding first's list", ps: ps, more: "\nlet s1: [S] keyed by k = aa.first.oo\n\nlet h: Bool = s1.hasKey(\"x\")\n", want: []string{second}},
		{name: "lambda over the mapped lists", ps: ps, more: "\nlet h: [Bool] = aa.map(a => a.oo).map(o => o.hasKey(\"x\"))\n", want: []string{second}, refused: canon.ReasonComputed},
		{name: "function parameter", ps: ps, more: "\nfn has(xs: [S] keyed by k) -> Bool {\n  return xs.hasKey(\"x\")\n}\n\nlet h: [Bool] = aa.map(a => has(a.oo))\n", want: []string{second}, refused: canon.ReasonComputed},
		{name: "function local", ps: ps, more: "\nfn has(a: A) -> Bool {\n  let o = a.oo\n  return o.hasKey(\"x\")\n}\n\nlet h: [Bool] = aa.map(a => has(a))\n", want: []string{second}, refused: canon.ReasonComputed},
		{name: "let of its own entries", ps: ps, more: "\nlet ss: [S] keyed by k = [{ k: \"x\" }]\n\nlet h: Bool = ss.hasKey(\"x\")\n", want: []string{second}},
		{name: "let filtering second's list", ps: ps, more: "\nlet s2: [S] keyed by k = aa.second.oo.filter(s => s.w == 0)\n\nlet h: Bool = s2.hasKey(\"x\")\n", want: []string{second}, refused: canon.ReasonComputed},
		{name: "let mapping second's keys", ps: ps, more: "\nlet s2: [S] keyed by k = aa.second.oo.map(s => S { k: s.k })\n\nlet h: Bool = s2.hasKey(\"x\")\n", want: []string{second}, refused: canon.ReasonComputed},
		{name: "let spreading second's entries", ps: ps, more: "\nlet s2: [S] keyed by k = aa.second.oo.map(s => S { ...s, w: 1 })\n\nlet h: Bool = s2.hasKey(\"x\")\n", want: []string{second}, refused: canon.ReasonComputed},
		{name: "let concatenating second's list", ps: ps, more: "\nlet s2: [S] keyed by k = aa.second.oo + []\n\nlet h: Bool = s2.hasKey(\"x\")\n", want: []string{second}, refused: canon.ReasonComputed},
	} {
		t.Run(r.name, r.run)
	}
}

// API.md R7, E12, E22 (DECISIONS 483): a constant's key is folded as a literal's: through a lambda
// the rename refuses; on second's list too, since the shared constant cannot be rewritten for one
// test; on first's list it is none.
func TestEditRenameKeyTestConstantKey(t *testing.T) {
	ps, second, kx := ", pick: \"{k}\"", "value   { id: \"second\"", "\nconst KX = \"x\"\n"
	for _, r := range []keyRow{
		{name: "through a lambda", ps: ps, more: kx + "\nlet h: [Bool] = aa.map(a => a.oo.hasKey(KX))\n", want: []string{second}, refused: canon.ReasonComputed},
		{name: "on second's list", ps: ps, more: kx + "\nlet h: Bool = aa.second.oo.hasKey(KX)\n", want: []string{second}, refused: canon.ReasonComputed},
		{name: "on first's list", ps: ps, more: kx + "\nlet h: Bool = aa.first.oo.hasKey(KX)\n", want: []string{second}},
	} {
		t.Run(r.name, r.run)
	}
}

// API.md R7, E12 (DECISIONS 483): a default whose record a lambda builds without its field is
// taken by evaluations no walk stores; the rename cannot judge that default and is refused.
func TestEditRenameTransientDefault(t *testing.T) {
	transient := "\nlet names: [String] = aa.filter(a => a.id == \"first\").map(a => \"{A { id: \"t\", oo: a.oo }.pick}\")\n"
	for _, r := range []keyRow{
		{name: "default taken by second and a transient", def: " = \"{k}\"", more: transient, want: []string{"code   pick: ref"}, refused: canon.ReasonComputed},
		{name: "default taken by second alone", def: " = \"{k}\"", want: []string{"code   pick: ref"}},
		{name: "default and an unused function", def: " = \"{k}\"", more: "\nfn mk(a: A) -> A {\n  return { id: \"t\", oo: a.oo }\n}\n", want: []string{"code   pick: ref"}},
	} {
		t.Run(r.name, r.run)
	}
}

// API.md E12 (DECISIONS 464, 483): a rename refused both for a default first and second share
// (`computed`) and for a target the active layer states (`layered`) is refused `layered`.
func TestEditRenameRefusalLayeredFirst(t *testing.T) {
	l := sharedLevel{path: "p:aa[second].oo[x]"}
	src := l.fill(keyHead+keyAA, "x", map[string]string{"{def}": " = \"x\"", "{ps}": ""})
	src = strings.Replace(src, ", pick: \"y\" }", " }", 1)
	files := map[string]string{srcMain: src, sharedLayer: "package p\nlayer dev\n\namend aa {\n  second.oo: [{ k: \"x\" }]\n}\n"}
	want := fmt.Sprintf("code %s:%d", srcMain, lineOf(src, "  pick: ref"))
	sharedRow{l: l, layers: []string{"dev"}, before: files, after: files, want: want, refused: canon.ReasonLayered}.run(t)
}
