package eval_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// methodSrc is a record whose view names its methods: one that computes, one that fails, one
// that loops past any budget, one with a parameter; and an applied record's arguments.
const methodSrc = `package a

record Mode {
  size: Int
}

record Q(m: ref modes) {
  n: Int
}

record P {
  n: Int
  mode: ref modes
  q: Q(mode)

  fn twice(self) -> Int { return n * 2 }

  fn boom(self) -> Int { return n / 0 }

  fn spin(self) -> Int {
    var i = 0
    while i < 100000 { i += 1 }
    return i
  }

  fn scaled(self, k: Int) -> Int { return n * k }
}

view P {
  twice "Twice"
  boom "Boom"
  spin "Spin"
  scaled "Scaled"
}

let modes: table Mode = { m1 { size: 4 } }

let ps: table P = { one { n: 3, mode: m1, q: { n: 1 } } }
`

// methodItems are the view's member items of methodSrc, by name.
func methodItems(p *program) map[string]*syntax.Ident {
	out := map[string]*syntax.Ident{}
	for _, f := range p.files {
		syntax.Inspect(f, func(n syntax.Node) bool {
			if vf, ok := n.(*syntax.ViewField); ok && vf.Name != nil {
				out[vf.Name.Name] = vf.Name
			}
			return true
		})
	}
	return out
}

// VIEWMODEL.md L21, 3.3, X7, API.md V11, V13: a view-named method evaluates on the record shown,
// spends no step and reports nothing; an error, a run past the budget's size, a method with
// parameters, a missing record fail silently.
func TestViewMethod(t *testing.T) {
	const budget = 5000
	p := parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(methodSrc)})
	b := runBuild(t, p, eval.Options{Budget: budget})
	one := b.values[eval.Root{Pkg: "a", Name: "ps"}].(*value.Table).Entries[0]
	items := methodItems(p)
	ctx := context.Background()
	for _, c := range []struct {
		name, want string // want "" fails
		self       *value.Record
		rule       string
	}{
		{"twice", "6", one, "L21: the method's value"},
		{"boom", "", one, "X7: an evaluation error"},
		{"spin", "", one, "API.md V13, DECISIONS 195: past the budget's size, that run alone fails"},
		{"scaled", "", one, "3.3: a method with a parameter"},
		{"twice", "", nil, "X7: no record"},
	} {
		v, ok := b.ev.ViewMethod(ctx, items[c.name], c.self)
		got := ""
		if ok {
			got = v.CanonText()
		}
		if got != c.want || ok != (c.want != "") {
			t.Errorf("%s (%s): %q %t, want %q", c.name, c.rule, got, ok, c.want)
		}
	}
	if b.bags["a"].Summary().Errors != 0 || b.ev.Err() != nil {
		t.Errorf("a view method reported: %s %v", b.findings(t), b.ev.Err())
	}
	if v, ok := b.ev.ViewMethod(ctx, items["twice"], one); !ok || v.CanonText() != "6" {
		t.Errorf("API.md V13: the budget was spent: %v %t", v, ok)
	}
	roomy := runBuild(t, p, eval.Options{})
	self := roomy.values[eval.Root{Pkg: "a", Name: "ps"}].(*value.Table).Entries[0]
	if v, ok := roomy.ev.ViewMethod(ctx, items["spin"], self); !ok || v.CanonText() != "100000" {
		t.Errorf("API.md V13: spin within the default budget: %v %t", v, ok)
	}
}

// brokenSrc is a record whose view names two methods the checker marks broken.
const brokenSrc = `package a

record P {
  n: Int

  fn bad(self) -> Int { return n + "x" }

  fn lost(self) -> Int { return nope(n) }
}

view P {
  bad "Bad"
  lost "Lost"
}

let ps: table P = { one { n: 3 } }
`

// EVALUATION.md §1, X7, V11 (log-2026-09-29 M4 U9b-r): a broken method fails, no internal error.
func TestViewMethodBroken(t *testing.T) {
	p := parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(brokenSrc)})
	b := runBuild(t, p, eval.Options{})
	one := b.values[eval.Root{Pkg: "a", Name: "ps"}].(*value.Table).Entries[0]
	items := methodItems(p)
	for _, name := range []string{"bad", "lost"} {
		if v, ok := b.ev.ViewMethod(context.Background(), items[name], one); ok || b.ev.Err() != nil {
			t.Errorf("%s: %v %t, internal error %v", name, v, ok, b.ev.Err())
		}
	}
}

// TYPES.md 11.1, DECISIONS 147: BoundParams are the arguments an applied record keeps, a copy.
func TestBoundParams(t *testing.T) {
	p := parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(methodSrc)})
	b := runBuild(t, p, eval.Options{})
	one := b.values[eval.Root{Pkg: "a", Name: "ps"}].(*value.Table).Entries[0]
	q := one.Fields[2].(*value.Record)
	param := q.T.Base().(*types.AppliedRecord).Rec.Params[0]
	got := b.ev.BoundParams(q)
	if m, ok := got[param].(*value.Ref); !ok || m.Key.Text() != "m1" {
		t.Fatalf("TYPES.md 11.1: q is bound to %v", got)
	}
	delete(got, param)
	if len(b.ev.BoundParams(q)) != 1 || b.ev.BoundParams(one) != nil {
		t.Errorf("DECISIONS 147: a copy of q's, none for one: %v, %v", b.ev.BoundParams(q), b.ev.BoundParams(one))
	}
}
