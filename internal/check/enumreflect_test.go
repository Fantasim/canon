package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// STDLIB.md §3, TYPES.md §4.3, DECISIONS 299: E.members is [E] with no Selection; .retired is Bool.
func TestEnumMembersInfo(t *testing.T) {
	prog, f := checkSource(t, `package a

local enum E { a, retired b }

local let xs: [E] = E.members

local let r: Bool = E.a.retired
`)
	for _, s := range nodesOf[*syntax.SelectorExpr](f) {
		name := s.Name.Name
		obj := prog.Info.NameUses[s.Name]
		sel := prog.Info.Selections[s]
		switch name {
		case "members":
			if got := prog.Info.Types[s].String(); got != "[a.E]" {
				t.Errorf("Types[E.members] = %s, want [a.E]", got)
			}
			if obj == nil || obj.Kind() != check.ObjBuiltin || obj.Name() != name || sel != nil {
				t.Errorf("E.members names %v with selection %v, want the built-in members and none", obj, sel)
			}
		case "retired":
			if got := prog.Info.Types[s].String(); got != "Bool" {
				t.Errorf("Types[.retired] = %s, want Bool", got)
			}
			if sel == nil || sel.Kind != check.SelBuiltinMember {
				t.Errorf("Selections[.retired] = %v, want a built-in member", sel)
			}
		}
	}
}

const reflectSource = `package a

local enum K { a, b }

local enum E { x, y }

local type AE = E

local record Ev {
  k: K
}

local let evs: table Ev = {
  one { k: a }
}

local type F(e: Ev) = match e.k {
  a => AE
  _ => K
}

local record Row {
  ev: Ev
  f: F(ev)
}

local let first: ref evs = evs.one
local let all = F(first).members
local let name: String = F(first).typeName
local let enumName: String = E.typeName

local fn of(r: Row) -> String {
  return r.f.name
}
`

// TYPES.md §4.3, §11.4, STDLIB.md §3, DECISIONS 306: what Info records for F(a…).members and `.typeName`, an alias branch included.
func TestTypeFuncReflectionInfo(t *testing.T) {
	prog, f := checkSource(t, reflectSource)
	info := prog.Info
	want := map[string]string{"members": "[a.F(*)]", "typeName": "String"}
	for _, s := range nodesOf[*syntax.SelectorExpr](f) {
		name := s.Name.Name
		if call, isCall := s.X.(*syntax.CallExpr); isCall {
			checkReflection(t, info, s, call, want[name])
			continue
		}
		sel := info.Selections[s]
		switch name {
		case "name":
			if sel == nil || sel.Kind != check.SelBuiltinMember || sel.Recv.String() != "a.F(*)" {
				t.Errorf("Selections[r.f.name] = %+v, want a built-in member on a.F(*)", sel)
			}
		case "typeName":
			obj := info.NameUses[s.Name]
			if info.Types[s] != types.StringType || sel != nil || obj == nil || obj.Kind() != check.ObjBuiltin {
				t.Errorf("E.typeName: %v, %v, %v; want a String naming the built-in, no selection", info.Types[s], sel, obj)
			}
		}
	}
}

// checkReflection checks what Info records for one F(a…).member.
func checkReflection(t *testing.T, info *check.Info, s *syntax.SelectorExpr, call *syntax.CallExpr, want string) {
	t.Helper()
	name := s.Name.Name
	if got := info.Types[s]; got == nil || got.String() != want {
		t.Errorf("Types[F(…).%s] = %v, want %s", name, got, want)
	}
	if obj := info.NameUses[s.Name]; obj == nil || obj.Kind() != check.ObjBuiltin || obj.Name() != name {
		t.Errorf("NameUses[.%s] = %v, want the built-in member", name, obj)
	}
	if info.Selections[s] != nil || info.Calls[call] != nil {
		t.Errorf("F(…).%s has a Selection or a Callee", name)
	}
	if fn := info.Uses[call.Fun.(*syntax.IdentExpr)]; fn == nil || fn.Kind() != check.ObjTypeName || fn.Name() != "F" {
		t.Errorf("Uses[F] = %v, want the type function", fn)
	}
	if got := info.Types[call]; got == nil || got.String() != "a.F(*)" {
		t.Errorf("Types[F(…)] = %v, want a.F(*)", got)
	}
	arg := call.Args[0].Value
	if conv := info.Conv[arg]; conv == nil || conv.Kind != check.ConvDeref {
		t.Errorf("Conv[%T argument] = %+v, want a Deref", arg, conv)
	}
}

const inferredSource = `package a

local enum K { a, retired b }

local record Row {
  kind: past K
  kinds: [past K]
  bounded: past K where it != b
  u: past K | "none"
  m: {past K: [past K]}
}

local let row: Row = Row { kind: a, kinds: [a], bounded: a, u: "none", m: {} }
local let kind = row.kind
local let kinds = row.kinds
local let bounded = row.bounded
local let joined = if true { row.kind } else { K.a }
local let u = row.u
local let m = row.m

local fn pick(k: past K) -> past K {
  return k
}

local fn local1() -> Int {
  let k = row.kind
  let f = pick
  return k.index + f(a).index
}
`

// TYPES.md §8.4, DECISIONS 304: an inferred let or local drops `past`, keeping a real refinement.
func TestInferredDropsPast(t *testing.T) {
	prog, f := checkSource(t, inferredSource)
	want := map[string]string{"kind": "a.K", "kinds": "[a.K]", "bounded": "a.K where it != b", "joined": "a.K", "k": "a.K",
		"u": `a.K | "none"`, "m": "{a.K: [a.K]}", "f": "fn(a.K) -> a.K",
	}
	for _, id := range nodesOf[*syntax.Ident](f) {
		o, ok := prog.Info.Defs[id]
		w, named := want[id.Name]
		if !ok || !named || (o.Kind() != check.ObjLet && o.Kind() != check.ObjLocal) {
			continue
		}
		if got := o.Type().String(); got != w {
			t.Errorf("%s: inferred %s, want %s", id.Name, got, w)
		}
		delete(want, id.Name)
	}
	if len(want) > 0 {
		t.Errorf("not met: %v", want)
	}
}
