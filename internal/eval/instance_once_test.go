package eval_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// onceSrc is a row with a costly check and a dependent field, held by lr; HOLD is what j1 holds.
const onceSrc = pkgA + `/// Kind.
enum Kind { sound, count }

/// Bonus by kind.
type Bonus(k: Kind) = match k {
  sound => String
  count => Int
}

/// A row.
record Row {
  /// Its code.
  code: String
  /// Its kind.
  kind: Kind
  /// Its bonus.
  bonus: Bonus(kind)

  check costly: [i for i in 0..40].filter(i => i >= 0).len() == 40 else "never"
}

/// A row.
let lr: Row = { code: "y", kind: count, bonus: 3 }

/// Keyed by code.
let j1: [Row] keyed by code = HOLD
`

// EVALUATION.md §4.2, §8.1: lr's copy in a keyed list runs no check and converts nothing again.
func TestCopyChecksOnce(t *testing.T) {
	steps := func(hold string) int64 {
		b := buildFiles(t, eval.Options{}, "a/a.canon", strings.Replace(onceSrc, "HOLD", hold, 1))
		assertBuilt(t, b)
		return b.ev.StepsSpent()
	}
	empty, held := steps("[]"), steps("[lr]")
	fresh := steps(`[{ code: "z", kind: count, bonus: 3 }]`)
	t.Logf("[] %d, [lr] %d, a second row %d", empty, held, fresh)
	const literal, check = 5, 40 // a one-name list's steps at most; the check's at least
	if held-empty > literal || fresh-held < check {
		t.Errorf("[] %d, [lr] %d (+%d), a second row %d (+%d): want lr's copy within %d, a second row %d more",
			empty, held, held-empty, fresh, fresh-empty, literal, check)
	}
}

// identitySrc is a row whose costly check reads self's identity, held by lr; HOLD1 and HOLD2 are what j1 and j2 hold.
const identitySrc = pkgA + `/// A row.
record Row {
  /// Its code.
  code: String

  check costly: [i for i in 0..40].filter(i => i >= 0).len() == 40 and self == self else "never"
}

/// A row.
let lr: Row = { code: "y" }

/// Keyed by code.
let j1: [Row] keyed by code = HOLD1

/// Keyed by code.
let j2: [Row] keyed by code = HOLD2
`

// TYPES.md §6.3, log-2026-11-01 "EVAL-OWED review FAIL": a check reading self's identity runs once per identity of lr.
func TestIdentityChecksPerIdentity(t *testing.T) {
	steps := func(hold1, hold2 string) int64 {
		src := strings.NewReplacer("HOLD1", hold1, "HOLD2", hold2).Replace(identitySrc)
		b := buildFiles(t, eval.Options{}, "a/a.canon", src)
		assertBuilt(t, b)
		return b.ev.StepsSpent()
	}
	empty, one, two := steps("[]", "[]"), steps("[lr]", "[]"), steps("[lr]", "[lr]")
	t.Logf("no copy %d, one %d, two %d", empty, one, two)
	const check = 40 // the check's steps at least
	if one-empty < check || two-empty != 2*(one-empty) {
		t.Errorf("no copy %d, one copy %d (+%d), two %d (+%d): want each copy to run the check once", empty, one, one-empty, two, two-empty)
	}
}
