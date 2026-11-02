package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/types"
)

// reachProject has a record for each way a type is reached: Loaded by a load, Emitted by an emit json,
// Texted by a @text fn, Through only through a dependent field and a map of a loaded record, Viaref only
// through a ref, and Plain by nothing.
var reachProject = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon": file(`package a

enum Kind { x, y }

record Through { n: Int = 0 }

record Branch { n: Int = 0 }

type Body(k: Kind) = match k {
  x => Through
  y => Int
}

record Loaded {
  kind: Kind
  body: Body(kind)
  by: {String: Branch}
}

record Emitted { n: Int = 0 }

record Texted { n: Int = 0 }

record Viaref { n: Int = 0 }

record Plain { n: Int = 0 }

let viarefs: table Viaref = {}

record Holder { to: ref viarefs }

let loaded: table Loaded = load("loaded.json")

let emitted: [Emitted] = []

let holders: [Holder] = []

let plain: [Plain] = []

emit json { out: "out/emitted.json", values: [emitted, holders] }

@text("texted.json")
export fn texted() -> Texted {
  return Texted {}
}
`),
	"law/a/loaded.json": file("{}\n"),
}

// recordNamed is the type of the declaration called name in package a.
func recordNamed(t *testing.T, s session, name string) types.Type {
	t.Helper()
	for _, pkg := range s.a.Program().Packages {
		for _, obj := range pkg.Decls {
			if obj.Name() == name {
				return obj.Type()
			}
		}
	}
	t.Fatalf("no declaration %s", name)
	return nil
}

// API.md E34 and §5.7: WireReach, OutputReach and TextReach reach a record through a dependent type, a map, an emit and a @text result, and never through a ref.
func TestReachExports(t *testing.T) {
	s := open(t, reachProject, nil, "", "a")
	wire, output, text := s.snap.WireReach(), s.snap.OutputReach(), s.snap.TextReach()
	cases := []struct {
		name               string
		wire, output, text bool
	}{
		{"Loaded", true, false, false},
		{"Through", true, false, false},
		{"Branch", true, false, false},
		{"Emitted", true, true, false},
		{"Texted", true, true, true},
		{"Viaref", false, false, false},
		{"Plain", false, false, false},
	}
	for _, c := range cases {
		typ := recordNamed(t, s, c.name)
		for _, r := range []struct {
			set  edit.Reached
			want bool
			kind string
		}{{wire, c.wire, "WireReach"}, {output, c.output, "OutputReach"}, {text, c.text, "TextReach"}} {
			if got := r.set.Has(typ); got != r.want {
				t.Errorf("API.md E34, %s in %s: %v, want %v", c.name, r.kind, got, r.want)
			}
		}
	}
}

// API.md §5.7: the first origin that reaches a record is an emit before a @text fn, named by package and target or fn.
func TestReachOrigin(t *testing.T) {
	s := open(t, reachProject, nil, "", "a")
	want := map[string]edit.Origin{
		"Emitted": {Pkg: "a", Name: "json"},
		"Texted":  {Pkg: "a", Name: "texted", Text: true},
		"Loaded":  {Pkg: "a", Load: true},
	}
	reached := s.snap.WireReach()
	for name, o := range want { //canon:unordered each case alone
		if got, ok := reached.From(recordNamed(t, s, name)); !ok || got != o {
			t.Errorf("API.md §5.7, %s: origin %+v, want %+v", name, got, o)
		}
	}
}
