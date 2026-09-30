package check_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// budgetTop is past the steps the priced fixture spends cold, so its last budgets never run out.
const budgetTop = 40

// priced folds its bounds while checking and its defaults after, as stage E does; an entry file
// earlier in path order names a const and a let declared after it.
var priced = map[string]string{
	"p/p.canon": `package p

/// The base price.
const BASE = 50

/// The highest price.
const LIMIT = BASE * 2

/// The lowest stock.
const LOW = BASE / 25

/// A thing sold.
record Item {
  /// Its name.
  name: String(1..)
  /// Its price.
  price: Int(0..=LIMIT)
  /// How many are in stock.
  stock: Int(LOW..=99) = LOW + 3
  /// Its weight.
  weight: Int = 2 * 3 + 1
}

/// Every item.
let items: table Item = {}

local let spare = LOW + LIMIT
`,
	"p/a.canon": `package p

entry items.apple {
  name: "Apple"
  price: LIMIT - 1
  weight: spare
}
`,
	"p/items/b.canon": `package p

entry items.bread {
  name: "Bread"
  price: 5
}
`,
}

// pricedSteps edit both entry files, break and mend one, then recheck a file as it is: a step
// replacing a text by itself swaps nothing.
var pricedSteps = []step{
	{"p/items/b.canon", "price: 5", "price: 6", true},
	{"p/a.canon", "price: LIMIT - 1", "price: LIMIT - 2", true},
	{"p/items/b.canon", "price: 6", `price: "x"`, true},
	{"p/items/b.canon", `price: "x"`, "price: 5", true},
	{"p/a.canon", "weight: spare", "weight: spare", true},
}

// EVALUATION.md §12.2, DECISIONS 104, IMPLEMENTATION-PLAN §7.6 NFR-02: Recheck's E4401 is cold's.
func TestRecheckBudgetEqualsCold(t *testing.T) {
	stageOnly := 0
	for n := int64(1); n <= budgetTop; n++ {
		t.Run(strconv.FormatInt(n, 10), func(t *testing.T) {
			w := newWorld(t, priced)
			inCheck, after := exhaustion(w, n)
			if after && !inCheck {
				stageOnly++
			}
			s := w.budgetSession(n)
			for _, st := range pricedSteps {
				s = w.budgetStep(s, st, n)
			}
		})
	}
	if stageOnly == 0 {
		t.Error("no budget runs out only in the folds after the check")
	}
	if inCheck, after := exhaustion(newWorld(t, priced), budgetTop); inCheck || after {
		t.Errorf("budget %d runs out", budgetTop)
	}
}

// budgetSession checks w keeping a session under budget n, then folds the defaults, as cold.
func (w *world) budgetSession(n int64) *check.Session {
	w.t.Helper()
	bags := w.bags()
	fold := budgetFolder(bags, n)
	prog, s := check.CheckSession(context.Background(), w.proj, w.list(), bags, fold)
	if s == nil {
		w.t.Fatal("no session")
	}
	foldDefaults(fold, prog)
	if got, want := canonical(w.fs, prog, bags), w.coldAt(n); got != want {
		w.t.Fatalf("budget %d: the session differs from Check:\n%s", n, firstDiff(got, want))
	}
	return s
}

// budgetStep applies one edit under budget n, folds the defaults with the Recheck's folder, and
// compares the result with a cold Check followed by the same folds.
func (w *world) budgetStep(s *check.Session, st step, n int64) *check.Session {
	w.t.Helper()
	nf := w.files[st.path]
	if st.old != st.new {
		nf = w.edit(st.path, st.old, st.new)
	}
	bags := check.Bags{}
	w.parseInto(bags, nf)
	fold := budgetFolder(bags, n)
	prog, next, ok := s.Recheck(context.Background(), []*syntax.File{nf}, bags, fold)
	if ok != st.ok {
		w.t.Fatalf("budget %d: %s %q to %q: Recheck ok=%v, want %v", n, st.path, st.old, st.new, ok, st.ok)
	}
	if !ok {
		return s
	}
	w.commit(nf.Src.Path, nf)
	foldDefaults(fold, prog)
	if got, want := canonical(w.fs, prog, bags), w.coldAt(n); got != want {
		w.t.Fatalf("budget %d: %s %q to %q: Recheck differs from Check:\n%s", n, st.path, st.old, st.new, firstDiff(got, want))
	}
	return next
}

// coldAt is the canonical text of Check under budget n, the defaults folded after it.
func (w *world) coldAt(n int64) string {
	bags := w.bags()
	fold := budgetFolder(bags, n)
	prog := check.Check(context.Background(), w.proj, w.list(), bags, fold)
	foldDefaults(fold, prog)
	return canonical(w.fs, prog, bags)
}

// exhaustion reports whether a cold Check under budget n runs out, and whether it has once the
// defaults are folded.
func exhaustion(w *world, n int64) (inCheck, after bool) {
	bags := w.bags()
	fold := budgetFolder(bags, n)
	prog := check.Check(context.Background(), w.proj, w.list(), bags, fold)
	inCheck = holdsCode(bags, diag.E4401.Def().Code)
	foldDefaults(fold, prog)
	return inCheck, holdsCode(bags, diag.E4401.Def().Code)
}

func budgetFolder(bags check.Bags, n int64) check.Folder {
	return eval.NewFolder(bags, eval.Options{Budget: n})
}

// foldDefaults folds the field defaults of prog's records with prog's Info, as stage E does.
func foldDefaults(fold check.Folder, prog *check.Program) {
	for _, p := range prog.Packages {
		for _, o := range p.Decls {
			foldRecord(fold, prog.Info, o)
		}
	}
}

// foldRecord folds each field default of o in order when o is an unbroken record.
func foldRecord(fold check.Folder, info *check.Info, o check.Object) {
	rt, ok := o.Type().(*types.RecordType)
	if !ok || o.Kind() != check.ObjTypeName || info.Broken[o] {
		return
	}
	for _, f := range rt.Fields {
		if f.Default != nil {
			fold.Fold(context.Background(), o, f.Default, info)
		}
	}
}

// holdsCode reports a finding of code in any bag.
func holdsCode(bags check.Bags, code diag.Code) bool {
	for _, b := range bags { //canon:unordered a membership test
		if hasCode(b, code) {
			return true
		}
	}
	return false
}
