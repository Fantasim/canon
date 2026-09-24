package conform

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// CONFORMANCE.md §6.3: pairwise on three lists of two, by hand: 000 011 100 111 001 110, sorted.
func TestPairwiseByHand(t *testing.T) {
	want := [][]int{{0, 0, 0}, {0, 0, 1}, {0, 1, 1}, {1, 0, 0}, {1, 1, 0}, {1, 1, 1}}
	if got := pairwise([]int{2, 2, 2}); !slices.EqualFunc(got, want, slices.Equal[[]int]) {
		t.Errorf("pairwise(2, 2, 2) = %v, want %v", got, want)
	}
}

// CONFORMANCE.md §6.3: the product up to 256 tuples, p1 slowest; past it, every pair covered.
func TestCombine(t *testing.T) {
	if got, want := combine([]int{2, 3}), [][]int{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}}; !slices.EqualFunc(got, want, slices.Equal[[]int]) {
		t.Errorf("combine(2, 3) = %v, want %v", got, want)
	}
	if got := combine([]int{300}); len(got) != 300 || got[299][0] != 299 {
		t.Errorf("one list of 300 is the list itself, got %d tuples", len(got))
	}
	if got := combine([]int{16, 16}); len(got) != 256 {
		t.Errorf("a product of 256 is whole, got %d tuples", len(got))
	}
	if got := combine([]int{3, 0}); len(got) != 0 {
		t.Errorf("an empty list gives no tuple, got %v", got)
	}
	sizes := []int{11, 7, 5, 3}
	got := combine(sizes)
	if len(got) >= 11*7*5*3 || !slices.IsSortedFunc(got, slices.Compare[[]int]) || !slices.Equal(tupleKeys(got), tupleKeys(combine(sizes))) {
		t.Fatalf("pairwise over %v: %d tuples, sorted %t", sizes, len(got), slices.IsSortedFunc(got, slices.Compare[[]int]))
	}
	pairs := 0
	for i := range sizes {
		for j := i + 1; j < len(sizes); j++ {
			pairs += sizes[i] * sizes[j]
		}
	}
	if n := len(coveredPairs(got)); n != pairs {
		t.Errorf("pairwise over %v covers %d pairs of %d", sizes, n, pairs)
	}
}

// coveredPairs is every pair of values of two lists some tuple holds.
func coveredPairs(tuples [][]int) map[pair]bool {
	out := map[pair]bool{}
	for _, tu := range tuples {
		for k := range tu {
			for l := k + 1; l < len(tu); l++ {
				out[pair{k, tu[k], l, tu[l]}] = true
			}
		}
	}
	return out
}

func tupleKeys(ts [][]int) []string {
	out := make([]string, len(ts))
	for i, tu := range ts {
		for _, n := range tu {
			out[i] += strconv.Itoa(n) + ","
		}
	}
	return out
}

// CONFORMANCE.md §6.2: each kind's candidates, dropped outside the type range, sorted.
func TestCandidates(t *testing.T) {
	cases := []struct {
		name        string
		got         []value.Value
		want        []string
		negZeroNext bool
	}{
		{"Int8 ..10: the bound is 9", intCandidates(types.Int8Type, &types.Bound{HasHi: true, Hi: types.Limit{I: 10}}, nil, nil),
			[]string{"-128", "-1", "0", "1", "8", "9", "10", "127"}, false},
		{"UInt8 with a read of 255", intCandidates(types.UInt8Type, nil, []value.Value{&value.Int{V: 300, T: types.IntType}}, []value.Value{&value.Int{V: 255}, &value.Dur{Ms: 7}}),
			[]string{"0", "1", "254", "255"}, false},
		{"Duration 0s..=10m", intCandidates(types.DurationType, &types.Bound{HasLo: true, HasHi: true, HiIncluded: true, Hi: types.Limit{I: 600_000}}, []value.Value{&value.Dur{Ms: 2000}}, nil),
			[]string{"-106751d23h47m16s854ms", "-1ms", "0s", "1ms", "2s", "9m59s999ms", "10m", "10m1ms", "106751d23h47m16s854ms"}, false},
		{"Float with a test argument", floatCandidates(types.FloatType, nil, []value.Value{&value.Float{V: 2.5}}, nil),
			[]string{"-1.7976931348623157e+308", "-1e+300", "-1", "-0.5", "0", "0", "0.5", "1", "2.5", "1e+300", "1.7976931348623157e+308"}, true},
		{"Float32 rounds and drops 1e300", floatCandidates(types.Float32Type, nil, []value.Value{&value.Float{V: 0.1}}, nil),
			[]string{"-3.4028235e+38", "-1", "-0.5", "0", "0", "0.1", "0.5", "1", "3.4028235e+38"}, true},
		{"String", stringCandidates(types.StringType, nil, []value.Value{&value.Str{V: "b"}, &value.Str{V: "a"}, &value.Str{V: "b"}}, nil),
			[]string{"", "a", "b"}, false},
		{"Bool", basicRules[types.Bool](types.BoolType, nil, nil, nil), []string{"false", "true"}, false},
		{"enum: every member in declaration order, retired included", paramCandidates(doorState(), nil),
			[]string{"open", "jammed", "closed"}, false},
		{"DECISIONS 204: Float(0.0..=0.5), each bound's adjacent values", paramCandidates(types.FloatType, &types.Bound{HasLo: true, HasHi: true, HiIncluded: true, Hi: types.Limit{F: 0.5}}),
			[]string{"-1.7976931348623157e+308", "-1e+300", "-1", "-0.5", "-5e-324", "0", "0", "5e-324", "0.49999999999999994", "0.5", "0.5000000000000001", "1", "1e+300", "1.7976931348623157e+308"}, true},
		{"DECISIONS 204: Float ..2.0, an exclusive bound's neighbours and itself", paramCandidates(types.FloatType, &types.Bound{HasHi: true, Hi: types.Limit{F: 2}}),
			[]string{"-1.7976931348623157e+308", "-1e+300", "-1", "-0.5", "0", "0", "0.5", "1", "1.9999999999999998", "2", "2.0000000000000004", "1e+300", "1.7976931348623157e+308"}, true},
		{"DECISIONS 204: Float bound 1e20, where b ± 1 is b", floatCandidates(types.FloatType, &types.Bound{HasLo: true, Lo: types.Limit{F: 1e20}}, nil, nil),
			[]string{"-1.7976931348623157e+308", "-1e+300", "-1", "-0.5", "0", "0", "0.5", "1", "99999999999999980000", "100000000000000000000", "100000000000000020000", "1e+300", "1.7976931348623157e+308"}, true},
		{"DECISIONS 204 (Float32 in binary32, meta/decisions/log-2026-09-24.md): Float32(0.1..)", paramCandidates(types.Float32Type, &types.Bound{HasLo: true, Lo: types.Limit{F: 0.1}}),
			[]string{"-3.4028235e+38", "-1", "-0.5", "0", "0", "0.099999994", "0.1", "0.10000001", "0.5", "1", "3.4028235e+38"}, true},
		{"CONFORMANCE.md §6.2 (meta/decisions/log-2026-09-24.md): a Float read keeps v-1, v, v+1", floatCandidates(types.FloatType, nil, nil, []value.Value{&value.Float{V: 1e20}, &value.Float{V: 2.5}}),
			[]string{"-1.7976931348623157e+308", "-1e+300", "-1", "-0.5", "0", "0", "0.5", "1", "1.5", "2.5", "3.5", "100000000000000000000", "1e+300", "1.7976931348623157e+308"}, true},
	}
	for _, c := range cases {
		var got []string
		for _, v := range c.got {
			got = append(got, v.CanonText())
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
		if i := slices.Index(got, "0"); c.negZeroNext && (i < 0 || !math.Signbit(c.got[i].(*value.Float).V) || math.Signbit(c.got[i+1].(*value.Float).V)) {
			t.Errorf("%s: -0.0 is not just before 0.0", c.name)
		}
	}
	if f := floatCandidates(types.Float32Type, nil, []value.Value{&value.Float{V: 0.1}}, nil)[5].(*value.Float).V; f != float64(float32(0.1)) {
		t.Errorf("Float32 candidate 0.1 is %v, want it rounded to binary32", f)
	}
}

// paramCandidates is the candidates of a lone parameter of type typ refined by rng, with no read or test call.
func paramCandidates(typ types.Type, rng *types.Bound) []value.Value {
	s := &site{fn: &ir.ExportFn{Params: []*ir.Param{{Range: rng}}}, sig: &types.FuncType{Params: []types.Type{typ}}}
	return s.candidates(0, nil, nil)
}

// doorState is an enum whose second member is retired.
func doorState() *types.EnumType {
	return &types.EnumType{Name: "DoorState", Members: []*types.Member{
		{Name: "open", Index: 0}, {Name: "jammed", Index: 1, Retired: true}, {Name: "closed", Index: 2},
	}}
}

// CONFORMANCE.md §6.1: a projection keys a variant by its case, an absent optional as none.
func TestProject(t *testing.T) {
	v := &types.VariantType{Name: "Reward"}
	gold := &types.CaseType{Variant: v, Name: "gold", Index: 1, Fields: []*types.Field{{Name: "n", Type: types.IntType}}}
	v.Cases = []*types.CaseType{{Variant: v, Name: "item"}, gold}
	rec := &types.RecordType{Name: "Chest", Fields: []*types.Field{
		{Name: "reward", Type: v}, {Name: "bonus", Type: gold}, {Name: "key", Type: types.IntType},
	}}
	chest := func(n int64) *value.Record {
		return &value.Record{T: rec, Fields: []value.Value{
			&value.Record{T: gold, Fields: []value.Value{&value.Int{V: n}}}, &value.None{}, nil,
		}}
	}
	reads := []*ir.Read{{Path: []string{"reward"}}, {Path: []string{"bonus", "n"}}}
	_, k1, err1 := project(chest(1), reads, noSelfFn)
	_, k2, err2 := project(chest(2), reads, noSelfFn)
	if err1 != nil || err2 != nil || k1 != k2 || k1 != keyCase+"1"+keyEnd+keyNone+keyEnd {
		t.Errorf("projections %q, %q (%v, %v): want one key, the case then none", k1, k2, err1, err2)
	}
	if _, _, err := project(chest(1), []*ir.Read{{Path: []string{"key"}}}, noSelfFn); !errors.Is(err, ErrUnsupported) {
		t.Errorf("an input read gives %v, want ErrUnsupported (DECISIONS 204: ir refuses it)", err)
	}
}

// noSelfFn is an owner without precomputed methods.
func noSelfFn(value.Value, string) (value.Value, error) { return nil, nil }

// CONFORMANCE.md §2.2 (meta/decisions/log-2026-09-24.md): a precomputed method of self is read like a field; any other name, as DECISIONS 204's non-precomputed method, is ErrUnsupported.
func TestProjectSelfMethod(t *testing.T) {
	rec := &types.RecordType{Name: "Potion", Fields: []*types.Field{{Name: "heal", Type: types.IntType}}}
	recv := &value.Record{T: rec, Fields: []value.Value{&value.Int{V: 500}}}
	method := func(r value.Value, name string) (value.Value, error) {
		if r != recv || name != "potency" {
			return nil, nil
		}
		return &value.Int{V: 1000}, nil
	}
	got, key, err := project(recv, []*ir.Read{{Path: []string{"heal"}}, {Path: []string{"potency"}}}, method)
	if err != nil || len(got) != 2 || got[1].CanonText() != "1000" || key != keyInt+"500"+keyEnd+keyInt+"1000"+keyEnd {
		t.Errorf("heal, potency: %v %q %v, want 500 then 1000", got, key, err)
	}
	if _, _, err := project(recv, []*ir.Read{{Path: []string{"scaled"}}}, method); !errors.Is(err, ErrUnsupported) {
		t.Errorf("a method that is not precomputed gives %v, want ErrUnsupported", err)
	}
	if _, _, err := project(recv, []*ir.Read{{Path: []string{"potency"}}}, func(value.Value, string) (value.Value, error) { return nil, ErrNoOutcome }); !errors.Is(err, ErrNoOutcome) {
		t.Errorf("a failed method read gives %v, want its error", err)
	}
}
