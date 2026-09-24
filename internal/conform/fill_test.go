package conform_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

// wrapped is the reference evaluator with each outcome passed through edit, and the modes asked.
type wrapped struct {
	*reference
	edit  func(c conform.Call, o conform.Outcome) conform.Outcome
	modes []conform.Mode
}

func (e *wrapped) Evaluate(ctx context.Context, c conform.Call, m conform.Mode) conform.Outcome {
	e.modes = append(e.modes, m)
	o := e.reference.Evaluate(ctx, c, m)
	if e.edit != nil {
		o = e.edit(c, o)
	}
	return o
}

// argIs reports a call whose only argument is the integer n.
func argIs(c conform.Call, n int64) bool {
	v, ok := c.Args[0].(*value.Int)
	return ok && len(c.Args) == 1 && v.V == n
}

// CONFORMANCE.md §6.5: a 1 000 000-step cap per vector; a cut one (arg 1, vector 7) is E9009, the fn gets none.
func TestStepCap(t *testing.T) {
	w, ref := pipeline(t)
	_, healObj := w.fn(t, "pipeline", "Potion", "healFor")
	e := &wrapped{reference: ref, edit: func(c conform.Call, o conform.Outcome) conform.Outcome {
		if c.Fn == healObj && argIs(c, 1) {
			return conform.Outcome{Exceeded: conform.StepLimit}
		}
		return o
	}}
	if err := conform.Fill(context.Background(), w.prog, w.pkgs, e, w.bags); err != nil {
		t.Fatal(err)
	}
	for _, m := range e.modes {
		if m.Steps != 1_000_000 {
			t.Fatalf("a vector ran with a cap of %d steps", m.Steps)
		}
	}
	heal, _ := w.fn(t, "pipeline", "Potion", "healFor")
	damage, _ := w.fn(t, "pipeline", "Potion", "damageAt")
	out := w.findings(t)
	if heal.Vectors != nil || len(damage.Vectors) != 13 || strings.Count(out, "error[") != 1 ||
		!strings.Contains(out, "["+string(diag.E9009.Def().Code)+"]") || !strings.Contains(out, "vector 7 of Potion.healFor") {
		t.Errorf("healFor %d vectors, damageAt %d, findings:\n%s", len(heal.Vectors), len(damage.Vectors), out)
	}
}

// CONFORMANCE.md §4: TS expectations only for a package with a ts emit, whose test file alone reads them.
func TestTSModeOnlyForTS(t *testing.T) {
	w, ref := pipelineOf(t, strings.Replace(potion, `emit ts { out: "@features/pipeline/potion.ts" }`, "", 1))
	heal, _ := w.fn(t, "pipeline", "Potion", "healFor")
	e := &wrapped{reference: ref}
	if err := conform.Fill(context.Background(), w.prog, w.pkgs, e, w.bags); err != nil {
		t.Fatal(err)
	}
	for _, m := range e.modes {
		if m.TS {
			t.Fatal("TS mode asked without a ts emit")
		}
	}
	if len(heal.Vectors) != 11 || heal.Vectors[3].TSCode != "" || heal.Vectors[3].TSWant != nil {
		t.Errorf("healFor without TS: %s", text(heal.Vectors))
	}
}

// CONFORMANCE.md §6.4: the test vectors in call order, then each distinct receiver's, first seen first.
func TestReceiversInOrder(t *testing.T) {
	src := strings.Replace(potion, `let q: Potion = { name: "large", heal: 500 }`, `let q: Potion = { name: "large", heal: 20 }`, 1)
	w := newWorld(t, "pipeline/potion.canon", src)
	heal, obj := w.fn(t, "pipeline", "Potion", "healFor")
	heal.Reads = []*ir.Read{{Name: "heal", Path: []string{"heal"}}}
	p, q := w.value(t, "pipeline", "p"), w.value(t, "pipeline", "q")
	ref := &reference{w: w, calls: []conform.Call{
		{Fn: obj, Recv: p, Args: []value.Value{integer(200)}},
		{Fn: obj, Recv: q, Args: []value.Value{integer(5)}},
		{Fn: obj, Recv: p, Args: []value.Value{integer(200)}},
	}}
	if err := conform.Fill(context.Background(), w.prog, w.pkgs, ref, w.bags); err != nil {
		t.Fatal(err)
	}
	var heals []int64
	for _, v := range heal.Vectors {
		heals = append(heals, intOf(v.Recv[0]))
	}
	want := []int64{500, 20}
	for range 9 {
		want = append(want, 500)
	}
	for range 9 {
		want = append(want, 20)
	}
	if !slices.Equal(heals, want) {
		t.Fatalf("receivers %v, want %v", heals, want)
	}
	if intOf(heal.Vectors[1].Args[0]) != 5 || intOf(heal.Vectors[1].Want) != 5 || intOf(heal.Vectors[19].Want) != 20 {
		t.Errorf("vectors:%s", text(heal.Vectors))
	}
}

// CONFORMANCE.md §6.2: a package fn needs no test; its type and refinement give its candidates.
func TestPackageFn(t *testing.T) {
	w := newWorld(t, "a/a.canon", "/// A.\npackage a\n\n/// Half.\nexport fn half(n: Int(0..=10)) -> Int { return n / 2 }\n\nemit cpp { out: \"@features/a\" }\n")
	if err := conform.Fill(context.Background(), w.prog, w.pkgs, &reference{w: w}, w.bags); err != nil {
		t.Fatal(err)
	}
	half, _ := w.fn(t, "a", "", "half")
	want := []row{
		{0, intMin, 0, 0, codeRange, ""}, {0, -1, 0, 0, codeRange, ""}, {0, 0, 0, 0, "", ""}, {0, 1, 0, 0, "", ""},
		{0, 9, 4, 0, "", ""}, {0, 10, 5, 0, "", ""}, {0, 11, 0, 0, codeRange, ""}, {0, intMax, 0, 0, codeRange, ""},
	}
	if len(half.Vectors) != len(want) {
		t.Fatalf("half: %s", text(half.Vectors))
	}
	for i, r := range want {
		v := half.Vectors[i]
		if len(v.Recv) != 0 || intOf(v.Args[0]) != r.arg || v.Code != r.code || r.code == "" && intOf(v.Want) != r.want {
			t.Errorf("half vector %d:%s", i+1, text(half.Vectors[i:i+1]))
		}
	}
}

// CONFORMANCE.md §7.1 (meta/decisions/log-2026-09-24.md): a json-only or view-only package gets no vector, E9008 or E9009.
func TestNoCodeEmit(t *testing.T) {
	cases := []struct {
		name, emit string
		target     ir.Target
	}{
		{"view only", `emit view { out: "@features/pipeline/potion.view.json" }`, ir.TargetView},
		{"json only", `emit json { out: "@features/pipeline/potion.json", values: [] }`, ir.TargetJSON},
	}
	for _, c := range cases {
		src := strings.Replace(strings.Replace(potion, `emit cpp { out: "@features/pipeline" }`, "", 1),
			`emit ts { out: "@features/pipeline/potion.ts" }`, c.emit, 1)
		w := newWorld(t, "pipeline/potion.canon", src)
		if len(w.pkgs) != 1 || len(w.pkgs[0].Emits) != 1 || w.pkgs[0].Emits[0].Target != c.target {
			t.Fatalf("%s: the package has another emit: %d packages", c.name, len(w.pkgs))
		}
		e := &wrapped{reference: &reference{w: w}}
		if err := conform.Fill(context.Background(), w.prog, w.pkgs, e, w.bags); err != nil {
			t.Fatal(err)
		}
		heal, _ := w.fn(t, "pipeline", "Potion", "healFor")
		if out := w.findings(t); strings.Contains(out, "["+string(diag.E9008.Def().Code)+"]") || len(e.modes) != 0 || heal.Vectors != nil {
			t.Errorf("%s: %d evaluations, %d vectors, findings:\n%s", c.name, len(e.modes), len(heal.Vectors), out)
		}
	}
}

// boostedWorld is the potion world with a precomputed potency, written as body, and a translated
// boosted that reads it: potency() + bonus.
func boostedWorld(t *testing.T, body string) (*world, *reference) {
	t.Helper()
	src := strings.Replace(potion, "  /// The damage", "  /// A heal-derived potency.\n  export fn potency(self) -> Int { return "+body+" }\n\n  /// Potency plus a bonus.\n  export fn boosted(self, bonus: Int(0..=10)) -> Int { return potency() + bonus }\n\n  /// The damage", 1)
	w, ref := pipelineOf(t, src)
	boosted, _ := w.fn(t, "pipeline", "Potion", "boosted")
	if potency, _ := w.fn(t, "pipeline", "Potion", "potency"); potency.Kind != ir.FnPrecomputed || boosted.Kind != ir.FnTranslated {
		t.Fatalf("potency is kind %d, boosted %d", potency.Kind, boosted.Kind)
	}
	boosted.Reads = []*ir.Read{{Name: "potency", Path: []string{"potency"}}}
	return w, ref
}

// CONFORMANCE.md §2.2 (meta/decisions/log-2026-09-24.md): a precomputed method of self is read like a field.
func TestPrecomputedSelfMethod(t *testing.T) {
	w, ref := boostedWorld(t, "heal * 2")
	boosted, obj := w.fn(t, "pipeline", "Potion", "boosted")
	ref.calls = append(ref.calls, conform.Call{Fn: obj, Recv: w.value(t, "pipeline", "p"), Args: []value.Value{integer(3)}})
	if err := conform.Fill(context.Background(), w.prog, w.pkgs, ref, w.bags); err != nil {
		t.Fatal(err)
	}
	checkBoosted(t, boosted, 1000)
}

// checkBoosted checks that boosted has vectors, each on the receiver whose potency is potency.
func checkBoosted(t *testing.T, boosted *ir.ExportFn, potency int64) {
	t.Helper()
	if len(boosted.Vectors) == 0 {
		t.Fatal("boosted has no vectors")
	}
	for i, v := range boosted.Vectors {
		arg := intOf(v.Args[0])
		if len(v.Recv) != 1 || intOf(v.Recv[0]) != potency || v.Code == "" && intOf(v.Want) != potency+arg {
			t.Errorf("boosted vector %d:%s", i+1, text(boosted.Vectors[i:i+1]))
		}
	}
}

// meta/decisions/log-2026-09-24.md, conform N1: a test call whose receiver fails the precomputed method (E4102) gives no vector, no finding, no error, and still counts against E9008.
func TestFailedSelfMethodSkipsCall(t *testing.T) {
	cases := []struct {
		name     string
		withGood bool
	}{
		{"beside a good call", true},
		{"the only call", false},
	}
	for _, c := range cases {
		w, ref := boostedWorld(t, "100_000 / heal")
		boosted, obj := w.fn(t, "pipeline", "Potion", "boosted")
		p := w.value(t, "pipeline", "p").(*value.Record)
		broken := &value.Record{T: p.T, Fields: slices.Clone(p.Fields)}
		broken.Fields[1] = integer(0)
		ref.calls = append(ref.calls, conform.Call{Fn: obj, Recv: broken, Args: []value.Value{integer(4)}})
		if c.withGood {
			ref.calls = append(ref.calls, conform.Call{Fn: obj, Recv: p, Args: []value.Value{integer(3)}})
		}
		if err := conform.Fill(context.Background(), w.prog, w.pkgs, ref, w.bags); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if out := w.findings(t); strings.Contains(out, "error[") {
			t.Errorf("%s: findings:\n%s", c.name, out)
		}
		if !c.withGood {
			if boosted.Vectors != nil {
				t.Errorf("%s: vectors:%s", c.name, text(boosted.Vectors))
			}
			continue
		}
		checkBoosted(t, boosted, 200)
		if intOf(boosted.Vectors[0].Args[0]) != 3 {
			t.Errorf("%s: the first test vector is not the good call:%s", c.name, text(boosted.Vectors))
		}
	}
}

// cancelOn is the reference evaluator that cancels the run during its n-th evaluation, which
// stops with the outcome stop, as an evaluation the context ends does.
type cancelOn struct {
	*reference
	cancel context.CancelFunc
	n      int
	stop   conform.Outcome
}

func (e *cancelOn) Evaluate(ctx context.Context, c conform.Call, m conform.Mode) conform.Outcome {
	if e.n--; e.n == 0 {
		e.cancel()
		return e.stop
	}
	return e.reference.Evaluate(ctx, c, m)
}

// An evaluation the context ends is the context's error, never ErrNoOutcome nor an E9009.
func TestCanceledDuringEvaluation(t *testing.T) {
	for _, stop := range []conform.Outcome{{}, {Exceeded: conform.StepLimit}, {Exceeded: conform.DepthLimit}} {
		w, ref := pipeline(t)
		ctx, cancel := context.WithCancel(context.Background())
		err := conform.Fill(ctx, w.prog, w.pkgs, &cancelOn{reference: ref, cancel: cancel, n: 2, stop: stop}, w.bags)
		cancel()
		if out := w.findings(t); !errors.Is(err, context.Canceled) || strings.Contains(out, "error[") {
			t.Errorf("stopped by %d: %v, findings:\n%s", stop.Exceeded, err, out)
		}
	}
}

// Fill's Go errors: a package without a bag, the internal errors of DECISIONS 204, an IR fn with
// no declaration, a canceled context.
func TestFillErrors(t *testing.T) {
	cases := []struct {
		name string
		want error
		run  func(t *testing.T, w *world, ref *reference) error
	}{
		{"no bag", conform.ErrNoBag, func(_ *testing.T, w *world, ref *reference) error {
			return conform.Fill(context.Background(), w.prog, w.pkgs, ref, check.Bags{})
		}},
		// DECISIONS 204: a vector stopped by a poisoned read cannot happen in a build that emits.
		{"no outcome", conform.ErrNoOutcome, func(_ *testing.T, w *world, ref *reference) error {
			e := &wrapped{reference: ref, edit: func(conform.Call, conform.Outcome) conform.Outcome { return conform.Outcome{} }}
			return conform.Fill(context.Background(), w.prog, w.pkgs, e, w.bags)
		}},
		{"no declaration", conform.ErrNoDecl, func(_ *testing.T, w *world, ref *reference) error {
			w.pkgs[0].Fns = append(w.pkgs[0].Fns, &ir.ExportFn{Name: "ghost", Kind: ir.FnTranslated})
			return conform.Fill(context.Background(), w.prog, w.pkgs, ref, w.bags)
		}},
		// DECISIONS 204: ir refuses at stage E a read with no candidate rule, here a translated method of self.
		{"IR invariant: a read with no candidate rule", conform.ErrUnsupported, func(t *testing.T, w *world, ref *reference) error {
			heal, _ := w.fn(t, "pipeline", "Potion", "healFor")
			heal.Reads = []*ir.Read{{Name: "damageAt", Path: []string{"damageAt"}}}
			return conform.Fill(context.Background(), w.prog, w.pkgs, ref, w.bags)
		}},
		{"canceled", context.Canceled, func(_ *testing.T, w *world, ref *reference) error {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return conform.Fill(ctx, w.prog, w.pkgs, ref, w.bags)
		}},
	}
	for _, c := range cases {
		w, ref := pipeline(t)
		if err := c.run(t, w, ref); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}

// DOCTRINE §5: two runs give the same vectors in the same order.
func TestDeterministic(t *testing.T) {
	var runs []string
	for range 2 {
		w, ref := pipeline(t)
		if err := conform.Fill(context.Background(), w.prog, w.pkgs, ref, w.bags); err != nil {
			t.Fatal(err)
		}
		heal, _ := w.fn(t, "pipeline", "Potion", "healFor")
		damage, _ := w.fn(t, "pipeline", "Potion", "damageAt")
		runs = append(runs, text(heal.Vectors)+text(damage.Vectors))
	}
	if runs[0] != runs[1] {
		t.Errorf("two runs differ:%s\n---%s", runs[0], runs[1])
	}
}

// brokenSrc is a potion whose translated healFor only the test named calls, with its body.
const brokenSrc = `/// P.
package pipeline

/// A healing potion.
record Potion {
  /// Hit points restored.
  heal: Int

  /// What a player missing missingHp gets back.
  export fn healFor(self, missingHp: Int) -> Int { return min(heal, max(missingHp, 0)) }
}

/// A level that does not check.
let level: Int = "high"

test "calls healFor" {
  let p: Potion = { heal: 500 }
  BODY
}

emit cpp { out: "@features/pipeline" }
`

// meta/decisions/log-2026-09-24.md, build wiring (M2): a package whose tests have an error reports no E9008; a test stopped while running still does.
func TestBrokenTestNoE9008(t *testing.T) {
	e9008 := "[" + string(diag.E9008.Def().Code) + "]"
	for _, c := range []struct {
		name, body string
		want       bool // E9008 reported
	}{
		{"its own error", "expect p.healFor(nowhere) == 0", false},
		{"names a broken let", "expect p.healFor(level) == 0", false},
		{"stops at run time", "let z = 1 / (p.heal - 500)\n  expect p.healFor(z) == 0", true},
	} {
		w := newWorld(t, "pipeline/potion.canon", strings.Replace(brokenSrc, "BODY", c.body, 1))
		if err := conform.Fill(context.Background(), w.prog, w.pkgs, &reference{w: w}, w.bags); err != nil {
			t.Fatal(err)
		}
		if out := w.findings(t); strings.Contains(out, e9008) != c.want {
			t.Errorf("%s: %s reported %t, want %t:\n%s", c.name, e9008, !c.want, c.want, out)
		}
	}
}

// meta/decisions/log-2026-09-24.md, build wiring review: a vector with no outcome is skipped once an error is reported, else ErrNoOutcome.
func TestPoisonedVector(t *testing.T) {
	w, ref := pipeline(t)
	if err := conform.Fill(context.Background(), w.prog, w.pkgs, ref, w.bags); err != nil {
		t.Fatal(err)
	}
	heal, _ := w.fn(t, "pipeline", "Potion", "healFor")
	all := len(heal.Vectors)
	for _, reported := range []bool{false, true} {
		w, ref := pipeline(t)
		_, healObj := w.fn(t, "pipeline", "Potion", "healFor")
		if reported {
			diag.E9008.At(source.Span{}, "Potion", "other", "pipeline").Report(w.bags["pipeline"])
		}
		e := &wrapped{reference: ref, edit: func(c conform.Call, o conform.Outcome) conform.Outcome {
			if c.Fn == healObj && argIs(c, 1) {
				return conform.Outcome{} // it read a poisoned value
			}
			return o
		}}
		err := conform.Fill(context.Background(), w.prog, w.pkgs, e, w.bags)
		heal, _ := w.fn(t, "pipeline", "Potion", "healFor")
		switch {
		case !reported && !errors.Is(err, conform.ErrNoOutcome):
			t.Errorf("no error reported: %v, want ErrNoOutcome", err)
		case reported && (err != nil || len(heal.Vectors) != all-1):
			t.Errorf("an error reported: %v, %d vectors, want %d", err, len(heal.Vectors), all-1)
		}
	}
}
