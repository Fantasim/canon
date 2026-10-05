package types_test

import (
	"regexp"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// TYPES.md §8.4, DECISIONS 304: `past` takes an enum, variant or ref under aliases and refinements.
func TestPastApplies(t *testing.T) {
	for _, tc := range []struct {
		name string
		of   types.Type
		want bool
	}{
		{"an enum", element, true},
		{"a variant", reward, true},
		{"a ref", refStatus, true},
		{"a ref resolved in an enclosing record", &types.RefType{Target: talentNodes}, true},
		{"an alias of an enum", &types.Alias{Pkg: "resource.vocab", Name: "Elem", Def: element}, true},
		{"an enum with a where", &types.Refined{Of: element, Where: positive}, true},
		{"an unresolved type", types.ErrorType, true},
		{"Int is refused", types.IntType, false},
		{"a record is refused", eventType, false},
		{"a variant case is refused", rewardItem, false},
		{"Kind(V) is refused", &types.VariantKindType{Variant: reward}, false},
		{"a list of enums is refused", list(element), false},
		{"an alias of an optional enum is refused", &types.Alias{Name: "O", Def: &types.OptionalType{Elem: element}}, false},
		{"a table is refused", &types.TableType{Elem: eventType}, false},
		{"an application with a Never branch is refused", &types.TypeAppType{Fn: paramFn}, false},
		{"an alias of Int is refused", penya, false},
	} {
		got, ok := types.Past(tc.of)
		if ok != tc.want {
			t.Errorf("%s: Past(%s) ok = %v, want %v", tc.name, tc.of, ok, tc.want)
			continue
		}
		r, isRefined := got.(*types.Refined)
		switch {
		case !ok && got != tc.of:
			t.Errorf("%s: Past(%s) = %s, want %s unchanged", tc.name, tc.of, got, tc.of)
		case ok && (!isRefined || !r.Past || r.Of != tc.of):
			t.Errorf("%s: Past(%s) = %#v, want a past Refined of it", tc.name, tc.of, got)
		}
	}
}

// TYPES.md §8.4: statically `past X` is X, assignable both ways; only its text says past.
func TestPastIsStaticallyX(t *testing.T) {
	for _, of := range []types.Type{element, reward, refStatus, &types.Alias{Name: "R", Def: reward}, &types.TypeAppType{Fn: armsFn(element, paramKind)}} {
		past, ok := types.Past(of)
		if !ok {
			t.Fatalf("Past(%s) refused", of)
		}
		switch {
		case past.Kind() != of.Kind():
			t.Errorf("past %s: Kind() = %d, want %d", of, past.Kind(), of.Kind())
		case past.Base() != of.Base():
			t.Errorf("past %s: Base() = %s, want %s", of, past.Base(), of.Base())
		case past.Underlying() != past:
			t.Errorf("past %s: Underlying() must keep the past layer", of)
		case !types.Identical(past, of) || !types.Identical(of, past):
			t.Errorf("past %s is not identical to %s", of, of)
		case !types.Assignable(past, of) || !types.Assignable(of, past):
			t.Errorf("past %s and %s are not assignable both ways", of, of)
		case !types.Assignable(list(past), list(of)) || !types.Assignable(list(of), list(past)):
			t.Errorf("[past %s] and [%s] are not assignable both ways", of, of)
		case past.String() != "past "+of.String():
			t.Errorf("past %s prints %q", of, past.String())
		}
		if j, ok := types.Join(past, of); !ok || !types.Identical(j, of) {
			t.Errorf("Join(past %s, %s) = %v, %v", of, of, j, ok)
		}
	}
	if types.Assignable(&types.Refined{Of: refStatus, Past: true}, refOther) {
		t.Errorf("a past ref of one collection is assignable to a ref of another")
	}
}

// armsFn is a type function over a match, one arm per result.
func armsFn(results ...types.Type) *types.TypeFunc {
	fn := &types.TypeFunc{Pkg: "resource.events", Name: "Pick"}
	for i, r := range results {
		fn.Arms = append(fn.Arms, &types.TypeArm{Members: []int{i}, Result: r})
	}
	return fn
}

// TYPES.md §8.4: `past F(x)` and `past match` need every branch an enum, a variant or a ref.
func TestPastTypeFunctions(t *testing.T) {
	other := &types.VariantType{Pkg: "resource.events", Name: "Cost", Tag: "kind"}
	refMonsters := &types.RefType{Target: monsters}
	for _, tc := range []struct {
		name string
		fn   *types.TypeFunc
		want bool
	}{
		{"every branch an enum", armsFn(element, paramKind), true},
		{"every branch a variant", armsFn(reward, other), true},
		{"every branch a ref", armsFn(refMonsters, refStatus), true},
		{"enums and variants mixed", armsFn(element, reward, &types.Alias{Name: "E", Def: paramKind}), true},
		{"a plain body that is an enum", &types.TypeFunc{Name: "Same", Body: element}, true},
		{"no branch resolved yet", &types.TypeFunc{Name: "Open"}, true},
		{"an Int branch", armsFn(element, types.IntType), false},
		{"a list branch", armsFn(element, list(element)), false},
		{"a Never branch", armsFn(refMonsters, types.NeverType), false},
		{"an optional branch", armsFn(element, &types.OptionalType{Elem: element}), false},
		{"a case branch", armsFn(reward, rewardItem), false},
		{"an application branch", armsFn(element, &types.TypeAppType{Fn: armsFn(element)}), false},
		{"a literal union body", &types.TypeFunc{Name: "SK", Body: overParam}, false},
	} {
		for _, of := range []types.Type{&types.TypeAppType{Fn: tc.fn}, &types.DepUnionType{Fn: tc.fn}} {
			if _, ok := types.Past(of); ok != tc.want {
				t.Errorf("%s: Past(%s) ok = %v, want %v", tc.name, of, ok, tc.want)
			}
		}
	}
}

// TYPES.md §11.2: a type function's branches are its plain body, or its arms' results in order.
func TestBranches(t *testing.T) {
	body := types.Branches(&types.TypeFunc{Body: element})
	arms := types.Branches(armsFn(reward, element))
	if len(body) != 1 || body[0] != element {
		t.Errorf("Branches of a plain body = %v", body)
	}
	if len(arms) != 2 || arms[0] != reward || arms[1] != element {
		t.Errorf("Branches of arms = %v", arms)
	}
	if got := types.Branches(paramFn); len(got) != 2 || got[1] != types.NeverType {
		t.Errorf("Branches(Param) = %v, want its ref and Never arms", got)
	}
}

// TYPES.md §8.4: a layer holding only `past` has no other refinement.
func TestPastOnly(t *testing.T) {
	of := types.IntType
	for _, tc := range []struct {
		name string
		r    *types.Refined
		want bool
	}{
		{"past alone", &types.Refined{Of: of, Past: true}, true},
		{"bare", &types.Refined{Of: of}, true},
		{"range", &types.Refined{Of: of, Past: true, Range: &types.Bound{}}, false},
		{"pattern", &types.Refined{Of: of, Pattern: regexp.MustCompile("a")}, false},
		{"where", &types.Refined{Of: of, Where: &types.Predicate{}}, false},
		{"asset", &types.Refined{Of: of, Asset: &types.AssetSpec{}}, false},
	} {
		if got := types.PastOnly(tc.r); got != tc.want {
			t.Errorf("%s: PastOnly = %v, want %v", tc.name, got, tc.want)
		}
	}
}
