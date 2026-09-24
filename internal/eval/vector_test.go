package eval_test

import (
	"context"
	"math"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// vectorCap is each vector's own step cap (CONFORMANCE.md §6.5).
const vectorCap = 1_000_000

// The integer limits CONFORMANCE.md §6.6 writes INT64_MIN and INT64_MAX.
const (
	intMin = math.MinInt64
	intMax = math.MaxInt64
)

var (
	codeRange = diag.E3204.Def().Code
	codeTS    = diag.E8303.Def().Code
)

// integer is an Int argument, as conform's candidates hold it.
func integer(n int64) value.Value {
	return &value.Int{V: n, T: types.IntType}
}

// row is one vector of CONFORMANCE.md §6.6: an argument, then the Go/C++ and TS expectations.
type row struct {
	arg          int64
	want, tsWant int64
	code, tsCode diag.Code
}

// CONFORMANCE.md §6.6: both worked tables through TestCalls and Vector, the TS column in TS mode.
func TestWorkedExamples(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "potion/potion.canon", potionSrc)
	heal := fnObj(t, b, potionPkg, "Potion", "healFor")
	damage := fnObj(t, b, potionPkg, "Potion", "damageAt")
	ev, sub, _ := freshTests(b, eval.Options{})
	calls, err := ev.TestCalls(context.Background(), potionPkg, []check.Object{heal, damage}, sub)
	if err != nil || len(calls) == 0 {
		t.Fatalf("no test calls: %v", err)
	}
	recv := calls[0].Recv
	tables := []struct {
		fn   check.Object
		rows []row
	}{
		{heal, []row{
			{200, 200, 200, "", ""}, {9000, 500, 500, "", ""}, {-5, 0, 0, "", ""},
			{intMin, 0, 0, "", codeTS}, {-1, 0, 0, "", ""}, {0, 0, 0, "", ""}, {1, 1, 1, "", ""},
			{499, 499, 499, "", ""}, {500, 500, 500, "", ""}, {501, 500, 500, "", ""},
			{intMax, 500, 0, "", codeTS},
		}},
		{damage, []row{
			{10, 50, 50, "", ""}, {intMin, 0, 0, codeRange, codeTS}, {-1, 0, 0, codeRange, codeRange},
			{0, 0, 0, codeRange, codeRange}, {1, 5, 5, "", ""}, {2, 10, 10, "", ""},
			{149, 745, 745, "", ""}, {150, 750, 750, "", ""}, {151, 0, 0, codeRange, codeRange},
			{499, 0, 0, codeRange, codeRange}, {500, 0, 0, codeRange, codeRange},
			{501, 0, 0, codeRange, codeRange}, {intMax, 0, 0, codeRange, codeTS},
		}},
	}
	for _, tab := range tables {
		for _, r := range tab.rows {
			c := eval.Call{Fn: tab.fn, Recv: recv, Args: []value.Value{integer(r.arg)}}
			expectOutcome(t, b.ev.Vector(context.Background(), c, eval.VectorMode{Steps: vectorCap}), r.want, r.code)
			expectOutcome(t, b.ev.Vector(context.Background(), c, eval.VectorMode{Steps: vectorCap, TS: true}), r.tsWant, r.tsCode)
		}
	}
	assertEmpty(t, b.bags)
}

// expectOutcome fails unless o is the Int want, or the code when there is one.
func expectOutcome(t *testing.T, o eval.Outcome, want int64, code diag.Code) {
	t.Helper()
	if code != "" || o.Code != "" || o.Exceeded != eval.NoLimit {
		if o.Code != code || o.Exceeded != eval.NoLimit {
			t.Errorf("outcome %+v, want code %q", o, code)
		}
		return
	}
	if n, ok := intOf(o.Value); !ok || n != want {
		t.Errorf("outcome %+v, want %d", o, want)
	}
}

// intOf is an Int's value, or a Duration's milliseconds.
func intOf(v value.Value) (int64, bool) {
	switch x := v.(type) {
	case *value.Int:
		return x.V, true
	case *value.Dur:
		return x.Ms, true
	}
	return 0, false
}

// gaugeSrc holds translated methods that exercise TS mode, the first error and the limits.
const gaugeSrc = `/// Gauges.
package gauge

/// A gauge with one reading past TypeScript's safe range.
record Gauge {
  /// A reading of 2^60.
  big: Int
  /// A small reading.
  small: Int
  /// A period.
  every: Duration

  /// big when x is positive, else small.
  export fn pick(self, x: Int) -> Int {
    if x > 0 {
      return big
    }
    return small
  }

  /// x squared.
  export fn square(self, x: Int) -> Int { return x * x }

  /// small plus x.
  export fn plus(self, x: Int) -> Int { return small + x }

  /// every, n times.
  export fn spread(self, n: Int) -> Duration { return every * n }

  /// small twice, scaled and x: each read of self paid for once per evaluation.
  export fn mix(self, x: Int) -> Int { return small + small + scaled() + x }

  /// f truncated.
  export fn whole(self, f: Float) -> Int { return Int(f) }

  /// a plus b.
  export fn both(self, a: Int(1..=10), b: Int) -> Int { return a + b }

  /// big scaled down, precomputed.
  export fn scaled(self) -> Int { return big / 1_073_741_824 }

  /// big itself, precomputed.
  export fn raw(self) -> Int { return big }

  /// scaled plus x.
  export fn addScaled(self, x: Int) -> Int { return scaled() + x }

  /// raw plus x.
  export fn addRaw(self, x: Int) -> Int { return raw() + x }

  /// A division by zero after the refinement of x.
  export fn ratio(self, x: Int(0..=10)) -> Int { return 100 / (x - x) }

  /// 2^n calls.
  export fn blow(self, n: Int) -> Int {
    if n <= 0 {
      return 0
    }
    return blow(n - 1) + blow(n - 1)
  }

  /// Never returns.
  export fn deep(self, n: Int) -> Int { return deep(n + 1) }

  /// The count of heavy.
  export fn heavyCount(self) -> Int { return heavy.len() }

  /// heavy's second square.
  export fn heavySecond(self) -> Int { return heavy[1] }

  /// heavyCount plus x when x is positive, else heavySecond plus x.
  export fn probe(self, x: Int) -> Int {
    if x > 0 {
      return heavyCount() + x
    }
    return heavySecond() + x
  }

  /// heavyCount plus x.
  export fn plusHeavy(self, x: Int) -> Int { return heavyCount() + x }
}

/// The gauge of the vectors.
let g: Gauge = { big: 1_152_921_504_606_846_976, small: 3, every: 1000h }

/// A list that costs steps to build.
let heavy: [Int] = [n * n for n in 0..200]
`

const (
	gaugePkg = "gauge"
	pow30    = 1 << 30
	pow40    = 1 << 40
	pow54    = 1 << 54
	pow60    = 1 << 60
	pow27    = 1 << 27
	// everyPast times the gauge's 1000h is 1.08e16 ms: an int64, not a safe integer nor a Duration.
	everyPast  = 3_000_000
	everyTwice = 7_200_000_000
)

// gaugeCase is one vector of a Gauge method: its arguments, then Go/C++ and TS expectations.
type gaugeCase struct {
	name         string
	fn           string
	args         []value.Value
	want, tsWant int64
	code, tsCode diag.Code
}

// CONFORMANCE.md §2.3, §4, §6.5: TS entry reads and arguments, E8303 over E4101 (Int and Duration), the first error decides.
func TestVectorModes(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "gauge/gauge.canon", gaugeSrc)
	g := b.values[eval.Root{Pkg: gaugePkg, Name: "g"}]
	var (
		overflow = diag.E4101.Def().Code
		byZero   = diag.E4102.Def().Code
		toInt    = diag.E4103.Def().Code
		width    = diag.E3201.Def().Code
	)
	for _, c := range []gaugeCase{
		{"branch taken", "pick", ints(1), pow60, 0, "", codeTS},
		{"branch not taken, read anyway", "pick", ints(0), 3, 0, "", codeTS},
		{"unread field", "plus", ints(1), 4, 4, "", ""},
		{"safe result", "plus", ints(maxSafe - 3), maxSafe, maxSafe, "", ""},
		{"unsafe result", "plus", ints(maxSafe), maxSafe + 3, 0, "", codeTS},
		{"int64 overflow", "square", ints(pow40), 0, 0, overflow, codeTS},
		{"unsafe product", "square", ints(pow27), pow54, 0, "", codeTS},
		{"small product", "square", ints(3), 9, 9, "", ""},
		{"Int(f) outside int64", "whole", floats(1e300), 0, 0, toInt, toInt},
		{"Int(f) unsafe", "whole", floats(pow60), pow60, 0, "", codeTS},
		{"Int(f) truncated", "whole", floats(2.5), 2, 2, "", ""},
		{"entry: unsafe argument before a refinement", "both", ints(0, pow60), 0, 0, codeRange, codeTS},
		{"entry: refinement", "both", ints(0, 1), 0, 0, codeRange, codeRange},
		{"precomputed read, native inside", "addScaled", ints(1), pow30 + 1, pow30 + 1, "", ""},
		{"precomputed read, unsafe value", "addRaw", ints(1), pow60 + 1, 0, "", codeTS},
		{"a precomputed method alone, read like a field", "raw", nil, pow60, 0, "", codeTS},
		{"soft error first", "ratio", ints(11), 0, 0, codeRange, codeRange},
		{"Duration times Int beyond int64", "spread", ints(pow40), 0, 0, overflow, codeTS},
		{"Duration times Int past the safe range", "spread", ints(everyPast), 0, 0, width, codeTS},
		{"Duration times Int", "spread", ints(2), everyTwice, everyTwice, "", ""},
		{"hard error", "ratio", ints(5), 0, 0, byZero, byZero},
	} {
		call := eval.Call{Fn: fnObj(t, b, gaugePkg, "Gauge", c.fn), Recv: g, Args: c.args}
		t.Run(c.name, func(t *testing.T) {
			expectOutcome(t, b.ev.Vector(context.Background(), call, eval.VectorMode{Steps: vectorCap}), c.want, c.code)
			expectOutcome(t, b.ev.Vector(context.Background(), call, eval.VectorMode{Steps: vectorCap, TS: true}), c.tsWant, c.tsCode)
		})
	}
	assertEmpty(t, b.bags)
}

// maxSafe is TypeScript's largest safe integer (CONFORMANCE.md §4).
const maxSafe = 1<<53 - 1

func ints(ns ...int64) []value.Value {
	out := make([]value.Value, len(ns))
	for i, n := range ns {
		out[i] = integer(n)
	}
	return out
}

func floats(fs ...float64) []value.Value {
	out := make([]value.Value, len(fs))
	for i, f := range fs {
		out[i] = &value.Float{V: f, T: types.FloatType}
	}
	return out
}

// The caps TestVectorStepsTSNative tries: every one up to a bound past each probe's cost.
const (
	mixCaps   = 200
	probeCaps = 3_000
)

// Decisions log, eval round 2: a TS vector pays every entry read there, so it never succeeds at
// a cap that cuts the native one; with a read in a branch not taken it costs strictly more.
func TestVectorStepsTSNative(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "gauge/gauge.canon", gaugeSrc)
	g := b.values[eval.Root{Pkg: gaugePkg, Name: "g"}]
	mix := eval.Call{Fn: fnObj(t, b, gaugePkg, "Gauge", "mix"), Recv: g, Args: ints(1)}
	if tsOverNative(t, func() *eval.Evaluator { return b.ev }, mix, mixCaps) {
		t.Errorf("mix reads every path it reads in TS: it costs no more than native")
	}
	fresh := func() *eval.Evaluator {
		ev := eval.New(b.checked, verifyInto{prog: b.checked}, check.Bags{}, eval.Options{})
		ev.Force(context.Background(), eval.Root{Pkg: gaugePkg, Name: "g"})
		return ev
	}
	probe := eval.Call{Fn: fnObj(t, b, gaugePkg, "Gauge", "probe"), Recv: g, Args: ints(0)}
	if !tsOverNative(t, fresh, probe, probeCaps) {
		t.Errorf("probe(0) reads heavyCount only in TS: it must cost more there")
	}
}

// tsOverNative runs c natively and in TS at every cap up to caps, failing when TS succeeds where
// native is cut, or when native never succeeds; it reports a cap where native succeeds and TS is cut.
func tsOverNative(t *testing.T, ev func() *eval.Evaluator, c eval.Call, caps int64) bool {
	t.Helper()
	ok, more := false, false
	for steps := int64(1); steps <= caps; steps++ {
		native := ev().Vector(context.Background(), c, eval.VectorMode{Steps: steps})
		ts := ev().Vector(context.Background(), c, eval.VectorMode{Steps: steps, TS: true})
		if ts.Value != nil && native.Value == nil {
			t.Fatalf("cap %d: TS %+v succeeds, native %+v is cut", steps, ts, native)
		}
		ok = ok || native.Value != nil
		more = more || native.Value != nil && ts.Value == nil
	}
	if !ok {
		t.Errorf("%s never completes within %d steps", c.Fn.Name(), caps)
	}
	return more
}
