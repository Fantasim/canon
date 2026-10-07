package eval_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// perPackageCase is E4401_7: a spends its budget, b reads nothing of a, c reads two values of a.
const perPackageCase = "testdata/findings/E4401_7.txtar"

var (
	aBig, aSmall, aAfter = eval.Root{Pkg: "a", Name: "big"}, eval.Root{Pkg: "a", Name: "small"}, eval.Root{Pkg: "a", Name: "after"}
	bWide                = eval.Root{Pkg: "b", Name: "wide"}
	cViaBig, cViaAfter   = eval.Root{Pkg: "c", Name: "viaBig"}, eval.Root{Pkg: "c", Name: "viaAfter"}
	cOwn                 = eval.Root{Pkg: "c", Name: "own"}
)

// perPackageRuns are orders and selections of the case: roots forced first, packages selected
// (every one when none). c reads a, so its roots keep their source order: what it gets of a
// depends on when a's budget runs out.
var perPackageRuns = []struct {
	name     string
	first    []eval.Root
	selected []string
}{
	{"package order", nil, nil},
	{"a first", []eval.Root{aBig, aSmall, aAfter}, nil},
	{"c first", []eval.Root{cOwn, cViaBig, cViaAfter}, nil},
	{"b first", []eval.Root{bWide}, nil},
	{"b and c selected", nil, []string{"b", "c"}},
	{"c then b selected, c first", []eval.Root{cOwn, cViaBig}, []string{"b", "c"}},
	{"b alone", nil, []string{"b"}},
}

// EVALUATION.md §12.2, DECISIONS 328: a package's budget is its own, in any order or selection; its readers are poisoned silently.
func TestBudgetPerPackage(t *testing.T) {
	ar, err := txtar.ParseFile(perPackageCase)
	if err != nil {
		t.Fatal(err)
	}
	opt := eval.Options{Budget: 200}
	want := map[string]string{}
	for _, run := range perPackageRuns {
		b := runFirst(t, fromArchive(t, ar), opt, run.first, run.selected...)
		for _, pkg := range []string{"b", "c"} {
			if len(run.selected) > 0 && !slices.Contains(run.selected, pkg) {
				continue
			}
			got := packageView(t, b, pkg)
			if want[pkg] == "" {
				want[pkg] = got
			} else if got != want[pkg] {
				t.Errorf("%s: %s\n%s\nwant, as in package order,\n%s", run.name, pkg, got, want[pkg])
			}
		}
		stops := codes(b.bags["a"], diag.E4401.Def().Code)
		forcesA := !slices.Equal(run.selected, []string{"b"})
		if len(stops) != btoi(forcesA) || len(b.bags["a"].Findings()) != len(stops) {
			t.Errorf("%s: a's findings %v, want %d budget stop alone", run.name, b.bags["a"].Findings(), btoi(forcesA))
		}
	}
	b := runFirst(t, fromArchive(t, ar), opt, nil)
	heavy := codes(b.bags["a"], diag.E4401.Def().Code)
	if len(heavy) != 1 || !strings.Contains(heavy[0], "heaviest: big") || spentOn(b.ev, bWide) <= spentOn(b.ev, aBig) {
		t.Errorf("a's budget stop %v: want a's own heaviest root, big (%d steps), although b's wide spent %d", heavy, spentOn(b.ev, aBig), spentOn(b.ev, bWide))
	}
}

// runFirst checks p and evaluates it as runBuild does, forcing first before the forced set.
func runFirst(t *testing.T, p *program, opt eval.Options, first []eval.Root, selected ...string) *build {
	t.Helper()
	ctx := context.Background()
	b := &build{prog: p, bags: check.Bags{}, values: map[eval.Root]value.Value{}, order: first}
	fold := eval.NewFolder(b.bags, opt)
	b.checked = check.Check(ctx, exampleProject(), p.files, b.bags, fold)
	b.folded = eval.FoldsOf(fold).Reads()
	b.evaluate(ctx, &host{}, opt, selected, func(ev *eval.Evaluator) { ev.UseFolder(fold) })
	return b
}

// packageView is pkg's findings and the values of its roots, each of c's readers of a poisoned,
// every other value evaluated.
func packageView(t *testing.T, b *build, pkg string) string {
	t.Helper()
	var sb strings.Builder
	for _, f := range b.bags[pkg].Findings() {
		fmt.Fprintf(&sb, "%s %s\n", f.Code, f.Message)
	}
	for _, root := range []eval.Root{bWide, cViaBig, cViaAfter, cOwn} {
		if root.Pkg != pkg {
			continue
		}
		v, ok := b.ev.Settled(root)
		if poisoned := root == cViaBig || root == cViaAfter; poisoned == ok {
			t.Errorf("%v: settled %t, want %t", root, ok, !poisoned)
		}
		if ok {
			fmt.Fprintf(&sb, "%s = %s\n", root.Name, v.CanonText())
		}
	}
	return sb.String()
}

// codes is the findings of bag with code, as text.
func codes(bag *diag.Bag, code diag.Code) []string {
	var out []string
	if bag == nil {
		return nil
	}
	for _, f := range bag.Findings() {
		if f.Code == code {
			out = append(out, f.Message)
		}
	}
	return out
}

// spentOn is the steps e charged root.
func spentOn(e *eval.Evaluator, root eval.Root) int64 {
	for _, c := range e.Charged() {
		if c.Pkg == root.Pkg && c.Name == root.Name {
			return c.Steps
		}
	}
	return 0
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
