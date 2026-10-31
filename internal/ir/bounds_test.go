package ir_test

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// level is a TypeRef of kind k (an integer of bits and signedness) refined by r.
func level(k types.Kind, bits int, signed bool, r *types.Bound) *ir.TypeRef {
	return &ir.TypeRef{Kind: k, Bits: bits, Signed: signed, Range: r}
}

func intLevel(r *types.Bound) *ir.TypeRef { return level(types.Int, 64, true, r) }

func floats(lo, hi float64, hasLo, hasHi, incl bool) *types.Bound {
	return &types.Bound{Lo: types.Limit{F: lo}, Hi: types.Limit{F: hi}, HasLo: hasLo, HasHi: hasHi, HiIncluded: incl}
}

func ints(lo, hi int64, hasLo, hasHi, incl bool) *types.Bound {
	return &types.Bound{Lo: types.Limit{I: lo}, Hi: types.Limit{I: hi}, HasLo: hasLo, HasHi: hasHi, HiIncluded: incl}
}

// CODEGEN.md §5.15 `<R>`: the folded range in Canon's text, ints decimal, floats as binary64 (FloatText), Durations as literals, lengths decimal.
func TestRangeText(t *testing.T) {
	for _, c := range []struct {
		t    *ir.TypeRef
		want string
	}{
		{intLevel(ints(1, 120, true, true, true)), "1..=120"},
		{intLevel(ints(0, 1, true, true, false)), "0..1"},
		{intLevel(ints(0, 8, false, true, true)), "..=8"},
		{intLevel(ints(-5, 2, true, true, true)), "-5..=2"},
		{intLevel(ints(-40, 0, true, false, false)), "-40.."},
		{intLevel(ints(0, 8, false, true, false)), "..8"},
		{level(types.Duration, 0, false, ints(1000, 3600000, true, true, true)), "1s..=1h"},
		{level(types.Duration, 0, false, ints(-250, 90000, true, true, false)), "-250ms..1m30s"},
		{level(types.Float, 64, true, floats(0, 1, true, true, false)), "0..1"},
		{level(types.Float, 32, true, floats(0, 0.1, false, true, true)), "..=0.1"},
		{level(types.Float, 64, true, floats(-2.5, 1e21, true, true, true)), "-2.5..=1e+21"},
		{level(types.String, 0, false, ints(0, 16, false, true, true)), "..=16"},
		{level(types.List, 0, false, ints(1, 0, true, false, false)), "1.."},
		{intLevel(nil), ""},
	} {
		if got := ir.RangeText(c.t); got != c.want {
			t.Errorf("RangeText(%+v) = %q, want %q", c.t.Range, got, c.want)
		}
	}
}

// CODEGEN.md §5.15, DECISIONS 399 (Decision 5): a value, a String's byte length or a collection's element count, then <R>, the same bytes in every target.
func TestRangeFailure(t *testing.T) {
	for _, c := range []struct {
		t    *ir.TypeRef
		want string
	}{
		{intLevel(ints(0, 5, true, true, true)), "expected a value in 0..=5"},
		{intLevel(ints(-5, 0, true, false, false)), "expected a value in -5.."},
		{level(types.Float, 64, true, floats(0, 1, true, true, false)), "expected a value in 0..1"},
		{level(types.Duration, 0, false, ints(1000, 3600000, true, true, true)), "expected a value in 1s..=1h"},
		{level(types.String, 0, false, ints(1, 8, true, true, true)), "expected a byte length in 1..=8"},
		{level(types.List, 0, false, ints(0, 500, false, true, true)), "expected an element count in ..=500"},
		{level(types.Map, 0, false, ints(1, 0, true, false, false)), "expected an element count in 1.."},
		{level(types.DepMap, 0, false, ints(0, 3, false, true, false)), "expected an element count in ..3"},
		{level(types.Bool, 0, false, ints(0, 3, false, true, false)), ""},
		{intLevel(nil), ""},
	} {
		if got := ir.RangeFailure(c.t); got != c.want {
			t.Errorf("RangeFailure(%v %+v) = %q, want %q", c.t.Kind, c.t.Range, got, c.want)
		}
	}
}

// CODEGEN.md §5.15, DECISIONS 399 (Decision 3): an integer, Duration or length side the stored type guarantees writes no comparison (GCC's -Wtype-limits); a Float side always does; an absent side has none.
func TestImplied(t *testing.T) {
	maxDur := types.DurationLimit
	for _, c := range []struct {
		name   string
		t      *ir.TypeRef
		lo, hi bool
	}{
		{"UInt8(0..=255)", level(types.Int, 8, false, ints(0, 255, true, true, true)), true, true},
		{"Int8(0..=300)", level(types.Int, 8, true, ints(0, 300, true, true, true)), false, true},
		{"Int8(-128..127)", level(types.Int, 8, true, ints(-128, 127, true, true, false)), true, false},
		{"Int8(..128)", level(types.Int, 8, true, ints(0, 128, false, true, false)), true, true},
		{"UInt8(-3..)", level(types.Int, 8, false, ints(-3, 0, true, false, false)), true, true},
		{"Int(0..)", intLevel(ints(0, 0, true, false, false)), false, true},
		{"Int(..=max)", intLevel(ints(0, math.MaxInt64, false, true, true)), true, true},
		{"Int(..max)", intLevel(ints(0, math.MaxInt64, false, true, false)), true, false},
		{"hand-built Int, no width", level(types.Int, 0, false, ints(-5, 5, true, true, true)), false, false},
		{"Duration(1s..=1h)", level(types.Duration, 0, false, ints(1000, 3600000, true, true, true)), false, false},
		{"Duration at its limits", level(types.Duration, 0, false, ints(-maxDur, maxDur, true, true, true)), true, true},
		{"String(0..=16)", level(types.String, 0, false, ints(0, 16, true, true, true)), true, false},
		{"[T](1..=max)", level(types.List, 0, false, ints(1, math.MaxInt64, true, true, true)), false, true},
		{"{K: V}(-5..)", level(types.Map, 0, false, ints(-5, 0, true, false, false)), true, true},
		{"Float(0.0..1.0)", level(types.Float, 64, true, floats(0, 1, true, true, false)), false, false},
		{"Float(..=1e308)", level(types.Float, 64, true, floats(0, 1e308, false, true, true)), true, false},
		{"no refinement", intLevel(nil), true, true},
	} {
		if lo, hi := ir.Implied(c.t, false), ir.Implied(c.t, true); lo != c.lo || hi != c.hi {
			t.Errorf("%s: Implied = %v, %v, want %v, %v", c.name, lo, hi, c.lo, c.hi)
		}
	}
}

// boundsSrc has a record and a case whose fields refine several levels, with overrides and @ts(bigint).
const boundsSrc = `/// B.
package b

/// A probe.
record Probe {
  /// Tags.
  tags: [Int(0..=5)](..=3)
  /// Weights.
  weights: {String(1..=8): Float(0.0..1.0)}(1..)
  /// Renamed.
  level: Int(1..) @cpp(name: "GetLv") @go(name: "Lv") @ts(name: "lv")
  /// Big.
  big: [UInt64(..=9_000_000_000_000_000_000)] @ts(bigint)
  /// Unrefined.
  plain: Int
}

/// A shape.
variant Shape @json(tag: "type") {
  /// A circle.
  circle {
    /// Its radius.
    r: Float(0.0..)
  }
}

emit cpp { out: "@features/bc", namespace: "b", mode: types }
emit go { out: "@features/bg", package: "b", mode: types }
emit ts { out: "@features/bt/b.ts", mode: types }
`

// CODEGEN.md §5.15, §3.3, DECISIONS 400: FieldBounds walks each field's levels depth first, Min first; the three plans name each bound (C++ k<F>… in the class, Go and TS T_<F>…), overrides and a case included, and reserve them in their scopes.
func TestBoundNames(t *testing.T) {
	w := newWorld(t)
	w.add(t, "b/b.canon", []byte(boundsSrc))
	pkgs := w.build(t, "b")
	if out := w.findings(t); !strings.HasPrefix(out, "0 errors") {
		t.Fatalf("findings:\n%s", out)
	}
	p := pkgs[0]
	probe, shape := p.Types[0].(*ir.Record), p.Types[1].(*ir.Variant)
	fields := append(append([]*ir.Field{}, probe.Fields...), shape.Cases[0].Fields...)
	var cpp, gos, ts []string
	var cp *ir.CppNamePlan
	var gp *ir.GoNamePlan
	var te *ir.Emit
	for _, e := range p.Emits {
		switch e.Target {
		case ir.TargetCpp:
			cp = ir.PlanCppNames(p, e)
		case ir.TargetGo:
			gp = ir.PlanGoNames(p, e)
		case ir.TargetTS:
			te = e
		}
	}
	for i, f := range fields {
		owner, goOwner := "Probe", gp.TypeName(probe)
		if i >= len(probe.Fields) {
			owner, goOwner = "ShapeCircle", gp.CaseName(shape, shape.Cases[0])
		}
		for _, b := range ir.FieldBounds(f) {
			cpp = append(cpp, cp.BoundName(f, b))
			gos = append(gos, gp.BoundName(goOwner, f, b))
			ts = append(ts, ir.TSBoundName(owner, f, b))
		}
	}
	suffixes := []string{"TagsLenMax", "TagsElemMin", "TagsElemMax", "WeightsLenMin", "WeightsKeyLenMin", "WeightsKeyLenMax",
		"WeightsValueMin", "WeightsValueEnd", "LvMin", "BigElemMax", "RMin"}
	var wantCpp, wantGo []string
	for i, s := range suffixes {
		wantCpp = append(wantCpp, "k"+s)
		owner := "Probe_"
		if i == len(suffixes)-1 {
			owner = "ShapeCircle_"
		}
		wantGo = append(wantGo, owner+s)
	}
	same(t, "C++", cpp, wantCpp)
	same(t, "Go", gos, wantGo)
	same(t, "TS", ts, wantGo)
	reserved(t, "C++ Probe", ir.CppScopeNames(cp)["Probe"], wantCpp[:len(wantCpp)-1])
	reserved(t, "C++ ShapeCircle", ir.CppScopeNames(cp)["ShapeCircle"], wantCpp[len(wantCpp)-1:])
	reserved(t, "Go package", ir.GoScopeNames(gp)["package"], wantGo)
	reserved(t, "TS module", ir.TSModuleNames(p, te), wantGo)
	boundFacts(t, probe, shape)
}

// boundFacts checks each BoundConst's path, side, kind, limit, level and bigint position.
func boundFacts(t *testing.T, probe *ir.Record, shape *ir.Variant) {
	t.Helper()
	tags := ir.FieldBounds(probe.Fields[0])
	want := []ir.BoundConst{
		{Length: true, Side: ir.SideMax, Kind: types.Int, Limit: types.Limit{I: 3}, Level: &probe.Fields[0].Type},
		{Path: []ir.BoundStep{ir.StepElem}, Side: ir.SideMin, Kind: types.Int, Limit: types.Limit{I: 0}, Level: probe.Fields[0].Type.Elem},
		{Path: []ir.BoundStep{ir.StepElem}, Side: ir.SideMax, Kind: types.Int, Limit: types.Limit{I: 5}, Level: probe.Fields[0].Type.Elem},
	}
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("tags bounds:\n got %+v\nwant %+v", tags, want)
	}
	weights := ir.FieldBounds(probe.Fields[1])
	if v := weights[len(weights)-1]; v.Kind != types.Float || v.Side != ir.SideEnd || v.Limit.F != 1 ||
		!reflect.DeepEqual(v.Path, []ir.BoundStep{ir.StepValue}) || v.Level != probe.Fields[1].Type.Elem {
		t.Errorf("weights' value end: %+v", v)
	}
	if k := weights[1]; !k.Length || k.Kind != types.Int || !reflect.DeepEqual(k.Path, []ir.BoundStep{ir.StepKey}) {
		t.Errorf("weights' key length: %+v", k)
	}
	big := ir.FieldBounds(probe.Fields[3])
	if len(big) != 1 || !big[0].Big || big[0].Limit.I != 9_000_000_000_000_000_000 {
		t.Errorf("a @ts(bigint) element bound is Big: %+v", big)
	}
	if tags[0].Big || ir.FieldBounds(probe.Fields[4]) != nil {
		t.Errorf("no Big off a @ts(bigint) field, no bound on an unrefined one")
	}
	if r := ir.FieldBounds(shape.Cases[0].Fields[0]); len(r) != 1 || r[0].Kind != types.Float || r[0].Side != ir.SideMin {
		t.Errorf("the case's radius: %+v", r)
	}
}

func same(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s bound names:\n got %v\nwant %v", what, got, want)
	}
}

// reserved checks that every name of want is declared in scope.
func reserved(t *testing.T, what string, scope map[string]bool, want []string) {
	t.Helper()
	for _, n := range want {
		if !scope[n] {
			t.Errorf("%s does not reserve %s", what, n)
		}
	}
}
