package types_test

import (
	"math"
	"regexp"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// Fixtures shaped like the examples: an event package with a dependent type and a parameterized record.
var (
	element    = &types.EnumType{Pkg: "resource.vocab", Name: "Element", Members: []*types.Member{{Name: "FIRE", Wire: "FIRE", Code: 1, HasCode: true}}}
	paramKind  = &types.EnumType{Pkg: "resource.vocab", Name: "ParamKind", Members: []*types.Member{{Name: "monster"}, {Name: "none_", Index: 1}, {Name: "stat", Index: 2}}}
	eventType  = &types.RecordType{Pkg: "resource.events", Name: "EventType", Fields: []*types.Field{{Name: "param", Wire: "param", WirePath: []string{"param"}, Type: paramKind}}}
	monsters   = &types.Collection{Kind: types.CollDefines, Pkg: "resource.vocab", Name: "monsters", Elem: types.DefineType, Local: true}
	eventTypes = &types.Collection{Kind: types.CollLet, Pkg: "resource.events", Name: "eventTypes", Elem: eventType}
	param      = &types.Param{Name: "e", Type: &types.RefType{Target: eventTypes}}
	paramFn    = &types.TypeFunc{
		Pkg: "resource.events", Name: "Param", Params: []*types.Param{param},
		Scrutinee: &types.Scrutinee{Param: param, Path: eventType.Fields, Type: paramKind},
		Arms: []*types.TypeArm{
			{Members: []int{0}, Result: &types.RefType{Target: monsters}},
			{Members: []int{1, 2}, Result: types.NeverType},
		},
	}
	eventField  = &types.Field{Name: "eventType", Type: &types.RefType{Target: eventTypes}}
	hourly      = &types.RecordType{Pkg: "resource.events", Name: "HourlyTarget", Params: []*types.Param{param}}
	reward      = &types.VariantType{Pkg: "resource.events", Name: "Reward", Tag: "kind"}
	rewardItem  = &types.CaseType{Variant: reward, Name: "item", Wire: "item"}
	talentNode  = &types.RecordType{Pkg: "resource.rules", Name: "TalentNode"}
	talentTree  = &types.RecordType{Pkg: "resource.rules", Name: "GuildTalentTree"}
	talentNodes = &types.Collection{Kind: types.CollField, Owner: talentTree, FieldPath: []string{"nodes"}, Elem: talentNode}
	penya       = &types.Alias{Pkg: "resource.farm", Name: "Penya", Def: &types.Refined{Of: types.IntType, Range: &types.Bound{HasLo: true}}}
	keyedEvents = &types.ListType{Elem: eventType, KeyedBy: eventType.Fields[0]}
	positive    = &types.Predicate{Text: "it > 0"}
)

var kindCases = []struct {
	t    types.Type
	kind types.Kind
	text string
}{
	{types.BoolType, types.Bool, "Bool"},
	{types.IntType, types.Int, "Int"},
	{types.Int8Type, types.Int, "Int8"},
	{types.Int16Type, types.Int, "Int16"},
	{types.Int32Type, types.Int, "Int32"},
	{types.UInt8Type, types.Int, "UInt8"},
	{types.UInt16Type, types.Int, "UInt16"},
	{types.UInt32Type, types.Int, "UInt32"},
	{types.UInt64Type, types.Int, "UInt64"},
	{types.FloatType, types.Float, "Float"},
	{types.Float32Type, types.Float, "Float32"},
	{types.StringType, types.String, "String"},
	{types.DurationType, types.Duration, "Duration"},
	{types.AnyType, types.Any, "_"},
	{types.NoneType, types.None, "none"},
	{types.ErrorType, types.Error, "invalid"},
	{types.NeverType, types.Never, "Never"},
	{types.RangeType, types.Range, "Range"},
	{element, types.Enum, "resource.vocab.Element"},
	{eventType, types.Record, "resource.events.EventType"},
	{types.DefineType, types.Define, "Define"},
	{reward, types.Variant, "resource.events.Reward"},
	{rewardItem, types.Case, "resource.events.Reward.item"},
	{&types.VariantKindType{Variant: reward}, types.VariantKind, "Kind(resource.events.Reward)"},
	{penya, types.Int, "resource.farm.Penya"},
	{&types.Refined{Of: types.IntType, Range: &types.Bound{HasLo: true, HasHi: true, HiIncluded: true, Hi: types.Limit{I: 100}}}, types.Int, "Int(0..=100)"},
	{&types.Refined{Of: types.DurationType, Range: &types.Bound{HasLo: true, HasHi: true, HiIncluded: true, Lo: types.Limit{I: 1000}, Hi: types.Limit{I: 86_400_000}}}, types.Duration, "Duration(1s..=1d)"},
	{&types.Refined{Of: types.FloatType, Range: &types.Bound{HasLo: true, Lo: types.Limit{F: 0.5}}}, types.Float, "Float(0.5..)"},
	{&types.Refined{Of: types.StringType, Range: &types.Bound{HasHi: true, Hi: types.Limit{I: 33}}, Pattern: regexp.MustCompile(`^[a-z]+$`)}, types.String, "String(..33, /^[a-z]+$/)"},
	{&types.Refined{Of: &types.Refined{Of: penya, Range: &types.Bound{HasHi: true, HiIncluded: true, Hi: types.Limit{I: 1000}}}, Where: positive}, types.Int, "resource.farm.Penya(..=1000) where it > 0"},
	{&types.Refined{Of: &types.OptionalType{Elem: types.IntType}, Where: positive}, types.Optional, "Int? where it > 0"},
	{&types.Refined{Of: keyedEvents, Range: &types.Bound{HasHi: true, HiIncluded: true, Hi: types.Limit{I: 256}}}, types.List, "[resource.events.EventType](..=256) keyed by param"},
	{&types.Refined{Of: types.StringType, Asset: &types.AssetSpec{Root: "@resource/Icon/Item", Exts: []string{"dds", "tar.gz"}}}, types.String, `asset("@resource/Icon/Item", ext: [dds, "tar.gz"])`},
	{&types.Refined{Of: types.StringType, Asset: &types.AssetSpec{Root: "@resource/Icon"}}, types.String, `asset("@resource/Icon")`},
	{&types.ListType{Elem: types.IntType}, types.List, "[Int]"},
	{keyedEvents, types.List, "[resource.events.EventType] keyed by param"},
	{&types.MapType{Key: types.StringType, Value: types.IntType}, types.Map, "{String: Int}"},
	{&types.DepMapType{Binder: "e", Coll: eventTypes, Value: &types.AppliedRecord{Rec: hourly, Args: []*types.Arg{{Source: types.ArgKey, Binder: "e"}}}}, types.DepMap, "{e in resource.events.eventTypes: resource.events.HourlyTarget(e)}"},
	{&types.TableType{Elem: eventType}, types.Table, "table resource.events.EventType"},
	{&types.TableType{Elem: eventType, Stable: true}, types.Table, "stable table resource.events.EventType"},
	{&types.RefType{Target: eventTypes}, types.Ref, "ref resource.events.eventTypes"},
	{&types.RefType{Target: monsters}, types.Ref, "ref resource.vocab.monsters"},
	{&types.RefType{Target: talentNodes}, types.Ref, "ref resource.rules.TalentNode"},
	{&types.OptionalType{Elem: types.IntType}, types.Optional, "Int?"},
	{&types.OptionalType{Elem: &types.FuncType{Params: []types.Type{types.IntType}, Result: types.IntType}}, types.Optional, "(fn(Int) -> Int)?"},
	{&types.OptionalType{Elem: &types.Refined{Of: types.IntType, Where: positive}}, types.Optional, "(Int where it > 0)?"},
	{&types.OptionalType{Elem: &types.Refined{Of: types.IntType, Range: &types.Bound{HasLo: true, Lo: types.Limit{I: -3}}}}, types.Optional, "Int(-3..)?"},
	{&types.LitUnionType{Of: &types.TypeAppType{Fn: paramFn, Args: []*types.Arg{{Source: types.ArgField, Path: []*types.Field{eventField}}}}, Literals: []string{"default"}}, types.LitUnion, `resource.events.Param(eventType) | "default"`},
	{&types.FuncType{Params: []types.Type{types.IntType, types.StringType}, Result: types.BoolType}, types.Func, "fn(Int, String) -> Bool"},
	{&types.PairType{A: types.IntType, B: types.StringType}, types.Pair, "Pair(Int, String)"},
	{&types.AppliedRecord{Rec: hourly, Args: []*types.Arg{{Source: types.ArgParam, Param: param, Path: eventType.Fields}}}, types.Record, "resource.events.HourlyTarget(e.param)"},
	{&types.TypeAppType{Fn: paramFn, Args: []*types.Arg{{Source: types.ArgParam, Param: param}}}, types.TypeApp, "resource.events.Param(e)"},
	{&types.DepUnionType{Fn: paramFn}, types.DepUnion, "resource.events.Param(*)"},
}

// TYPES.md §2: every kind, its Kind and its canonical type text.
func TestKindAndText(t *testing.T) {
	for _, c := range kindCases {
		if got := c.t.Kind(); got != c.kind {
			t.Errorf("%s: Kind() = %d, want %d", c.text, got, c.kind)
		}
		if got := c.t.String(); got != c.text {
			t.Errorf("String() = %q, want %q", got, c.text)
		}
	}
}

// TYP-04: Base removes alias and refinement layers at the top; Underlying removes aliases only.
func TestBaseAndUnderlying(t *testing.T) {
	refined := &types.Refined{Of: penya, Where: positive}
	if refined.Base() != types.IntType || penya.Base() != types.IntType {
		t.Errorf("Base of a refined alias of Int(0..) is not Int")
	}
	if penya.Underlying() != penya.Def || refined.Underlying() != refined {
		t.Errorf("Underlying keeps refinements and expands aliases")
	}
	for _, c := range kindCases {
		if c.t == penya || isRefined(c.t) {
			continue
		}
		if c.t.Base() != c.t || c.t.Underlying() != c.t {
			t.Errorf("%s: Base and Underlying of a type without layers must be itself", c.text)
		}
	}
}

func isRefined(t types.Type) bool {
	_, ok := t.(*types.Refined)
	return ok
}

// TYPES.md §7.2: the implicit range of every integer type and of a stored Duration.
func TestLimits(t *testing.T) {
	cases := []struct {
		b      types.Basic
		lo, hi int64
		ok     bool
	}{
		{types.IntType, math.MinInt64, math.MaxInt64, true},
		{types.Int8Type, -128, 127, true},
		{types.Int16Type, -32768, 32767, true},
		{types.Int32Type, math.MinInt32, math.MaxInt32, true},
		{types.UInt8Type, 0, 255, true},
		{types.UInt16Type, 0, 65535, true},
		{types.UInt32Type, 0, math.MaxUint32, true},
		{types.UInt64Type, 0, math.MaxInt64, true},
		{types.DurationType, -types.DurationLimit, 9_223_372_036_854, true},
		{types.StringType, 0, 0, false},
		{types.Float32Type, 0, 0, false},
	}
	for _, c := range cases {
		lo, hi, ok := c.b.Limits()
		if lo != c.lo || hi != c.hi || ok != c.ok {
			t.Errorf("%s.Limits() = %d, %d, %v; want %d, %d, %v", c.b, lo, hi, ok, c.lo, c.hi, c.ok)
		}
	}
}

// WIRE.md §5.1: the units of @json(unit:) and their size in milliseconds.
func TestUnits(t *testing.T) {
	cases := []struct {
		u    types.Unit
		text string
		ms   int64
	}{{types.UnitMs, "ms", 1}, {types.UnitS, "s", 1_000}, {types.UnitM, "m", 60_000}, {types.UnitH, "h", 3_600_000}, {types.UnitD, "d", 86_400_000}}
	for _, c := range cases {
		if c.u.String() != c.text || c.u.Millis() != c.ms {
			t.Errorf("unit %s = %d ms, want %s = %d ms", c.u, c.u.Millis(), c.text, c.ms)
		}
	}
	if (types.Field{}).Unit != types.UnitMs || (types.Field{}).Enc != types.EncPlain {
		t.Errorf("the zero field has the default unit ms and no encoding")
	}
	bits := types.Field{Name: "flags", Enc: types.EncBits, Pairs: &types.Pairs{Keys: [2]string{"k{i}", "v{i}"}, Slots: 6}}
	if bits.Enc == types.EncInt || bits.Pairs.Slots != 6 {
		t.Errorf("field encodings are distinct")
	}
}

// TYPES.md §11.2: the arm of a type-level match that covers a member.
func TestTypeFuncArm(t *testing.T) {
	if a := paramFn.Arm(0); a == nil || a.Result.Kind() != types.Ref {
		t.Errorf("Arm(monster) = %v, want the ref arm", a)
	}
	if a := paramFn.Arm(2); a == nil || a.Result != types.NeverType {
		t.Errorf("Arm(stat) = %v, want the Never arm", a)
	}
	if a := paramFn.Arm(3); a != nil {
		t.Errorf("Arm(3) = %v, want nil", a)
	}
	wild := &types.TypeFunc{Arms: []*types.TypeArm{{Wildcard: true, Result: types.StringType}}}
	if a := wild.Arm(7); a == nil || a.Result != types.StringType {
		t.Errorf("a wildcard arm covers every member")
	}
}
