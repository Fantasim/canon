package types_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

var (
	status     = &types.RecordType{Pkg: "teamboard", Name: "Status"}
	statuses   = &types.Collection{Kind: types.CollLet, Pkg: "teamboard", Name: "statuses", Elem: status}
	others     = &types.Collection{Kind: types.CollLet, Pkg: "teamboard", Name: "others", Elem: status}
	refStatus  = &types.RefType{Target: statuses}
	refOther   = &types.RefType{Target: others}
	label      = &types.Refined{Of: types.StringType, Range: &types.Bound{HasLo: true, Lo: types.Limit{I: 1}}}
	layout     = &types.Alias{Pkg: "teamboard", Name: "Layout", Def: label}
	rewardNone = &types.CaseType{Variant: reward, Name: "nothing", Wire: "nothing", Index: 1}
	optInt     = &types.OptionalType{Elem: types.IntType}
)

var otherFn = &types.TypeFunc{Pkg: "resource", Name: "Other", Body: types.IntType}

func list(t types.Type) *types.ListType { return &types.ListType{Elem: t} }

// TYPES.md §6.1: static identity ignores refinements, aliases, widths and record arguments.
func TestIdentical(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b types.Type
		want bool
	}{
		{"refinement removed", label, types.StringType, true},
		{"alias expanded", layout, types.StringType, true},
		{"integer widths", types.UInt16Type, types.IntType, true},
		{"Float32 is Float", types.Float32Type, types.FloatType, true},
		{"Int is not Float", types.IntType, types.FloatType, false},
		{"record arguments erased", &types.AppliedRecord{Rec: hourly}, hourly, true},
		{"refs compare collections", refStatus, &types.RefType{Target: statuses}, true},
		{"refs of other collections", refStatus, refOther, false},
		{"lists component-wise", list(label), list(types.StringType), true},
		{"keyed list compares its key", keyedEvents, list(eventType), false},
		{"type applications by function", &types.TypeAppType{Fn: paramFn}, &types.TypeAppType{Fn: paramFn}, true},
		{"cases by declaration", rewardItem, rewardNone, false},
	} {
		if got := types.Identical(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: Identical(%s, %s) = %v, want %v", tc.name, tc.a, tc.b, got, tc.want)
		}
	}
}

// TYPES.md §6.2: the rows of the assignability table, and what no row allows.
func TestAssignable(t *testing.T) {
	for _, tc := range []struct {
		name     string
		from, to types.Type
		want     bool
	}{
		{"same type, refinement checked later", types.IntType, penya, true},
		{"error both ways", types.ErrorType, status, true},
		{"never to anything", types.NeverType, status, true},
		{"none to optional", types.NoneType, optInt, true},
		{"wrap", types.IntType, optInt, true},
		{"present conversion", &types.OptionalType{Elem: status}, &types.OptionalType{Elem: refStatus}, true},
		{"optional never to plain (DECISIONS 17)", optInt, types.IntType, false},
		{"none to plain", types.NoneType, types.IntType, false},
		{"dereference", refStatus, status, true},
		{"entry to ref (TYP-02)", status, refStatus, true},
		{"refs of other collections", refStatus, refOther, false},
		{"case to variant", rewardItem, reward, true},
		{"variant to case", reward, rewardItem, false},
		{"list element-wise", list(status), list(refStatus), true},
		{"keyed list to list", keyedEvents, list(eventType), true},
		{"list to keyed list", list(eventType), keyedEvents, true},
		{"table to list", &types.TableType{Elem: status}, list(status), true},
		{"table to keyed list", &types.TableType{Elem: eventType}, keyedEvents, false},
		{"list to table", list(status), &types.TableType{Elem: status}, false},
		{"maps entry-wise", &types.MapType{Key: types.StringType, Value: status}, &types.MapType{Key: types.StringType, Value: refStatus}, true},
		{"dependent value kept", types.IntType, &types.DepUnionType{Fn: paramFn}, true},
		{"ref to a literal union over it", refStatus, &types.LitUnionType{Of: refStatus, Literals: []string{"all"}}, true},
		{"dependent value to its own type function (§11.4)", &types.DepUnionType{Fn: paramFn}, &types.DepUnionType{Fn: paramFn}, true},
		{"applied type function to the optional union", &types.TypeAppType{Fn: paramFn}, &types.OptionalType{Elem: &types.DepUnionType{Fn: paramFn}}, true},
		{"dependent value to another type function (§11.4)", &types.DepUnionType{Fn: paramFn}, &types.DepUnionType{Fn: otherFn}, false},
		{"dependent value to a plain type (§11.4)", &types.DepUnionType{Fn: paramFn}, types.IntType, false},
		{"literal union base", types.StringType, &types.LitUnionType{Of: types.StringType, Literals: []string{"x"}}, true},
		{"no Int to Float", types.IntType, types.FloatType, false},
		{"no Int to Duration", types.IntType, types.DurationType, false},
		{"no enum to String", element, types.StringType, false},
		{"function results covariant", &types.FuncType{Params: []types.Type{types.IntType}, Result: status}, &types.FuncType{Params: []types.Type{types.IntType}, Result: refStatus}, true},
		{"function parameters identical", &types.FuncType{Params: []types.Type{status}, Result: status}, &types.FuncType{Params: []types.Type{refStatus}, Result: status}, false},
	} {
		if got := types.Assignable(tc.from, tc.to); got != tc.want {
			t.Errorf("%s: Assignable(%s, %s) = %v, want %v", tc.name, tc.from, tc.to, got, tc.want)
		}
	}
}

// TYPES.md §6.4 (TYP-05): the rows of the join table; anything else is E3308.
func TestJoin(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b types.Type
		want string
	}{
		{"same type without refinements", label, label, "String"},
		{"none and T", types.NoneType, types.IntType, "Int?"},
		{"T? and T", optInt, types.IntType, "Int?"},
		{"T and ref T", refStatus, status, "teamboard.Status"},
		{"two cases", rewardItem, rewardNone, "resource.events.Reward"},
		{"case and its variant", reward, rewardItem, "resource.events.Reward"},
		{"refs of one collection", refStatus, refStatus, "ref teamboard.statuses"},
		{"lists component-wise", list(refStatus), list(status), "[teamboard.Status]"},
		{"maps component-wise", &types.MapType{Key: types.StringType, Value: optInt}, &types.MapType{Key: types.StringType, Value: types.IntType}, "{String: Int?}"},
		{"anything else", types.IntType, types.StringType, ""},
		{"refs of two collections", refStatus, refOther, ""},
	} {
		got, ok := types.Join(tc.a, tc.b)
		text := ""
		if ok {
			text = got.String()
		}
		if text != tc.want {
			t.Errorf("%s: Join(%s, %s) = %q, want %q", tc.name, tc.a, tc.b, text, tc.want)
		}
	}
}
