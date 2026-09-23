package ir_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

var (
	intRef    = ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true}
	boolRef   = ir.TypeRef{Kind: types.Bool}
	stringRef = ir.TypeRef{Kind: types.String}
	paramKind = &ir.Enum{Pkg: "resource.vocab", Name: "ParamKind", Members: []*ir.EnumMember{{Name: "monster", Wire: "monster"}, {Name: "none_", Wire: "none_", Index: 1}, {Name: "element", Wire: "element", Index: 2}}}
	element   = &ir.Enum{Pkg: "resource.vocab", Name: "Element", Codes: &ir.TypeRef{Kind: types.Int, Bits: 8}, JSONCodes: true, CppDefines: "ELEM_", Members: []*ir.EnumMember{{Name: "FIRE", Wire: "FIRE", Code: 1, Retired: true, Deprecated: true}}}
	statuses  = &ir.RefTarget{Coll: types.CollLet, Pkg: "teamboard", Value: "statuses", Elem: &ir.Record{Pkg: "teamboard", Name: "Status"}}
	status    = &ir.Record{Pkg: "teamboard", Name: "Status", Fields: []*ir.Field{
		{Name: "label", WirePath: []string{"label"}, Type: stringRef},
		{Name: "next", WirePath: []string{"next"}, Type: ir.TypeRef{Kind: types.List, Elem: &ir.TypeRef{Kind: types.Ref, Key: &stringRef, Ref: statuses}}},
		{Name: "limit", WirePath: []string{"limits", "max"}, Type: ir.TypeRef{Kind: types.Int, Bits: 16}, Optional: true, NoneWire: []byte("-1")},
	}}
	// Param(e: EventType) = match e.param { monster => ref monsters, none_ => Never, element => Element }
	param = &ir.Dependent{
		Pkg: "resource.events", Name: "Param", Params: 1, DiscParam: 0, DiscPath: []string{"param"},
		Disc: &ir.TypeRef{Kind: types.Enum, Named: paramKind},
		Branches: []*ir.Branch{
			{Name: "monster", Members: []int{0}, Type: ir.TypeRef{Kind: types.Ref, Key: &stringRef, Ref: &ir.RefTarget{Coll: types.CollDefines, Pkg: "resource.vocab", Value: "monsters", Local: true}}},
			{Name: "element", Members: []int{2}, Type: ir.TypeRef{Kind: types.Enum, Named: element}},
		},
		ByMember: []int{0, ir.NoBranch, 1},
	}
	reward = &ir.Variant{Pkg: "resource.events", Name: "Reward", Tag: "kind", Cases: []*ir.Case{
		{Name: "nothing", Wire: "nothing"},
		{Name: "item", Wire: "Item", Retired: true, Cpp: ir.CppCaseOptions{Value: 2, HasValue: true}, Fields: []*ir.Field{{Name: "count", WirePath: []string{"count"}, Type: intRef, Default: &value.Int{V: 1, T: types.IntType}}}},
	}}
)

// FINGERPRINT.md §4.3: a TypeRef points at the named IR type, of this package or another.
func TestQName(t *testing.T) {
	cases := []struct {
		t    ir.Type
		want string
	}{{status, "teamboard.Status"}, {element, "resource.vocab.Element"}, {reward, "resource.events.Reward"}, {param, "resource.events.Param"}}
	for _, c := range cases {
		if got := c.t.QName(); got != c.want {
			t.Errorf("QName() = %q, want %q", got, c.want)
		}
	}
	applied := ir.TypeRef{Kind: types.TypeApp, Named: param, Args: []*ir.Source{{From: types.ArgField, WirePath: []string{"eventType"}}}}
	if applied.Named.QName() != "resource.events.Param" || applied.Args[0].From != types.ArgField {
		t.Errorf("a dependent field names its type and the source of each argument")
	}
	caseRef := ir.TypeRef{Kind: types.Case, Named: reward, Case: reward.Cases[1]}
	if caseRef.Case.Wire != "Item" || !caseRef.Case.Retired || caseRef.Case.Cpp.Value != 2 {
		t.Errorf("a case type names its variant and case")
	}
}

// DEP-03: branches in arm order; a Never arm has no branch.
func TestDependentBranches(t *testing.T) {
	var got []string
	for m, b := range param.ByMember {
		if b == ir.NoBranch {
			got = append(got, paramKind.Members[m].Wire+"=never")
			continue
		}
		got = append(got, paramKind.Members[m].Wire+"="+param.Branches[b].Name)
	}
	if want := "monster=monster none_=never element=element"; strings.Join(got, " ") != want {
		t.Errorf("arms = %q, want %q", strings.Join(got, " "), want)
	}
}

// CODEGEN.md §2.1: targets, modes and C++ access modes are distinct values.
func TestEnums(t *testing.T) {
	targets := []ir.Target{ir.TargetGo, ir.TargetCpp, ir.TargetTS, ir.TargetJSON, ir.TargetView}
	modes := []ir.Mode{ir.ModeNone, ir.ModeBaked, ir.ModeEmbedded, ir.ModeData, ir.ModeTypes}
	access := []ir.Access{ir.AccessNone, ir.AccessFields, ir.AccessBoth, ir.AccessGetters}
	kinds := []ir.FnKind{ir.FnPrecomputed, ir.FnLookup, ir.FnTranslated}
	ops := []ir.Op{ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod, ir.OpNeg, ir.OpNot, ir.OpAnd, ir.OpOr, ir.OpEq, ir.OpNe, ir.OpLt, ir.OpLe, ir.OpGt, ir.OpGe}
	builtins := []ir.Builtin{ir.BuiltinFloat, ir.BuiltinInt, ir.BuiltinMin, ir.BuiltinMax, ir.BuiltinAbs, ir.BuiltinClamp, ir.BuiltinFloor, ir.BuiltinCeil, ir.BuiltinRound}
	for _, n := range []int{distinct(targets), distinct(modes), distinct(access), distinct(kinds), distinct(ops), distinct(builtins)} {
		if n != 0 {
			t.Errorf("%d repeated values", n)
		}
	}
}

func distinct[T comparable](xs []T) int {
	seen := map[T]bool{}
	for _, x := range xs {
		seen[x] = true
	}
	return len(xs) - len(seen)
}

// CONFORMANCE.md §2.3: the translated `healFor(self, missingHp: Int) -> Int { min(heal, max(missingHp, 0)) }`.
func TestTranslatedBody(t *testing.T) {
	zero := &ir.Lit{T: intRef, V: &value.Int{V: 0, T: types.IntType}}
	body := &ir.Call{T: intRef, Fn: ir.BuiltinMin, Args: []ir.PExpr{&ir.ReadRef{T: intRef}, &ir.Call{T: intRef, Fn: ir.BuiltinMax, Args: []ir.PExpr{&ir.ParamRef{T: intRef}, zero}}}}
	fn := &ir.ExportFn{
		Name: "healFor", Kind: ir.FnTranslated, Params: []*ir.Param{{Name: "missingHp", Type: intRef}}, Result: intRef,
		Body: body, Reads: []*ir.Read{{Name: "heal", Path: []string{"heal"}, Type: intRef}},
		Vectors: []*ir.Vector{
			{Recv: []value.Value{&value.Int{V: 500, T: types.IntType}}, Args: []value.Value{&value.Int{V: 200, T: types.IntType}}, Want: &value.Int{V: 200, T: types.IntType}, TSWant: &value.Int{V: 200, T: types.IntType}},
			{Recv: []value.Value{&value.Int{V: 500, T: types.IntType}}, Args: []value.Value{&value.Int{V: -1 << 63, T: types.IntType}}, Want: &value.Int{V: 0, T: types.IntType}, TSCode: diag.E8303.Def().Code},
		},
	}
	if fn.Body.Type().Kind != types.Int || fn.Vectors[1].TSCode != diag.E8303.Def().Code || fn.Vectors[1].Code != "" {
		t.Errorf("a translated fn keeps its typed body and per-target expectations")
	}
	nodes := []ir.PExpr{
		zero, body, &ir.ParamRef{T: intRef}, &ir.ReadRef{T: intRef, Index: 0}, &ir.LocalRef{T: intRef, Name: "x"},
		&ir.Unary{T: intRef, Op: ir.OpNeg, X: zero}, &ir.Binary{T: boolRef, Op: ir.OpLt, X: zero, Y: zero},
		&ir.CallFn{T: intRef, Fn: fn, Args: []ir.PExpr{zero}}, &ir.If{T: intRef, Cond: &ir.Lit{T: boolRef}, Then: zero, Else: zero},
		&ir.Let{T: intRef, Name: "x", Value: zero, Body: &ir.LocalRef{T: intRef, Name: "x"}},
		&ir.Template{T: stringRef, Parts: []ir.TemplatePart{{Text: "heal ", X: zero}, {Text: "!"}}},
		&ir.Coalesce{T: intRef, X: &ir.ReadRef{T: ir.TypeRef{Kind: types.Optional, Elem: &intRef}}, Y: zero},
		&ir.IsCase{T: boolRef, X: &ir.ReadRef{T: ir.TypeRef{Kind: types.Variant, Named: reward}}, Case: 1},
	}
	for _, n := range nodes {
		if k := n.Type().Kind; k != types.Int && k != types.Bool && k != types.String {
			t.Errorf("%T: Type() = %d", n, k)
		}
	}
}

// CODEGEN.md §5.12: an input keeps its own refinements for the generated loader; defines their values.
func TestInputsAndDefines(t *testing.T) {
	key := &ir.Field{
		Name: "apiKey", Type: stringRef, Optional: true, Input: &types.Input{Env: "RESOURCESTUDIO_GEMINI_KEY"},
		Range: &types.Bound{HasLo: true, Lo: types.Limit{I: 1}}, Pattern: regexp.MustCompile(`^[A-Za-z0-9_-]+$`),
	}
	if !key.Pattern.MatchString("abc") || key.Range.Lo.I != 1 || key.Input.Env == "" {
		t.Errorf("an input field keeps its refinements")
	}
	p := &ir.Package{Defines: []*ir.DefineTable{{Pkg: "resource.vocab", Value: "items", Names: []string{"II_GEN_GOLD", "II_POT_HEAL_L"}, Values: []int64{12, 3}}}}
	if d := p.Defines[0]; len(d.Names) != len(d.Values) || d.Names[0] > d.Names[1] {
		t.Errorf("a define table is sorted by name, one value per name")
	}
}

// CG-08: a lookup table is dense, row-major, the first parameter varying slowest.
func TestLookupTable(t *testing.T) {
	f, tr := &value.Bool{V: false}, &value.Bool{V: true}
	table := &ir.LookupTable{Domains: [][]value.Value{{f, tr}, {f, tr}}, Cells: []value.Value{f, tr, tr, f}}
	xor := &ir.ExportFn{Name: "xor", Kind: ir.FnLookup, Params: []*ir.Param{{Name: "a", Type: boolRef}, {Name: "b", Type: boolRef}}, Result: boolRef, Table: table}
	row, col := 1, 0
	if got := xor.Table.Cells[row*len(table.Domains[1])+col]; got != tr {
		t.Errorf("xor(true, false) = %v", got.CanonText())
	}
	strong := &ir.ExportFn{Name: "isStrong", Kind: ir.FnPrecomputed, Result: boolRef, Instances: []*ir.Instance{{Recv: &value.Record{}, Result: tr}}}
	always := &ir.ExportFn{Name: "always", Kind: ir.FnPrecomputed, Result: boolRef, Value: tr}
	if strong.Instances[0].Result != tr || always.Value != tr || (&ir.Instance{Table: table}).Table.Domains[0][1] != tr {
		t.Errorf("precomputed results are kept per receiver instance")
	}
}
