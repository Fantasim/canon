package check_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
)

const (
	cappedData = "c/data.canon"
	cappedLet  = "cap"
)

// capped is a file of lets checked again on a value edit, one of them with a bound folded while
// checking, which a budget may stop, and a let reading it.
var capped = map[string]string{
	"c/c.canon": `package c

/// The highest level.
const TOP = 9

/// A monster.
record Monster {
  /// Its level.
  level: Int(1..=TOP)
  /// Its hit points.
  hp: Int = 10
}
`,
	cappedData: `package c

/// The monsters.
let monsters: table Monster = {
  rat { level: 1, hp: 3 }
  bat { level: 2 }
}

/// A cap.
let cap: Int(0..=TOP * 5) = 5

/// Twice the cap.
let twice: Int = cap * 2
`,
}

// cappedSteps edit a row before the bounded let, the let's value, and the reader's.
var cappedSteps = []step{
	{cappedData, "hp: 3 }", "hp: 5 }", true},
	{cappedData, "= 5", "= 6", true},
	{cappedData, "cap * 2", "cap * 3", true},
	{cappedData, "hp: 5 }", "hp: 3 }", true},
}

// TYPES.md §1, §15, DECISIONS 150, 263, EVALUATION.md §12.2, NFR-02: E3015 budget breaks a let checked again.
func TestRecheckBudgetBreaksLet(t *testing.T) {
	stopped := 0
	for n := int64(1); n <= budgetTop; n++ {
		t.Run(strconv.FormatInt(n, 10), func(t *testing.T) {
			w := newWorld(t, capped)
			if brokenByBudget(w, n) {
				stopped++
			}
			s := w.budgetSession(n)
			for _, st := range cappedSteps {
				s = w.budgetStep(s, st, n)
			}
		})
	}
	if stopped == 0 {
		t.Error("no budget stops the fold of the let's bound")
	}
}

// brokenByBudget reports a cold Check under budget n breaking the bounded let and reporting
// E3015 budget.
func brokenByBudget(w *world, n int64) bool {
	bags := w.bags()
	prog := check.Check(context.Background(), w.proj, w.list(), bags, budgetFolder(bags, n))
	for _, p := range prog.Packages {
		for _, o := range p.Decls {
			if o.Name() == cappedLet && prog.Info.Broken[o] {
				return holdsCode(bags, diag.E3015.Def().Code)
			}
		}
	}
	return false
}

// zeroBound is a file of lets checked again, one with a bound whose fold fails on a division by
// zero, reported by the evaluator, and a let reading it.
var zeroBound = map[string]string{
	"z/z.canon": "package z\n\n/// The cap.\nlocal let cap: Int(0..=1 / 0) = 5\n\n/// Twice the cap.\nlocal let twice: Int = cap * 2\n",
}

// TYPES.md §1, DECISIONS 150, IMPLEMENTATION-PLAN §7.6 NFR-02: a failed bound breaks a let checked again.
func TestRecheckFailedFoldBreaksLet(t *testing.T) {
	w := newWorld(t, zeroBound)
	_, s, _ := w.session()
	s = w.step(s, step{"z/z.canon", "= 5", "= 6", true})
	w.step(s, step{"z/z.canon", "cap * 2", "cap * 3", true})
}
