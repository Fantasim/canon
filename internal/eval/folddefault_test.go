package eval_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// instanceSource has field defaults that read, or ignore, their type's instance argument.
const instanceSource = `/// A.
package a

/// K.
enum K { sub, num }

/// Node.
record Node {
  /// K.
  k: K
  /// Max.
  max: Int = 5
}

/// Nodes.
let nodes: table Node = { a { k: sub }, b { k: num } }

/// Reads its parameter in a default.
record Sub(e: Node) {
  /// The node's max.
  lim: Int = e.max
}

/// Reads no parameter.
record Flat(e: Node) {
  /// One.
  one: Int = 1
}

/// P.
type P(e: Node) = match e.k {
  sub => Sub(e)
  num => Int
}

/// Holder.
record Holder {
  /// Like.
  like: ref nodes
  /// Through a type function's branch.
  d: P(like) = {}
  /// An applied record that reads its argument.
  s: Sub(like) = {}
  /// In a list.
  ps: [P(like)] = [{}]
  /// An applied record that reads no argument.
  f: Flat(like) = {}
  /// An empty list.
  empty: [P(like)] = []
}
`

// TYPES.md §11.1, §15: a default reading an instance-bound parameter does not fold, silently.
func TestFoldInstanceDependentDefault(t *testing.T) {
	b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(instanceSource)}), eval.Options{})
	if out := b.findings(t); out != noFindings {
		t.Fatalf("findings:\n%s", out)
	}
	holder := declNamed(b.checked, "Holder")
	rec, ok := holder.Type().(*types.RecordType)
	if !ok {
		t.Fatalf("Holder is a %T", holder.Type())
	}
	want := map[string]bool{"d": false, "s": false, "ps": false, "f": true, "empty": true}
	for _, f := range rec.Fields {
		exp, listed := want[f.Name]
		if !listed {
			continue
		}
		fold := eval.NewFolder(b.bags, eval.Options{})
		if _, ok := fold.Fold(context.Background(), holder, f.Default, b.checked.Info); ok != exp {
			t.Errorf("%s: folded %v, want %v", f.Name, ok, exp)
		}
		if err := eval.FoldErr(fold); err != nil {
			t.Errorf("%s: internal error %v", f.Name, err)
		}
	}
	if out := b.findings(t); out != noFindings {
		t.Errorf("the folds reported:\n%s", out)
	}
}

// noFindings is the rendered findings of a build that has none.
const noFindings = "0 errors, 0 warnings in 1 package (…)\n"

// declNamed is the top-level declaration named name of the program's first package.
func declNamed(p *check.Program, name string) check.Object {
	for _, d := range p.Packages[0].Decls {
		if d.Name() == name {
			return d
		}
	}
	return nil
}

// selfSource has defaults that read an earlier field through `self`, in a plain and an applied record.
const selfSource = `/// A.
package a

/// A server.
record Server {
  /// A port.
  port: Int = 80
  /// Its name, in an interpolation.
  me: String = "{self.port}"
  /// Twice the port.
  twice: Int = self.port * 2
  /// Twice the port, named bare.
  bare: Int = port * 2
}

/// Ev.
record Ev {
  /// Max.
  max: Int = 3
}

/// Reads its parameter, then itself.
record Lim(e: Ev) {
  /// The max.
  lim: Int = e.max
  /// Its label.
  label: String = "{self.lim}"
}
`

// TYPES.md §15, DECISIONS 150: folding a default that reads its instance, by `self` or bare, stops silently, no internal error.
func TestFoldSelfDefault(t *testing.T) {
	b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(selfSource)}), eval.Options{})
	if out := b.findings(t); out != noFindings {
		t.Fatalf("findings:\n%s", out)
	}
	for _, c := range []struct{ owner, field string }{{"Server", "me"}, {"Server", "twice"}, {"Server", "bare"}, {"Lim", "label"}} {
		owner := declNamed(b.checked, c.owner)
		rec, ok := owner.Type().(*types.RecordType)
		if !ok {
			t.Fatalf("%s is a %T", c.owner, owner.Type())
		}
		i := slices.IndexFunc(rec.Fields, func(f *types.Field) bool { return f.Name == c.field })
		if i < 0 {
			t.Fatalf("%s has no field %s", c.owner, c.field)
		}
		f := rec.Fields[i]
		fold := eval.NewFolder(b.bags, eval.Options{})
		if v, ok := fold.Fold(context.Background(), owner, f.Default, b.checked.Info); ok {
			t.Errorf("%s.%s folded to %v", c.owner, c.field, v)
		}
		if err := eval.FoldErr(fold); err != nil {
			t.Errorf("%s.%s: internal error %v", c.owner, c.field, err)
		}
	}
	if out := b.findings(t); out != noFindings {
		t.Errorf("the folds reported:\n%s", out)
	}
}

// API.md X1, DECISIONS 195: an internal error's detail names the value and the function.
func TestBugNamesItsCause(t *testing.T) {
	b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(foldSource)}), eval.Options{})
	fold := eval.NewFolder(b.bags, eval.Options{})
	untyped := &syntax.IdentExpr{Name: "nowhere"} // a name the checker never resolved
	if _, ok := fold.Fold(context.Background(), b.checked.Packages[0].Decls[0], untyped, b.checked.Info); ok {
		t.Fatal("an unresolved name folded")
	}
	err := eval.FoldErr(fold)
	if err == nil {
		t.Fatal("no internal error")
	}
	for _, want := range []string{"evaluating a.ONE", "met in eval."} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("internal error %q does not hold %q", err, want)
		}
	}
}
