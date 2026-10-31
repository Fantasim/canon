package edit_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// shelvesLaw keys two instances of one record field's collection by the same key, and reads them
// through a let root with literal indices and through a function parameter.
const shelvesLaw = `/// P.
package p

/// Named.
record Named {
  /// Name.
  name: String
  /// N.
  n: Int
}

/// Shelf.
record Shelf {
  /// Named.
  named: [Named] keyed by name = []
}

/// Shelves.
let shelves: [Shelf] = [
  { named: [{ name: "a", n: 1 }, { name: "b", n: 2 }] },
  { named: [{ name: "a", n: 3 }] },
]

/// From the first.
let fromFirst: Int = shelves[0].named.a.n

/// From the second.
let fromSecond: Int = shelves[1].named.a.n

/// Through a parameter.
fn nOf(s: Shelf) -> Int {
  return s.named.a.n
}
`

// API.md R7, E11 (log-2026-09-29 M4 Cleanup-A-r): a key lookup through a record field's collection
// names an element in code only when its receiver is that element's own instance, read from a let
// root by literal indices and keys; a parameter's lookup names none.
func TestFieldCollectionKeyRefs(t *testing.T) {
	fsys := mapFS{"law/project.canon": file(projectCanon), "law/p/p.canon": file(shelvesLaw)}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	f := analyze(t, p, []string{"p"}, true)
	for _, c := range []struct{ path, want, not string }{
		{"p:shelves[0].named[a]", "fromFirst", "fromSecond"},
		{"p:shelves[1].named[a]", "fromSecond", "fromFirst"},
	} {
		refs, err := f.Refs(context.Background(), resolve(t, f, c.path))
		if err != nil {
			t.Fatal(err)
		}
		code := codeLines(f, refs)
		if len(code) != 1 || !strings.Contains(code[0], "p/p.canon:"+lineOf(shelvesLaw, c.want)) {
			t.Errorf("API.md R7 %s: code refs %q, want only the lookup of %s", c.path, code, c.want)
		}
	}
	s := open(t, fsys, nil, "", "p")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpRename, Path: "shelves[0].named[a]", Key: edit.Key("z")}}})
	if err != nil {
		t.Fatal(err)
	}
	after := string(edit.Writes(plan, "p/p.canon")[0].After)
	for _, want := range []string{"shelves[0].named.z.n", "shelves[1].named.a.n", "return s.named.a.n"} {
		if !strings.Contains(after, want) {
			t.Errorf("API.md E11: the rename of instance 0's element does not leave %q:\n%s", want, after)
		}
	}
}

// codeLines are the code references among refs, as refLines writes them.
func codeLines(f fixture, refs []edit.Ref) []string {
	var out []string
	for i, l := range refLines(f, refs) {
		if refs[i].Kind == edit.RefCode {
			out = append(out, l)
		}
	}
	return out
}

// lineOf is the 1-based line of src holding `let <name>`, as text.
func lineOf(src, name string) string {
	for i, l := range strings.Split(src, "\n") {
		if strings.HasPrefix(l, "let "+name) {
			return strconv.Itoa(i + 1)
		}
	}
	return ""
}

// spreadLaw builds other from arena by a spread, so other.mobs holds arena's elements.
const spreadLaw = `package a

local record Mob {
  id: Int
}

local record Arena {
  mobs: [Mob] keyed by id
  boss: ref Mob
}

local let arena: Arena = { mobs: [{ id: 1 }, { id: 2 }], boss: 2 }
local let other: Arena = { ...arena, boss: 1 }
`

// spreadFixture is spreadLaw analyzed, clean.
func spreadFixture(t *testing.T) fixture {
	t.Helper()
	p, err := build.Open(mapFS{"law/project.canon": file(projectCanon), "law/a/a.canon": file(spreadLaw)}, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return analyze(t, p, []string{"a"}, true)
}

// API.md R7 (DECISIONS 410): a bound level-1 ref is a ref of the entry its key names in the
// collection its instance's field holds: other.boss is a value ref of arena.mobs[1], not code.
func TestRefsSpreadLevelOne(t *testing.T) {
	checkRefs(t, spreadFixture(t), []refsCase{
		{"a:arena.mobs[1]", []string{"a/a.canon:13 value a:other.boss 1"}},
		{"a:arena.mobs[2]", []string{"a/a.canon:12 value a:arena.boss 2"}},
	})
}
