package eval_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	viewSrcA = `package a

import b

/// A point of a table, a map and a list.
record P {
  /// Its size.
  n: Int

  /// Twice its size.
  fn twice(self) -> Int { return n * 2 }
}

/// Every position a P can take.
let ps: table P = { one { n: 1 } }
let byKey: {String: P} = { "k": { n: 3 } }
let listed: [P] = [{ n: 4 }]
let base: Int = 10

view P {
  title "{id}|{key}|{index}|{twice()}|{n}|{self.n}|{base}|{b.val}|{b.boom}|{n / 0}"
}
`
	viewSrcB  = "package b\n\n/// Read by nothing in stage A.\nlet val: Int = 5\n\n/// Fails when forced.\nlet boom: Int = 1 / 0\n"
	viewSrcFr = "package a\ntranslation fr\n\nP.title \"fr {n}\"\n"
)

// viewBuild checks a, b and a's French translation, then evaluates a only (EVALUATION.md §2.1).
func viewBuild(t *testing.T, opt eval.Options) (*build, map[string]syntax.Expr) {
	t.Helper()
	p := parseFiles(t, []string{"a/a.canon", "a/a.fr.canon", "b/b.canon"}, [][]byte{[]byte(viewSrcA), []byte(viewSrcFr), []byte(viewSrcB)})
	proj := project.New("t", project.Version{Minor: 1})
	proj.Languages = append(proj.Languages, "fr")
	ctx := context.Background()
	b := &build{prog: p, bags: check.Bags{}, values: map[eval.Root]value.Value{}}
	b.checked = check.Check(ctx, proj, p.files, b.bags, eval.NewFolder(b.bags, opt))
	b.ev = eval.New(b.checked, &host{}, b.bags, opt)
	for _, name := range []string{"ps", "byKey", "listed", "base"} {
		v, ok := b.ev.Force(ctx, eval.Root{Pkg: "a", Name: name})
		if !ok {
			t.Fatalf("a.%s: %s", name, b.findings(t))
		}
		b.values[eval.Root{Pkg: "a", Name: name}] = v
	}
	return b, interpolations(p.files)
}

// interpolations are the interpolated expressions of files, by "<file>:<source text>".
func interpolations(files []*syntax.File) map[string]syntax.Expr {
	out := map[string]syntax.Expr{}
	for _, f := range files {
		syntax.Inspect(f, func(n syntax.Node) bool {
			if in, ok := n.(*syntax.Interp); ok {
				sp := f.Span(in.X)
				out[f.Src.Path+":"+string(f.Src.Content[sp.Start:sp.End])] = in.X
			}
			return true
		})
	}
	return out
}

// VIEWMODEL.md §3.4, G12, X7, API.md V11, EVALUATION.md §2.1: scope, magic names, silent failures.
func TestViewScope(t *testing.T) {
	b, xs := viewBuild(t, eval.Options{})
	ctx := context.Background()
	one := b.values[eval.Root{Pkg: "a", Name: "ps"}].(*value.Table).Entries[0]
	str := func(s string) value.Value { return &value.Str{V: s, T: types.StringType} }
	all := eval.Magic{ID: str("one"), Key: str("k"), Index: &value.Int{V: 1, T: types.IntType}}
	for _, c := range []struct {
		x     string
		magic eval.Magic
		want  string // "" fails
		rule  string
	}{
		{"a/a.canon:id", all, "one", "§3.4 1: id of a table entry"},
		{"a/a.canon:key", all, "k", "§3.4 1: key of a map value"},
		{"a/a.canon:index", all, "1", "§3.4 1: index of a list element"},
		{"a/a.canon:key", eval.Magic{ID: str("one")}, "", "G12: key without a value"},
		{"a/a.canon:index", eval.Magic{}, "", "G12: index without a value"},
		{"a/a.canon:twice()", all, "2", "§3.4 2: a method of the target"},
		{"a/a.canon:n", all, "1", "§3.4 2: a field"},
		{"a/a.canon:self.n", all, "1", "§3.4 2: self"},
		{"a/a.canon:base", all, "10", "§3.4 3: the package scope"},
		{"a/a.canon:b.val", all, "5", "EVALUATION.md §1 phase 8: an import's value stage A left alone, evaluated aside"},
		{"a/a.canon:b.boom", all, "", "X7: an import's value that fails, evaluated aside"},
		{"a/a.canon:n / 0", all, "", "X7: an evaluation error"},
		{"a/a.fr.canon:n", all, "1", "I18N.md T1: a translation's interpolation, in the source item's scope"},
	} {
		x := xs[c.x]
		if x == nil {
			t.Fatalf("%s: no such interpolation in %v\n%s", c.x, xs, b.findings(t))
		}
		v, ok := b.ev.View(ctx, x, one, c.magic)
		got := ""
		if ok {
			got = v.CanonText()
		}
		if got != c.want || ok != (c.want != "") {
			t.Errorf("%s (%s): %q %t, want %q", c.x, c.rule, got, ok, c.want)
		}
	}
	if b.bags["a"].Summary().Errors != 0 || b.bags["b"].Summary().Errors != 0 || b.ev.Err() != nil {
		t.Errorf("a view run reported: %s %v", b.findings(t), b.ev.Err())
	}
}

// VIEWMODEL.md C3, EVALUATION.md §1 phase 8: ForceAside settles into its own bags; View then reads it.
func TestForceAside(t *testing.T) {
	b, xs := viewBuild(t, eval.Options{})
	ctx := context.Background()
	scratch := check.Bags{"b": diag.NewBag(b.prog.fs, "b")}
	if v, ok := b.ev.ForceAside(ctx, eval.Root{Pkg: "b", Name: "val"}, scratch); !ok || v.CanonText() != "5" {
		t.Fatalf("b.val = %v %t", v, ok)
	}
	if _, ok := b.ev.ForceAside(ctx, eval.Root{Pkg: "b", Name: "boom"}, scratch); ok {
		t.Error("b.boom settled")
	}
	if scratch["b"].Summary().Errors != 1 || b.bags["b"].Summary().Errors != 0 {
		t.Errorf("findings: aside %d, build %d", scratch["b"].Summary().Errors, b.bags["b"].Summary().Errors)
	}
	one := b.values[eval.Root{Pkg: "a", Name: "ps"}].(*value.Table).Entries[0]
	if v, ok := b.ev.View(ctx, xs["a/a.canon:b.val"], one, eval.Magic{}); !ok || v.CanonText() != "5" {
		t.Errorf("b.val after ForceAside: %v %t", v, ok)
	}
}

// DECISIONS 195, API.md V11: a view run that meets an internal error fails, and the error stays for the caller (Err).
func TestViewKeepsInternalErrors(t *testing.T) {
	b, xs := viewBuild(t, eval.Options{})
	if _, ok := b.ev.View(context.Background(), xs["a/a.canon:n"], nil, eval.Magic{}); ok {
		t.Error("a field of no instance rendered")
	}
	if b.ev.Err() == nil {
		t.Error("the internal error vanished")
	}
}

// EVALUATION.md §12.2, DECISIONS 148, VIEWMODEL.md J5: view runs spend none of the project budget.
func TestViewSpendsNoStep(t *testing.T) {
	const budget, renders = 400, 1000
	b, xs := viewBuild(t, eval.Options{Budget: budget})
	ctx := context.Background()
	one := b.values[eval.Root{Pkg: "a", Name: "ps"}].(*value.Table).Entries[0]
	for i := range renders {
		if v, ok := b.ev.View(ctx, xs["a/a.canon:twice()"], one, eval.Magic{}); !ok || v.CanonText() != "2" {
			t.Fatalf("render %d: %v %t", i, v, ok)
		}
	}
	if _, ok := b.ev.Force(ctx, eval.Root{Pkg: "b", Name: "val"}); !ok || b.bags["a"].Summary().Errors != 0 {
		t.Errorf("the budget was spent: %s", b.findings(t))
	}
}
