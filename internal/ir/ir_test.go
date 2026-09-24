package ir_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
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

// TestLambdaParamIsNotComputed is TYPES.md §15, decision 80: a field default that reads the owner record's own parameter is Computed, but a lambda's own parameter of the same kind, bound inside that default, is not.
func TestLambdaParamIsNotComputed(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(`package a

/// R.
record R(n: Int) {
  /// Reads its own parameter: computed.
  double: Int = n * 2
  /// A lambda's own parameter must not count as reading the instance.
  total: Int = [1, 2, 3].map(x => x + 1).sum()
}

emit go { out: "@features/a", package: "a" }
`))
	w.calls = w.fixtureCalls
	pkgs := w.build(t)
	if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	rec, ok := pkgs[0].Types[0].(*ir.Record)
	if !ok || len(rec.Fields) != 2 || !rec.Fields[0].Computed || rec.Fields[1].Computed {
		t.Errorf("double must be Computed and total must not be")
	}
}

// TestLookupTableCallsEveryCell is EVALUATION.md §2.3 and decision 194: a failing call does not stop the rest of the domain.
func TestLookupTableCallsEveryCell(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(`package a

/// A grade.
enum Grade { normal, unique }

/// A tier.
enum Tier { vagrant, expert }

/// Whether tier may use grade: a 2 x 2 table, every cell always fails here.
export fn canUse(grade: Grade, tier: Tier) -> Bool {
  return true
}

emit go { out: "@features/a", package: "a" }
`))
	calls := 0
	w.calls = func(check.Object, value.Value, []value.Value) (value.Value, bool) {
		calls++
		return nil, false
	}
	pkgs := w.build(t)
	if calls != 4 {
		t.Errorf("Host.Call was invoked %d times, want 4 (one per cell of the 2x2 domain)", calls)
	}
	for _, fn := range pkgs[0].Fns {
		if fn.Name == "canUse" && (fn.Table != nil || fn.Value != nil) {
			t.Errorf("a fn with a failing cell must stay uncomputed")
		}
	}
}

// TestWildcardBranchName is decision 194: a wildcard `_ =>` arm's branch is named after the first member it covers, in declaration order.
func TestWildcardBranchName(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(`package a

/// A kind.
enum Kind { alpha, beta, gamma }

/// A thing.
record Thing {
  /// Its kind.
  k: Kind
}

/// What a thing's payload is.
type Payload(t: Thing) = match t.k {
  alpha => Int
  _ => String
}

/// A thing.
let thing: Thing = { k: alpha }

emit go { out: "@features/a", package: "a" }
`))
	w.calls = w.fixtureCalls
	pkgs := w.build(t)
	if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	dep, ok := pkgs[0].Types[2].(*ir.Dependent)
	if !ok || len(dep.Branches) != 2 || dep.Branches[1].Name != "beta" {
		t.Errorf("the wildcard branch should be named beta, the first member it covers")
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

// TestEveryReceiverIsEvaluated is EVALUATION.md §2.3 and decision 194: a method that fails on one receiver is still called on every other, each failure reported by the host (a false result follows its finding), and only the failed receivers lack an Instance.
func TestEveryReceiverIsEvaluated(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(`package a

/// A gem.
record Gem {
  /// Its power.
  power: Int

  /// Whether it is strong.
  export fn isStrong(self) -> Bool { return power >= 2 }
}

/// The gems.
let gems: [Gem] = [{ power: 1 }, { power: 3 }]

emit go { out: "@features/a", package: "a" }
`))
	w.add(t, "a.gems.json", []byte(`[{"power": 1}, {"power": 3}]`))
	failures := 0
	w.calls = func(check.Object, value.Value, []value.Value) (value.Value, bool) {
		failures++
		return nil, false
	}
	pkgs := w.build(t)
	if failures != 2 {
		t.Errorf("Host.Call failed %d times, want 2: one call, one failure and so one host finding per receiver", failures)
	}
	rec, ok := pkgs[0].Types[0].(*ir.Record)
	if !ok || len(rec.Methods) != 1 || len(rec.Methods[0].Instances) != 0 {
		t.Errorf("a receiver whose call failed must have no Instance")
	}
}
