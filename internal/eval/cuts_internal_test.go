package eval

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const cutsSource = `/// A.
package a

/// Pick.
record Pick {
  /// N.
  n: Int
}

/// A pick built n calls down.
fn down(n: Int) -> Pick {
  if n == 0 {
    return { n: 0 }
  }
  return down(n - 1)
}

/// Built deep, outside any precomputation.
let deep: Pick = down(40)

/// Built deep, precomputed.
export fn bad() -> Pick { return down(40) }
`

// DECISIONS 324: only a precomputation notes its cut stacks, until forgotten; elsewhere, nothing.
func TestCutsBounded(t *testing.T) {
	ctx := context.Background()
	fs := &source.FileSet{}
	src, err := fs.Add("a/a.canon", "/a/a.canon", []byte(cutsSource))
	if err != nil {
		t.Fatal(err)
	}
	files := []*syntax.File{syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, ""))}
	bags := check.Bags{}
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, NewFolder(bags, Options{}))
	ev := New(prog, nil, bags, Options{})
	if _, ok := ev.Force(ctx, Root{Pkg: "a", Name: "deep"}); !ok || len(ev.cuts) != 0 {
		t.Fatalf("deep: forced %t, %d cut stacks noted, want none", ok, len(ev.cuts))
	}
	fn := declNamed(t, prog, "bad")
	if _, ok := ev.Call(ctx, fn, nil, nil); !ok || len(ev.cuts) == 0 {
		t.Fatalf("bad: called %t, %d cut stacks noted, want some", ok, len(ev.cuts))
	}
	for _, top := range ev.cuts { //canon:unordered every one is checked
		if top != CallFrame(fn, nil, nil) {
			t.Errorf("a cut stack's outermost frame %v, want the precomputation's", top)
		}
	}
	ev.ForgetCuts()
	if len(ev.cuts) != 0 {
		t.Errorf("%d cut stacks kept after ForgetCuts", len(ev.cuts))
	}
}

// declNamed is the declaration name of the program's packages.
func declNamed(t *testing.T, prog *check.Program, name string) check.Object {
	t.Helper()
	for _, cp := range prog.Packages {
		for _, o := range cp.Decls {
			if o.Name() == name {
				return o
			}
		}
	}
	t.Fatalf("no declaration %s", name)
	return nil
}
