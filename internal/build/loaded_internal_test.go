package build

import (
	"context"
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	loadedCase  = "testdata/incremental/loaded.txtar"
	loadedText  = 1 << 12 // more than any value of the case writes
	loadedCap   = 5000    // a budget past the cold need: the sweep fails rather than loop on
	loadedTwice = 2       // the sweep goes on to this many times the cold need
)

// loadedRoots are the case's values: a's, which b loads without selecting, and b's own.
var loadedRoots = []eval.Root{{Pkg: "a", Name: "things"}, {Pkg: "a", Name: "lonely"}, {Pkg: "a", Name: "heavy"}, {Pkg: "b", Name: "empty"}, {Pkg: "b", Name: "some"}}

// API.md V13, VIEWMODEL.md C3 (log-2026-09-29 M4 P14): an analysis of b alone settles what it
// selects; a value of a, which it loads without selecting, is the one an analysis of every
// package settles, forced aside when b never forced it, and never a finding of the analysis.
func TestLoadedAside(t *testing.T) {
	z := archiveAnalyzer(t, loadedCase)
	all, b := loadedPair(t, z)
	before := len(b.Result().List)
	for _, root := range loadedRoots {
		if want, got := loadedView(all, root, false), loadedView(b, root, true); want != got {
			t.Errorf("%v: loaded %s, every package's %s", root, got, want)
		}
	}
	if _, ok := b.Force(eval.Root{Pkg: "a", Name: "lonely"}); ok {
		t.Error("Force settles a package the analysis did not select")
	}
	if _, ok := b.Loaded(context.Background(), eval.Root{Pkg: "zz", Name: "x"}); ok {
		t.Error("Loaded gives a value of a package the analysis did not load")
	}
	if len(b.Result().List) != before || b.ViewErr() != nil {
		t.Errorf("forcing aside added a finding or a failure: %v", b.ViewErr())
	}
}

// DECISIONS 244, API.md V13 (log-2026-09-29 M4 P14-r): at every budget from 1 to twice the cold
// need, when Covers says b's analysis gives every package's values, its values, provenance,
// invalid marks and bound arguments are every package's.
func TestLoadedBudgetSweep(t *testing.T) {
	need, covered := 0, 0
	for budget := 1; need == 0 || budget <= loadedTwice*need; budget++ {
		if budget > loadedCap {
			t.Fatalf("no budget up to %d holds the case", loadedCap)
		}
		all, b := loadedPair(t, withBudget(t, loadedCase, budget))
		if need == 0 && charged(all) < int64(budget) {
			need = budget
		}
		if !Covers(all, b) {
			continue
		}
		covered++
		for _, root := range loadedRoots {
			if want, got := loadedView(all, root, false), loadedView(b, root, true); want != got {
				t.Errorf("budget %d, %v: loaded %s, every package's %s", budget, root, got, want)
			}
		}
	}
	if covered == 0 {
		t.Error("Covers held at no budget")
	}
}

// loadedPair is the analysis of every package of z's case and the one of b alone.
func loadedPair(t *testing.T, z *analyzer) (all, b *Analysis) {
	t.Helper()
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if all, err = p.Analyze(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if b, err = p.Analyze(ctx, []string{"b"}); err != nil {
		t.Fatal(err)
	}
	return all, b
}

// loadedView is root's value in a, as Force or Loaded gives it: its text, provenance, whether it
// is marked invalid, and the arguments a record keeps bound.
func loadedView(a *Analysis, root eval.Root, loaded bool) string {
	v, ok := a.Force(root)
	if loaded {
		v, ok = a.Loaded(context.Background(), root)
	}
	if !ok {
		return "none"
	}
	out := fmt.Sprintf("%s %s invalid=%v", value.TextUpTo(v, loadedText), provText(v.Prov()), a.r.ev.Invalid(v))
	if rec, isRec := v.(*value.Record); isRec {
		out += fmt.Sprintf(" bound=%d", len(a.ViewBound()(rec)))
	}
	return out
}

// provText is p's kind, place, pointer and layer, and its Via's.
func provText(p *value.Prov) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("{%d %d-%d %s %s via %s}", p.Kind, p.Span.Start, p.Span.End, p.Pointer, p.Layer, provText(p.Via))
}
