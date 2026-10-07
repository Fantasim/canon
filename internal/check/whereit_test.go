package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// TYPES.md §3.4, §4.1, STDLIB.md §5, DECISIONS 317, 318: `it` as a key argument is the value, never the key "it".
func TestItIsNeverASymbolicKey(t *testing.T) {
	prog, f := checkSource(t, `package a

local record Row {
  v: Int
}

local let rows: table Row = {
  A { v: 1 }
}

local let loaded: table Row = load("rows.json")

local record Named {
  name: String
}

local let named: [Named] keyed by name = [{ name: "A" }]

local type T1 = String where rows.hasKey(it)

local type T2 = String where rows.get(it) != none and loaded.find(it) != none

local type T3 = String where loaded.hasKey(it) and named.hasKey(it) and named[it].name != ""

local record R {
  s: String where rows.hasKey(it) and loaded.get(it) != none
}
`)
	seen := 0
	for _, id := range nodesOf[*syntax.IdentExpr](f) {
		if id.Name != "it" {
			continue
		}
		seen++
		if o := prog.Info.Uses[id]; prog.Info.Keys[id] != nil || o == nil || o.Kind() != check.ObjLocal {
			t.Errorf("it #%d: Keys %v, Uses %v, want the predicate's value", seen, prog.Info.Keys[id], o)
		}
	}
	const its = 8
	if seen != its {
		t.Errorf("saw %d `it`, want %d", seen, its)
	}
}

// itDependent is a type function with a String branch and a record whose field and map key it types.
const itDependent = `
local enum K { name, off }

local type P(k: K) = match k {
  name => String
  off => Never
}

local record H {
  k: K
  d: P(k)
}

local record M {
  k: K
  m: {P(k): Int}
}

local fn held(h: H) -> Bool {
  return h.k == name
}

local fn mapped(x: M) -> Bool {
  return x.m.len() == 1
}
`

// DECISIONS 318, TYPES.md §3.4: `it` read by every other path a bare name takes is the predicate's value.
func TestItIsAName(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"callee", "local fn f(g: fn(Int) -> Bool where it(1)) -> Bool {\n  return g(2)\n}\n"},
		{"map literal key", "local enum E { a, b }\n\nlocal fn one(m: {E: Int}) -> Bool {\n  return m.len() == 1\n}\n\n" +
			"local type T = E where one({ it: 1 })\n"},
		{"dependent value", itDependent + "\nlocal type T = String where held(H { k: name, d: it })\n"},
		{"dependent map key", itDependent + "\nlocal type T = String where mapped(M { k: name, m: { it: 1 } })\n"},
	} {
		prog, f := checkSource(t, "package a\n\n"+tc.src)
		seen := 0
		for _, id := range nodesOf[*syntax.IdentExpr](f) {
			if id.Name == "it" {
				seen++
				assertIt(t, tc.name, prog.Info.Uses[id], prog.Info.Keys[id] != nil || prog.Info.Symbols[id])
			}
		}
		for _, id := range nodesOf[*syntax.Ident](f) {
			if id.Name == "it" {
				seen++
				assertIt(t, tc.name, prog.Info.NameUses[id], false)
			}
		}
		if seen != 1 {
			t.Errorf("%s: saw %d `it`, want 1", tc.name, seen)
		}
	}
}

// TYPES.md §3.4, DECISIONS 334: inside a predicate `it` is its value, never a member or key named `it`.
func TestPredicateItBeatsStepOne(t *testing.T) {
	prog, f := checkSource(t, `package a

local enum G { it, other }

local record Row {
  v: Int
}

local let marks: table Row = {
  it { v: 1 }
  A { v: 2 }
}

local fn pick(r: ref marks) -> Bool {
  return marks[r].v == 2
}

local fn isOther(g: G) -> Bool {
  return g == other
}

local fn oneG(m: {G: Int}) -> Bool {
  return m.len() == 1
}

local type T1 = ref marks where pick(it)

local type T2 = G where isOther(it) and oneG({ it: 1 })
`)
	seen := 0
	for _, id := range nodesOf[*syntax.IdentExpr](f) {
		if id.Name == "it" {
			seen++
			assertIt(t, "value", prog.Info.Uses[id], prog.Info.Keys[id] != nil || prog.Info.Symbols[id])
		}
	}
	for _, id := range nodesOf[*syntax.Ident](f) {
		if id.Name == "it" && prog.Info.Defs[id] == nil {
			seen++
			assertIt(t, "map key", prog.Info.NameUses[id], false)
		}
	}
	const its = 3
	if seen != its {
		t.Errorf("saw %d `it` in predicates, want %d", seen, its)
	}
}

// TYPES.md §3.4, DECISIONS 334: a value named `it` hides the predicate's, so step 1 finds the member `it`.
func TestHiddenItTakesStepOne(t *testing.T) {
	prog, f := checkSource(t, `package a

local enum G { it, other }

local let it: Int = 3

local fn pickG(g: G) -> Bool {
  return g == other
}

local fn one(m: {G: Int}) -> Bool {
  return m.len() == 1
}

local type A2 = String where pickG(it) and one({ it: 1 })
`)
	seen := 0
	for _, id := range nodesOf[*syntax.IdentExpr](f) {
		if id.Name == "it" {
			seen++
			assertMember(t, prog.Info.Uses[id])
		}
	}
	for _, id := range nodesOf[*syntax.Ident](f) {
		if id.Name == "it" && prog.Info.Defs[id] == nil {
			seen++
			assertMember(t, prog.Info.NameUses[id])
		}
	}
	const its = 2
	if seen != its {
		t.Errorf("saw %d `it` in the predicate, want %d", seen, its)
	}
}

// assertMember fails unless o is the enum member `it`.
func assertMember(t *testing.T, o check.Object) {
	t.Helper()
	if o == nil || o.Kind() != check.ObjMember || o.Name() != "it" {
		t.Errorf("it names %v, want the member G.it", o)
	}
}

// assertIt fails unless o is the predicate's `it` and the name was not kept as a key or a symbol.
func assertIt(t *testing.T, name string, o check.Object, kept bool) {
	t.Helper()
	if kept || o == nil || o.Kind() != check.ObjLocal || o.Name() != "it" {
		t.Errorf("%s: kept as a key or symbol %t, names %v; want the predicate's value", name, kept, o)
	}
}
