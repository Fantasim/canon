package build_test

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
)

const (
	heavyA = `/// A.
package a

import b

/// A use.
record Use {
  /// How many.
  count: Int

  /// Slow.
  fn slow(self) -> Int { return [i for i in 0..40000].sum() }
}

/// Uses.
let uses: table Use = {
  first { count: 20 }
}

view Use {
  title "{id} {slow()}"
  subtitle "{b.heavy}"
}

emit view { out: "@out/a.view.json" }
`
	heavyB  = viewB + "\n/// Slow to settle.\nlet heavy: Int = [i for i in 0..40000].sum()\n"
	budgetC = `/// C.
package c

/// A thing.
record Thing {
  /// Its size.
  size: Int
}

/// Things.
let things: table Thing = {
  one { size: 1 }
}

/// Heavy.
let heavy: Int = [i for i in 0..100000].sum()

view Thing {
  title "{id}"
}

emit view { out: "@out/c.view.json" }
`
	budgetProject = "project acme {\n  canon: \"0.1\"\n  budget: 5000\n  roots {\n    out: \"out\"\n  }\n}\n"
	keysK         = `/// K.
package k

/// A shape.
variant Shape {
  circle {
    /// Its radius.
    r: Int

    warn wide: r < 10 else "radius {r}"
  }
  dot

  warn round: self.kind == dot else "not a dot"
}

/// Shapes.
let shapes: [Shape] = [circle { r: 20 }]

/// Its size.
let size: Int = 3

warn small: size > 5 else "size {size}"

emit view { out: "@out/k.view.json" }
`
	keysFr = "package k\ntranslation fr\n\nShape.circle.check.wide \"rayon {r}\"\nShape.check.round \"pas un point\"\ncheck.small \"taille {size}\"\n"
)

// flipCtx is a context whose Err turns Canceled after n calls: a call cancelled mid-way.
type flipCtx struct {
	context.Context
	n, calls int64
}

func (c *flipCtx) Err() error {
	if atomic.AddInt64(&c.calls, 1) > c.n {
		return context.Canceled
	}
	return nil
}

func (c *flipCtx) Done() <-chan struct{}       { return nil }
func (c *flipCtx) Deadline() (time.Time, bool) { return time.Time{}, false }

// modelBytes is Analysis.ViewModel(pkg) written, failing on an error.
func modelBytes(t *testing.T, a *build.Analysis, ctx context.Context, pkg string) []byte {
	t.Helper()
	m, err := a.ViewModel(ctx, pkg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := viewgen.Write(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// VIEWMODEL.md J5, API.md R9: a cancelled ViewModel call fails that call only; the same Analysis then gives a fresh one's bytes.
func TestViewModelAfterCancel(t *testing.T) {
	p, err := build.Open(mapFS{"p/project.canon": file(viewProject), "p/a/a.canon": file(heavyA), "p/b/b.canon": file(heavyB)}, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	want := modelBytes(t, fresh, context.Background(), "a")
	const calls, stride = 60, 3
	for n := int64(1); n < calls; n += stride {
		a, err := p.Analyze(context.Background(), []string{"a"})
		if err != nil {
			t.Fatal(err)
		}
		_, cerr := a.ViewModel(&flipCtx{Context: context.Background(), n: n}, "a")
		if got := modelBytes(t, a, context.Background(), "a"); !bytes.Equal(got, want) {
			t.Fatalf("cancelled after %d looks (%v): the next call differs\n%s\nwant\n%s", n, cerr, got, want)
		}
	}
}

// EVALUATION.md §7.2, API.md R6: what a view model evaluates aside leaves Analysis.Cause as it was.
func TestViewModelKeepsCauses(t *testing.T) {
	fsys := viewFS()
	fsys["p/a/a.canon"] = file(strings.Replace(viewA, `subtitle "{b.label}"`, `subtitle "{b.unread}"`, 1))
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	root := eval.Root{Pkg: "b", Name: "unread"}
	before := causeOf(t, a, root)
	modelBytes(t, a, context.Background(), "a")
	if after := causeOf(t, a, root); !reflect.DeepEqual(before, after) {
		t.Errorf("Cause(b.unread) was %v, is %v after the view model", before, after)
	}
}

// EVALUATION.md §12.2, §1 phase 8: once stage A exhausts the budget, the view model still renders the titles of what was settled.
func TestViewModelAfterBudget(t *testing.T) {
	res := buildTree(t, mapFS{"p/project.canon": file(budgetProject), "p/c/c.canon": file(budgetC)}, build.BuildOptions{Targets: viewOnly})
	if !slices.Contains(codes(res.List), diag.E4401.Def().Code) {
		t.Fatalf("the budget held: %v", codes(res.List))
	}
	m := decodeModel(t, viewOutput(t, res, "@out/c.view.json").Content)
	if got := titles(m.Search["c:things"]); !maps.Equal(got, map[string]string{"one": "one"}) {
		t.Errorf("titles %v", got)
	}
}

// EVALUATION.md §2.1, API.md R4: Analysis.Force answers for what Analyze settled, not for what a view model evaluated since.
func TestAnalysisForceIsASnapshot(t *testing.T) {
	src := strings.Replace(budgetC, "view Thing", "/// Fine, past the budget.\nlet zfine: Int = 7\n\nview Thing", 1)
	p, err := build.Open(mapFS{"p/project.canon": file(budgetProject), "p/c/c.canon": file(src)}, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"c"})
	if err != nil {
		t.Fatal(err)
	}
	zfine, things := eval.Root{Pkg: "c", Name: "zfine"}, eval.Root{Pkg: "c", Name: "things"}
	if _, ok := a.Force(zfine); ok {
		t.Fatal("stage A settled c.zfine past the budget")
	}
	m := decodeModel(t, modelBytes(t, a, context.Background(), "c"))
	if _, ok := a.Force(zfine); ok || m.Values["c:zfine"].Failed {
		t.Errorf("after the view model: Force(c.zfine) %t, model failed %t", ok, m.Values["c:zfine"].Failed)
	}
	if _, ok := a.Force(things); !ok {
		t.Error("Force(c.things) lost")
	}
}

// DECISIONS 196, EVALUATION.md §1 phase 8: an unsupported load met only by the view model is ErrLoad, not hidden.
func TestViewModelLoadError(t *testing.T) {
	fsys := viewFS()
	fsys["p/b/b.canon"] = file(viewB + "\n/// Loaded.\nlet loaded: [Int] = load.dir(\"x.txt\")\n")
	fsys["p/b/x.txt"] = file("1")
	fsys["p/a/a.canon"] = file(strings.Replace(viewA, `subtitle "{b.label}"`, `subtitle "{b.loaded}"`, 1))
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"a"}, Targets: viewOnly}); !errors.Is(err, build.ErrLoad) {
		t.Errorf("err %v, want ErrLoad", err)
	}
}

// VIEWMODEL.md J15, I18N.md §3.3: the keys `V.c.check.n` (a case's check), `V.check.n` (a variant-level one) and `check.n` (a package check).
func TestViewModelMessageKeys(t *testing.T) {
	fsys := mapFS{"p/project.canon": file(viewProject), "p/k/k.canon": file(keysK), "p/k/k.fr.canon": file(keysFr)}
	m := decodeModel(t, viewOutput(t, buildTree(t, fsys, build.BuildOptions{Targets: viewOnly}), "@out/k.view.json").Content)
	want := map[string]string{"wide": "rayon 20", "round": "pas un point", "small": "taille 3"}
	got := map[string]string{}
	for _, f := range m.Findings {
		if f.Check != "" {
			got[f.Check] = f.Messages["fr"]
		}
	}
	if !maps.Equal(got, want) {
		t.Errorf("messages %v, want %v (findings %+v)", got, want, m.Findings)
	}
}
