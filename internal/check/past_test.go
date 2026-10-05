package check_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

const pastSource = `package a

local enum E { a, retired b }

local variant V {
  one
  two
}

local record Item {
  n: Int = 1
}

local let items: table Item = {
  first {}
}

local type Alias = E

local let plain: past E = a
local let list: [past E] = [a]
local let keyed: {past E: Int} = { a: 1 }
local let maybe: past E? = none
local let ref1: past ref items = items.first
local let refs: [past ref items] = [items.first]
local let variant: past V = one
local let aliased: past Alias = a
local let where1: past E where it != b = a
local let wrong: past Int = 1
local let wrongList: past [E] = [a]
`

// TYPES.md §8.4, DECISIONS 304: `past X` is a past layer over X; a refused one keeps X bare.
func TestPastTypeExprs(t *testing.T) {
	prog, f, out := checkFile(t, pastSource)
	code := string(diag.E3024.Def().Code)
	if got := strings.Count(out, "["+code+"]"); got != len(pastRefused) {
		t.Fatalf("%d %s, want %d:\n%s", got, code, len(pastRefused), out)
	}
	for _, d := range f.Decls {
		let, ok := d.(*syntax.LetDecl)
		if !ok || let.Type == nil {
			continue
		}
		name := let.Name.Name
		want, refused := pastRefused[name]
		if !refused {
			want, ok = pastTexts[name]
		}
		if !ok {
			continue
		}
		typ := prog.Info.TypeExprs[let.Type]
		if typ == nil || typ.String() != want {
			t.Errorf("%s: TypeExprs = %v, want %s", name, typ, want)
			continue
		}
		if refused && hasPast(typ) {
			t.Errorf("%s: a refused past kept its layer on %s", name, typ)
		}
	}
}

// pastTexts are the accepted lets of pastSource and the type text TypeExprs records.
var pastTexts = map[string]string{
	"plain":   "past a.E",
	"list":    "[past a.E]",
	"keyed":   "{past a.E: Int}",
	"maybe":   "past a.E?",
	"ref1":    "past ref a.items",
	"refs":    "[past ref a.items]",
	"variant": "past a.V",
	"aliased": "past a.Alias",
	"where1":  "past a.E where it != b",
}

// pastRefused are the lets of pastSource that `past` cannot apply to, and the type kept.
var pastRefused = map[string]string{
	"wrong":     "Int",
	"wrongList": "[a.E]",
}

// hasPast reports a past layer at the top of t, through aliases and refinements.
func hasPast(t types.Type) bool {
	for {
		switch x := t.(type) {
		case *types.Refined:
			if x.Past {
				return true
			}
			t = x.Of
		case *types.Alias:
			t = x.Def
		default:
			return false
		}
	}
}

const pastMatchSource = `package a

local enum K { a, b }

local enum E1 { x, retired y }

local enum E2 { u, v }

local type F(k: K) = past match k {
  a => E1
  b => E2
}

local type G(k: K) = match k {
  a => E1
  b => E2
}

local record Role {
  k: K
  one: F(k)
  two: G(k)
}
`

// TYPES.md §8.4, DECISIONS 304: the arms of a `past match` body are past, a plain match's are not.
func TestPastMatchArms(t *testing.T) {
	prog, f := checkSource(t, pastMatchSource)
	want := map[string]bool{"F": true, "G": false}
	for _, d := range f.Decls {
		td, ok := d.(*syntax.TypeDecl)
		if !ok {
			continue
		}
		dep, ok := prog.Info.Defs[td.Name].Type().(*types.DepUnionType)
		if !ok || len(dep.Fn.Arms) == 0 {
			t.Fatalf("%s: no type function with arms", td.Name.Name)
		}
		for _, arm := range dep.Fn.Arms {
			if hasPast(arm.Result) != want[td.Name.Name] {
				t.Errorf("%s: arm result %s, want past %v", td.Name.Name, arm.Result, want[td.Name.Name])
			}
		}
	}
}
