package eval_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// potionSrc is CONFORMANCE.md §6.6's Potion with its test calls.
const potionSrc = `/// Potions.
package potion

/// A healing potion.
record Potion {
  /// Its name.
  name: String
  /// Hit points restored.
  heal: Int(1..=100_000)

  /// What a player missing missingHp gets back.
  export fn healFor(self, missingHp: Int) -> Int { return min(heal, max(missingHp, 0)) }

  /// The damage a thrown potion does at level.
  export fn damageAt(self, level: Int(1..=150)) -> Int { return heal * level / 100 }
}

/// The potion of the tests.
let p: Potion = { name: "small", heal: 500 }

/// The same heal, another name.
let q: Potion = { name: "large", heal: 500 }

test "healFor never overheals and never goes negative" {
  expect p.healFor(200) == 200
  expect p.healFor(9000) == 500
  expect q.healFor(-5) == 0
  expect p.damageAt(10) == 50
}
`

const potionPkg = "potion"

// freshTests is a fresh evaluator over b's program, its host verifying through it, as build's
// conform adapter makes one per package; its bags are returned to prove them left empty.
func freshTests(b *build, opt eval.Options) (*eval.Evaluator, eval.Builder, check.Bags) {
	bags := check.Bags{}
	for _, cp := range b.checked.Packages {
		bags[cp.Path] = diag.NewBag(b.prog.fs, cp.Path)
	}
	h := &host{}
	ev := eval.New(b.checked, h, bags, opt)
	h.ev = ev
	h.verifier = verify.New(ev, b.checked, bags, nil)
	return ev, subjects{b: &build{prog: b.prog, checked: b.checked, ev: ev}, pkg: potionPkg}, bags
}

// fnObj is the declared object of fn name, in record owner ("" at top level) of package pkg.
func fnObj(t *testing.T, b *build, pkg, owner, name string) check.Object {
	t.Helper()
	for _, d := range b.decls(pkg) {
		if fn := declaredFn(d, owner, name); fn != nil {
			return b.checked.Info.Defs[fn.Name]
		}
	}
	t.Fatalf("no fn %s.%s in %s", owner, name, pkg)
	return nil
}

// decls is the declarations of package pkg, file by file.
func (b *build) decls(pkg string) []syntax.Decl {
	var out []syntax.Decl
	for _, cp := range b.checked.Packages {
		if cp.Path != pkg {
			continue
		}
		for _, f := range cp.Files {
			out = append(out, f.Decls...)
		}
	}
	return out
}

func declaredFn(d syntax.Decl, owner, name string) *syntax.FnDecl {
	switch x := d.(type) {
	case *syntax.FnDecl:
		if owner == "" && x.Name.Name == name {
			return x
		}
	case *syntax.RecordDecl:
		if x.Name.Name != owner || x.Body == nil {
			return nil
		}
		for _, it := range x.Body.Items {
			if fn, ok := it.(*syntax.FnDecl); ok && fn.Name.Name == name {
				return fn
			}
		}
	}
	return nil
}

// callText prints calls as `recv.fn(args)`, one per line.
func callText(calls []eval.Call) string {
	var sb strings.Builder
	for _, c := range calls {
		recv := "-"
		if c.Recv != nil {
			recv = c.Recv.CanonText()
		}
		args := make([]string, len(c.Args))
		for i, a := range c.Args {
			args[i] = a.CanonText()
		}
		sb.WriteString(recv + "." + c.Fn.Name() + "(" + strings.Join(args, ", ") + ")\n")
	}
	return sb.String()
}

// CONFORMANCE.md §6.1: the named fns' calls by the tests, in evaluation order, receiver and arguments.
func TestTestCallsPotion(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "potion/potion.canon", potionSrc)
	heal := fnObj(t, b, potionPkg, "Potion", "healFor")
	damage := fnObj(t, b, potionPkg, "Potion", "damageAt")
	for _, c := range []struct {
		fns  []check.Object
		want string
	}{
		{[]check.Object{heal, damage}, `Potion{name: "small", heal: 500}.healFor(200)
Potion{name: "small", heal: 500}.healFor(9000)
Potion{name: "large", heal: 500}.healFor(-5)
Potion{name: "small", heal: 500}.damageAt(10)
`},
		{[]check.Object{damage}, "Potion{name: \"small\", heal: 500}.damageAt(10)\n"},
		{nil, ""},
	} {
		ev, sub, bags := freshTests(b, eval.Options{})
		calls, err := ev.TestCalls(context.Background(), potionPkg, c.fns, sub)
		if err != nil || callText(calls) != c.want {
			t.Errorf("calls, %v:\n%s\nwant:\n%s", err, callText(calls), c.want)
		}
		assertEmpty(t, bags)
	}
}

// assertEmpty fails on any finding in bags: test-call collection reports none (CONFORMANCE.md §6.1).
func assertEmpty(t *testing.T, bags check.Bags) {
	t.Helper()
	for _, name := range slices.Sorted(maps.Keys(bags)) {
		if fs := bags[name].Findings(); len(fs) > 0 {
			t.Errorf("package %s: %d findings, first %s %s", name, len(fs), fs[0].Code, fs[0].Message)
		}
	}
}

// stopsSrc has tests that fail, stop, and call through a helper.
const stopsSrc = potionSrc + `
/// Two calls of healFor.
local fn twice(x: Int) -> Int { return p.healFor(x) + p.healFor(x + 1) }

test "stops after a hard error" {
  expect p.healFor(3) == 0
  let zero = p.heal - 500
  let z = 1 / zero
  expect p.healFor(4) == 4
}

test "calls through a helper" {
  expect twice(7) == 15
}
`

// CONFORMANCE.md §6.1, EVALUATION.md §10.4: calls before a stop kept, nested calls in order, no finding.
func TestTestCallsStops(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "potion/potion.canon", stopsSrc)
	ev, sub, bags := freshTests(b, eval.Options{})
	calls, err := ev.TestCalls(context.Background(), potionPkg, []check.Object{fnObj(t, b, potionPkg, "Potion", "healFor")}, sub)
	got := argsOf(calls)
	want := []int64{200, 9000, -5, 3, 7, 8}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("arguments %v, %v; want %v", got, err, want)
	}
	assertEmpty(t, bags)
}

// argsOf is the first argument of each call, an Int.
func argsOf(calls []eval.Call) []int64 {
	out := make([]int64, 0, len(calls))
	for _, c := range calls {
		if n, ok := c.Args[0].(*value.Int); ok {
			out = append(out, n.V)
		}
	}
	return out
}

// budgetSrc has two tests that each spend thousands of steps around a call.
const budgetSrc = potionSrc + `
test "calls, then burns" {
  expect p.healFor(1) == 1
  expect [n * n for n in 0..1000].len() == 1000
}

test "burns, then calls" {
  expect [n * n for n in 0..1000].len() == 1000
  expect p.healFor(2) == 2
}
`

// The budgets of TestTestCallsBudget: one the two tests share without running out, one that
// only the first fits in, each above a single test's cost.
const (
	roomyBudget = 1_000_000
	tightBudget = 6_000
)

// EVALUATION.md §10.1, CONFORMANCE.md §6.1: one budget shared by a package's tests, in order.
func TestTestCallsBudget(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "potion/potion.canon", budgetSrc)
	heal := fnObj(t, b, potionPkg, "Potion", "healFor")
	for _, c := range []struct {
		budget int64
		want   []int64
	}{
		{roomyBudget, []int64{200, 9000, -5, 1, 2}},
		{tightBudget, []int64{200, 9000, -5, 1}},
	} {
		ev, sub, bags := freshTests(b, eval.Options{Budget: c.budget})
		calls, err := ev.TestCalls(context.Background(), potionPkg, []check.Object{heal}, sub)
		if got := argsOf(calls); err != nil || !slices.Equal(got, c.want) {
			t.Errorf("budget %d: arguments %v, %v; want %v", c.budget, got, err, c.want)
		}
		assertEmpty(t, bags)
	}
}

// CONFORMANCE.md §6.1: tests force values afresh, so a used evaluator is refused; so is an unknown package.
func TestTestCallsRefusals(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "potion/potion.canon", potionSrc)
	ctx := context.Background()
	if _, err := b.ev.TestCalls(ctx, potionPkg, nil, nil); !errors.Is(err, eval.ErrNotFresh) {
		t.Errorf("stage A's evaluator: %v, want ErrNotFresh", err)
	}
	ev, sub, _ := freshTests(b, eval.Options{})
	ev.Force(ctx, eval.Root{Pkg: potionPkg, Name: "p"})
	if _, err := ev.TestCalls(ctx, potionPkg, nil, sub); !errors.Is(err, eval.ErrNotFresh) {
		t.Errorf("after a Force: %v, want ErrNotFresh", err)
	}
	ev, sub, _ = freshTests(b, eval.Options{})
	if _, err := ev.TestCalls(ctx, "nowhere", nil, sub); !errors.Is(err, eval.ErrNoPackage) {
		t.Errorf("unknown package: %v, want ErrNoPackage", err)
	}
}

// verifySrc has a value failing verification that only a test forces.
const verifySrc = potionSrc + `
/// A status.
record Status {
  /// Its label.
  label: String
}

/// The statuses.
let statuses: stable table Status = {
  open { label: "Open" }
}

/// A status no table holds.
let bad: ref Status = "draft"

test "reads a bad ref" {
  expect bad == bad
  expect p.healFor(6) == 6
}
`

// The host verifies into throwaway bags during TestCalls: a value failing verification that a
// test forces leaves the project's bags as they were (meta/decisions/0003-eval-conformance-api.md).
func TestTestCallsVerifyAside(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "potion/potion.canon", verifySrc)
	before := b.findings(t)
	aside := check.Bags{potionPkg: diag.NewBag(b.prog.fs, potionPkg)}
	h := &host{}
	ev := eval.New(b.checked, h, b.bags, eval.Options{})
	h.ev = ev
	h.verifier = verify.New(ev, b.checked, aside, nil)
	sub := subjects{b: &build{prog: b.prog, checked: b.checked, ev: ev}, pkg: potionPkg}
	calls, err := ev.TestCalls(context.Background(), potionPkg, []check.Object{fnObj(t, b, potionPkg, "Potion", "healFor")}, sub)
	if got := argsOf(calls); err != nil || !slices.Equal(got, []int64{200, 9000, -5, 6}) {
		t.Errorf("arguments %v, %v", got, err)
	}
	if after := b.findings(t); after != before {
		t.Errorf("the project's findings changed:\n%s\nwant:\n%s", after, before)
	}
	if fs := aside[potionPkg].Findings(); len(fs) == 0 || fs[0].Code != diag.E3501.Def().Code {
		t.Errorf("throwaway findings %v, want %s", fs, diag.E3501.Def().Code)
	}
}
