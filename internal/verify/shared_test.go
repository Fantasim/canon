package verify_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// sharedDepth is how deep the shared nesting goes: 2^sharedDepth places reach its leaf.
const sharedDepth = 12

// sharedNest is w0 = [leaf], w_i = [w_{i-1}, w_{i-1}] to depth.
func sharedNest(leaf value.Value, elem types.Type, depth int) value.Value {
	t := types.Type(&types.ListType{Elem: elem})
	v := value.Value(&value.List{T: t, Elems: []value.Value{leaf}})
	for range depth {
		t = &types.ListType{Elem: t}
		v = &value.List{T: t, Elems: []value.Value{v, v}}
	}
	return v
}

// checkShared verifies v as the top-level value w, typed as stored, into a bag of its own.
func checkShared(t *testing.T, ev *evaluator, v value.Value) (verify.Result, *diag.Bag) {
	t.Helper()
	bag := diag.NewBag(&source.FileSet{}, pkg)
	res, err := verify.New(ev, nil, map[string]*diag.Bag{pkg: bag}, nil).Check(context.Background(), eval.Root{Pkg: pkg, Name: "w"}, v)
	if err != nil {
		t.Fatal(err)
	}
	return res, bag
}

// EVALUATION.md §5, §12.1; DECISIONS 509: a value at 2^12 places is verified once, at its first place.
func TestSharedValueVerifiedOnce(t *testing.T) {
	small := &types.Refined{Of: types.IntType, Range: &types.Bound{Hi: types.Limit{I: 0}, HasHi: true}}
	checked := &types.Refined{Of: small, Where: &types.Predicate{Text: "it > 0"}}
	ev := &evaluator{holds: true}
	res, bag := checkShared(t, ev, sharedNest(&value.Int{V: 1, T: checked}, checked, sharedDepth))
	first := "w" + strings.Repeat("[0]", sharedDepth+1)
	found := bag.Findings()
	if len(ev.wheres) != 1 {
		t.Fatalf("where runs %d, want 1", len(ev.wheres))
	}
	if res.Valid || len(ev.invalid) != 1 || len(found) != 1 || found[0].Path != first {
		t.Fatalf("Check: valid %v, %d marks, findings %v; want invalid, 1 mark, one at %s", res.Valid, len(ev.invalid), found, first)
	}
	if ev.wheres[0] != first {
		t.Errorf("where run at %s, want %s", ev.wheres[0], first)
	}
}

// DECISIONS 509: a value at [2] and [10] is found at [2], its first place in walk order, though [10] sorts first.
func TestSharedValueFirstPlace(t *testing.T) {
	small := &types.Refined{Of: types.IntType, Range: &types.Bound{Hi: types.Limit{I: 0}, HasHi: true}}
	inner := &types.ListType{Elem: small}
	shared := &value.List{T: inner, Elems: []value.Value{&value.Int{V: 1, T: small}}}
	const n, a, b = 11, 2, 10
	elems := make([]value.Value, n)
	for i := range elems {
		elems[i] = &value.List{T: inner}
	}
	elems[a], elems[b] = shared, shared
	ev := &evaluator{}
	_, bag := checkShared(t, ev, &value.List{T: &types.ListType{Elem: inner}, Elems: elems})
	if found := bag.Findings(); len(found) != 1 || found[0].Path != "w[2][0]" || len(ev.invalid) != 1 {
		t.Errorf("findings %v, %d marks; want one at w[2][0], one mark", found, len(ev.invalid))
	}
}

// EVALUATION.md §5: a value shared between places of different declared types is verified at each.
func TestSharedValueEachType(t *testing.T) {
	small := &types.Refined{Of: types.IntType, Range: &types.Bound{Hi: types.Limit{I: 0}, HasHi: true}}
	plain := &types.ListType{Elem: types.IntType}
	shared := &value.List{T: plain, Elems: []value.Value{&value.Int{V: 1, T: types.IntType}}}
	pair := &types.PairType{A: plain, B: &types.ListType{Elem: small}}
	ev := &evaluator{}
	_, bag := checkShared(t, ev, &value.Pair{T: pair, A: shared, B: shared})
	if found := bag.Findings(); len(found) != 1 || len(ev.invalid) != 1 {
		t.Errorf("findings %v, %d marks; want the B half's one", found, len(ev.invalid))
	}
}

// sharedTree is w0 = {n}, w_i = {a: wrap(w_{i-1}), b: wrap(w_{i-1}), n} to depth, of a record type
// T whose a and b hold a T, or a list of one when list, and n is checked.
func sharedTree(checked types.Type, list bool, depth int) value.Value {
	rt := record("T")
	var part types.Type = rt
	if list {
		part = &types.ListType{Elem: rt}
	}
	rt.Fields = []*types.Field{field("a", part), field("b", part), field("n", checked)}
	for i, f := range rt.Fields {
		f.Index = i
	}
	n := &value.Int{V: 1, T: checked}
	v := value.Value(&value.Record{T: rt, Fields: []value.Value{nil, nil, n}})
	for range depth {
		below := v
		if list {
			below = &value.List{T: part, Elems: []value.Value{v}}
		}
		v = &value.Record{T: rt, Fields: []value.Value{below, below, n}}
	}
	return v
}

// DECISIONS 509: past a record boundary too, a shared record or list is verified once: the where
// predicate of its n runs once per level, not once per path.
func TestSharedAcrossRecords(t *testing.T) {
	checked := &types.Refined{Of: types.IntType, Where: &types.Predicate{Text: "it > 0"}}
	for _, list := range []bool{false, true} {
		ev := &evaluator{holds: true}
		if _, bag := checkShared(t, ev, sharedTree(checked, list, sharedDepth)); len(bag.Findings()) != 0 || len(ev.wheres) != sharedDepth+1 {
			t.Errorf("list %v: %d where runs, findings %v; want %d, none", list, len(ev.wheres), bag.Findings(), sharedDepth+1)
		}
	}
}
