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
)

// readsSource is a package whose values read one another: mid reads base, top reads mid only,
// twice reads base twice, apart reads nothing.
const readsSource = `/// A.
package a

/// Base.
let base: [Int] = [1, 2]

/// Mid.
let mid: [Int] = base

/// Top.
let top: Int = mid.len()

/// Twice.
let twice: Int = base.len() + base.len()

/// Apart.
let apart: Int = 3
`

// readsEvaluator is an evaluator of readsSource, with a memo when memo is true.
func readsEvaluator(t *testing.T, memo bool) *Evaluator {
	t.Helper()
	fs := &source.FileSet{}
	src, err := fs.Add("a/a.canon", "/a/a.canon", []byte(readsSource))
	if err != nil {
		t.Fatal(err)
	}
	bags := check.Bags{}
	files := []*syntax.File{syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, ""))}
	prog := check.Check(context.Background(), project.New("demo", project.Version{Minor: 1}), files, bags, NewFolder(bags, Options{}))
	ev := New(prog, nil, bags, Options{})
	if memo {
		ev.UseMemo(NewMemo(), 1)
	}
	return ev
}

// log-2026-09-29 M4 P18: the values whose evaluation read a value, directly, each once, in
// first-read order; a run charged to a value (stage B's) reads for it; none noted without a memo.
func TestReadBy(t *testing.T) {
	ctx := context.Background()
	ev := readsEvaluator(t, true)
	for _, name := range []string{"top", "twice", "apart"} {
		if _, ok := ev.Force(ctx, Root{Pkg: "a", Name: name}); !ok {
			t.Fatalf("%s did not complete", name)
		}
	}
	r := ev.newRun(ctx, charge{pkg: "a", name: "apart"}, nil)
	ev.force(ctx, ev.roots[Root{Pkg: "a", Name: "mid"}], r, nil)
	for _, c := range []struct {
		name string
		want []string
	}{
		{"base", []string{"mid", "twice"}},
		{"mid", []string{"top", "apart"}},
		{"top", nil},
		{"twice", nil},
		{"apart", nil},
	} {
		by, noted := ev.ReadBy(Root{Pkg: "a", Name: c.name})
		var got []string
		for _, root := range by {
			got = append(got, root.Name)
		}
		if !noted || !slices.Equal(got, c.want) {
			t.Errorf("%s: read by %v (noted %t), want %v", c.name, got, noted, c.want)
		}
	}
	plain := readsEvaluator(t, false)
	plain.Force(ctx, Root{Pkg: "a", Name: "top"})
	if _, noted := plain.ReadBy(Root{Pkg: "a", Name: "mid"}); noted {
		t.Error("without a memo: reads noted")
	}
}
