package check_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// dependentSource declares a type function and fields whose types contain applications of it.
const dependentSource = `package a

local enum K { num, text }

local record Ev {
  k: K
}

local record Item {
  n: Int = 1
}

local let items: table Item = {
  one {}
}

local type P(e: Ev) = match e.k {
  num => String
  text => ref items
}

local type SK(e: Ev) = P(e) | "default"

local record L(e: Ev) {
  lkeys: {SK(e): Int} = {}
}

local record R {
  ev: Ev
  p: P(ev) = sym
  o: P(ev)? = none
  ps: [P(ev)] = []
  keys: {SK(ev): Int} = { "default": 1, key: 2 }
  byItem: {i in items: P(ev)} = {}
  counts: {i in items: Int} = {}
  l: L(ev) = {}
}

local fn reads(r: R) -> Int {
  let a = r.o
  let b = r.ps
  let c = r.keys
  let d = r.byItem
  let e = r.counts
  let f = r.l.lkeys
  return r.ps.len()
}
`

// TYPES.md §11.3–§11.5: a field whose type contains an application, of R(x) too, reads with each one its DepUnion.
func TestDependentStaticViews(t *testing.T) {
	prog, f := checkSource(t, dependentSource)
	want := map[string]string{
		"o":      "a.P(*)?",
		"ps":     "[a.P(*)]",
		"keys":   "{a.SK(*): Int}",
		"byItem": "{ref a.items: a.P(*)}",
		"counts": "{ref a.items: Int}",
		"lkeys":  "{a.SK(*): Int}",
	}
	seen := 0
	for _, s := range nodesOf[*syntax.SelectorExpr](f) {
		w, ok := want[s.Name.Name]
		if !ok {
			continue
		}
		seen++
		if got := prog.Info.Types[s]; got == nil || got.String() != w {
			t.Errorf("r.%s has static type %v, want %s", s.Name.Name, got, w)
		}
	}
	if seen < len(want) {
		t.Errorf("found %d of the %d reads", seen, len(want))
	}
}

// TYPES.md §4.1, §11.4 "Literals": a bare name given to a dependent field stays symbolic.
func TestDependentNamesStaySymbolic(t *testing.T) {
	prog, f := checkSource(t, dependentSource)
	for _, id := range nodesOf[*syntax.IdentExpr](f) {
		if id.Name == "sym" && !prog.Info.Symbols[id] {
			t.Errorf("the default %s is not symbolic", id.Name)
		}
	}
}

// TYPES.md §11: DependsOn lists the earlier fields a type's arguments read, a dependent map's value included.
func TestDependsOnThroughDependentMaps(t *testing.T) {
	prog, f := checkSource(t, dependentSource)
	for _, d := range f.Decls {
		rd, ok := d.(*syntax.RecordDecl)
		if !ok || rd.Name.Name != "R" {
			continue
		}
		rec, _ := prog.Info.Defs[rd.Name].Type().(*types.RecordType)
		if rec == nil {
			t.Fatal("R has no record type")
		}
		for _, fl := range rec.Fields {
			uses := slices.Contains([]string{"p", "o", "ps", "keys", "byItem", "l"}, fl.Name)
			if got := slices.Contains(fl.DependsOn, 0); got != uses {
				t.Errorf("%s depends on ev: %v, want %v", fl.Name, got, uses)
			}
		}
		return
	}
	t.Fatal("no record R")
}

// namesSource names package values and built-ins like the members a branch offers.
const namesSource = `package a

local enum K { num, mode }

local enum Rounding { floor, max }

local record Ev {
  k: K
}

local let floor: Int = 1

local let limit: String = "limit"

local type P(e: Ev) = match e.k {
  num => String
  mode => Rounding
}

local record R {
  ev: Ev
  p: P(ev)
  byP: {P(ev): Int} = {}
}

local let keyed: R = { ev: { k: mode }, p: floor, byP: { floor: 1, limit: 2 } }

local fn values(max: String) -> [R] {
  return [
    { ev: { k: mode }, p: floor },
    { ev: { k: num }, p: limit },
    { ev: { k: num }, p: min },
    { ev: { k: num }, p: max },
  ]
}
`

// TYPES.md §4.1, §4.2: a branch's name stays symbolic unless a local has it; a package value wins, a built-in never.
func TestDependentNameResolution(t *testing.T) {
	prog, f := checkSource(t, namesSource)
	want := map[string]string{"floor": "symbol", "limit": "Let", "min": "symbol", "max": "Param"}
	for _, id := range nodesOf[*syntax.IdentExpr](f) {
		w, ok := want[id.Name]
		if !ok {
			continue
		}
		got := "nothing"
		switch o := prog.Info.Uses[id]; {
		case o != nil:
			got = o.Kind().String()
		case prog.Info.Symbols[id]:
			got = "symbol"
		}
		if got != w {
			t.Errorf("%s resolves to %s, want %s", id.Name, got, w)
		}
	}
	for _, fi := range nodesOf[*syntax.FieldItem](f) {
		o := prog.Info.NameUses[fi.Name]
		switch {
		case fi.Name.Name == "floor" && o != nil:
			t.Errorf("the key floor names %v, want the branch member (symbolic)", o.Kind())
		case fi.Name.Name == "limit" && (o == nil || o.Kind() != check.ObjLet):
			t.Errorf("the key limit names %v, want the let", o)
		}
	}
}
