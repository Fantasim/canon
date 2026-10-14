package eval

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

const lambdaStackSource = `/// A.
package a

/// Pick.
record Pick {
  /// N.
  n: Int
}

/// n calls down, a pick built in the fn, then picks built in a lambda.
fn down(n: Int) -> [Pick] {
  if n == 0 {
    let p = Pick { n: 7 }
    return [p] + [1, 2].map(k => Pick { n: k })
  }
  return down(n - 1)
}

/// Shallow: 5 frames.
export fn shallow() -> [Pick] { return down(3) }

/// Deep: 32 frames, cut.
export fn deep() -> [Pick] { return down(30) }
`

// EVALUATION.md §13 (stack), DECISIONS 324: a value a lambda builds carries its fn's stack, cut alike.
func TestLambdaValueStack(t *testing.T) {
	cases := []struct {
		fn         string
		len, more  int
		hiddenCall bool // the precomputation's frame is among those cut
	}{
		{"shallow", 5, 0, false},
		{"deep", diag.MaxStackFrames, 32 - diag.MaxStackFrames, true},
	}
	prog, ev := lambdaStackProgram(t)
	for _, c := range cases {
		fn := declNamed(t, prog, c.fn)
		v, ok := ev.Call(context.Background(), fn, nil, nil)
		l, isList := v.(*value.List)
		if !ok || !isList || len(l.Elems) != 3 {
			t.Fatalf("%s: %v, %t", c.fn, v, ok)
		}
		in := l.Elems[0].Prov()
		f := CallFrame(fn, nil, nil)
		for _, e := range l.Elems[1:] {
			p := e.Prov()
			if len(p.Stack) != c.len || p.MoreFrames != c.more || !slices.Equal(p.Stack, in.Stack) {
				t.Errorf("%s: %s has %d frames (%d more), want %d (%d more), its fn's %v", c.fn, e.CanonText(), len(p.Stack), p.MoreFrames, c.len, c.more, in.Stack)
			}
			if got := ev.Hides(p.Stack, &f); got != c.hiddenCall {
				t.Errorf("%s: %s hides the call frame: %t, want %t", c.fn, e.CanonText(), got, c.hiddenCall)
			}
		}
		ev.ForgetCuts()
	}
}

// lambdaStackProgram is lambdaStackSource checked, and its evaluator.
func lambdaStackProgram(t *testing.T) (*check.Program, *Evaluator) {
	t.Helper()
	ctx := context.Background()
	fs := &source.FileSet{}
	src, err := fs.Add("a/a.canon", "/a/a.canon", []byte(lambdaStackSource))
	if err != nil {
		t.Fatal(err)
	}
	files := []*syntax.File{syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, ""))}
	bags := check.Bags{}
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, NewFolder(bags, Options{}))
	return prog, New(prog, nil, bags, Options{})
}
