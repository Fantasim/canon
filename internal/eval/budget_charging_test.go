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

const (
	spentCase  = "testdata/findings/E4401_8.txtar"
	checkedLib = "/// A.\npackage a\n\n/// An item.\nrecord Item {\n  /// Its count.\n  count: Int\n\n" +
		"  check count > 0 else \"count must be positive\"\n}\n"
	checkedC   = "/// C.\npackage c\n\nimport a\n\n/// C's items.\nlet items: [a.Item] = [{ count: 1 }, { count: 0 }, { count: 2 }]\n"
	manyItems  = 20
	wideBudget = 100_000
	halves     = 2
)

// EVALUATION.md §12.2, API.md R6: E4401 is a finding of the spent package, whatever code ran, and its unevaluated values' cause.
func TestBudgetStopInSpentPackage(t *testing.T) {
	ar, err := txtar.ParseFile(spentCase)
	if err != nil {
		t.Fatal(err)
	}
	b := runFirst(t, fromArchive(t, ar), eval.Options{Budget: 200}, nil)
	for pkg, at := range map[string]string{"a": "b/b.canon", "b": "", "c": "a/a.canon"} {
		var files []string
		for _, f := range b.bags[pkg].Findings() {
			if f.Code == diag.E4401.Def().Code {
				files = append(files, b.prog.fs.Path(f.Span.File))
			}
		}
		if want := []string{at}; at == "" && len(files) != 0 || at != "" && !slices.Equal(files, want) {
			t.Errorf("%s's budget stops at %v, want %v", pkg, files, want)
		}
	}
	cause := b.ev.PoisonCause(eval.Root{Pkg: "a", Name: "again"})
	if len(cause.Findings) != 1 || cause.Findings[0].Code != diag.E4401.Def().Code || cause.Findings[0].Package != "a" {
		t.Errorf("a.again, left unevaluated: cause %+v, want a's budget stop", cause.Findings)
	}
}

// EVALUATION.md §12.2, DECISIONS 328: an instance check spends its value's package's budget; c's verdict ignores b's.
func TestInstanceChecksChargeTheirValue(t *testing.T) {
	items := strings.Repeat("{ count: 1 }, ", manyItems)
	lib := "/// B.\npackage b\n\nimport a\n\n/// B's items.\nlet items: [a.Item] = [" + items + "]\n"
	p := func() *program {
		return parseFiles(t, []string{"a/a.canon", "b/b.canon", "c/c.canon"}, [][]byte{[]byte(checkedLib), []byte(lib), []byte(checkedC)})
	}
	wide := runFirst(t, p(), eval.Options{Budget: wideBudget}, nil)
	value, checks := spentOn(wide.ev, eval.Root{Pkg: "b", Name: "items"}), spentOn(wide.ev, eval.Root{Pkg: "b", Name: "Item"})
	if checks == 0 || spentOn(wide.ev, eval.Root{Pkg: "a", Name: "Item"}) != 0 {
		t.Fatalf("Item's checks charged %v: want b's and c's, never a's", wide.ev.Charged())
	}
	opt := eval.Options{Budget: value + checks/halves}
	all, alone := runFirst(t, p(), opt, nil), runFirst(t, p(), opt, nil, "c")
	if got, want := bagText(alone.bags["c"]), bagText(all.bags["c"]); got != want || !strings.Contains(want, "count must be positive") {
		t.Errorf("c checked alone:\n%s\nwith b:\n%s", got, want)
	}
	if len(codes(all.bags["b"], diag.E4401.Def().Code)) != 1 || len(all.bags["a"].Findings()) != 0 {
		t.Errorf("b's checks past its budget: b %v, a %v; want b's budget stop alone", all.bags["b"].Findings(), all.bags["a"].Findings())
	}
}

// bagText is a bag's findings as lines.
func bagText(bag *diag.Bag) string {
	var sb strings.Builder
	for _, f := range bag.Findings() {
		fmt.Fprintf(&sb, "%s %d %s\n", f.Code, f.Span.Start, f.Message)
	}
	return sb.String()
}

// intoHost verifies every value, a vector's included.
type intoHost struct {
	served
}

func (intoHost) VerifyInto(context.Context, *eval.Evaluator, eval.Root, value.Value, check.Bags) bool {
	return true
}

const (
	vectorLib  = "/// B.\npackage b\n\nlocal fn spin(n: Int) -> Int {\n  var i = 0\n  while i < n { i += 1 }\n  return i\n}\n\n/// Costly.\nlet heavy: Int = spin(20)\n"
	vectorUser = "/// A.\npackage a\n\nimport b\n\nlocal fn spin(n: Int) -> Int {\n  var i = 0\n  while i < n { i += 1 }\n  return i\n}\n\n" +
		"/// Spends in a, then reads b's heavy.\nfn both(n: Int) -> Int { return spin(n) + b.heavy }\n"
)

// CONFORMANCE.md §6.5, EVALUATION.md §12.2: a vector's cap counts every package's steps, a build's budgets each one's.
func TestVectorCapCountsEveryPackage(t *testing.T) {
	ctx := context.Background()
	b := buildFiles(t, eval.Options{}, "a/a.canon", vectorUser, "b/b.canon", vectorLib)
	both := eval.Call{Fn: fnObj(t, b, "a", "", "both"), Args: ints(20)}
	fresh := func(budget int64) *eval.Evaluator {
		return eval.New(b.checked, intoHost{}, check.Bags{}, eval.Options{Budget: budget})
	}
	wide := fresh(wideBudget)
	if _, ok := wide.Call(ctx, both.Fn, nil, both.Args); !ok {
		t.Fatal("both failed")
	}
	inA, inB := spentOn(wide, eval.Root{Pkg: "a", Name: "both"}), spentOn(wide, eval.Root{Pkg: "b", Name: "heavy"})
	limit := max(inA, inB) + 1
	if limit > inA+inB {
		t.Fatalf("a spends %d, b %d: no cap between them", inA, inB)
	}
	if _, ok := fresh(limit).Call(ctx, both.Fn, nil, both.Args); !ok {
		t.Errorf("a budget of %d per package: both runs out", limit)
	}
	if o := fresh(wideBudget).Vector(ctx, both, eval.VectorMode{Steps: limit}); o.Exceeded != eval.StepLimit {
		t.Errorf("a vector capped at %d: %+v, want the step limit", limit, o)
	}
	if o := fresh(wideBudget).Vector(ctx, both, eval.VectorMode{Steps: inA + inB + 1}); o.Value == nil {
		t.Errorf("a vector capped past both: %+v", o)
	}
}
