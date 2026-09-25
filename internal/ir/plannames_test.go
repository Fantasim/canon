package ir_test

import (
	"fmt"
	"regexp"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// TestInputReasonText is CODEGEN.md §5.12 (log-2026-09-24 "W2 runtime inputs": the failure lines, both targets byte for byte; "W2 gen/cpp inputs review": a Float32 reads Float, a Duration beyond the limit is not a valid Duration, the enum line names the Canon enum).
func TestInputReasonText(t *testing.T) {
	tone := &ir.Enum{Name: "Tone", Cpp: ir.NameOptions{Name: "CppTone"}}
	for _, c := range []struct {
		r    ir.InputReason
		t    ir.TypeRef
		want string
	}{
		{ir.InputNotSet, ir.TypeRef{Kind: types.String}, "not set"},
		{ir.InputNotValid, ir.TypeRef{Kind: types.Int, Bits: 32}, "not a valid Int"},
		{ir.InputNotValid, ir.TypeRef{Kind: types.Float, Bits: 32}, "not a valid Float"},
		{ir.InputNotValid, ir.TypeRef{Kind: types.Bool}, "not a valid Bool"},
		{ir.InputNotValid, ir.TypeRef{Kind: types.Duration}, "not a valid Duration"},
		{ir.InputNotValid, ir.TypeRef{Kind: types.String}, "not a valid String"},
		{ir.InputNotValid, ir.TypeRef{Kind: types.Record}, ""},
		{ir.InputOutsideRange, ir.TypeRef{Kind: types.String}, "outside its refinement range"},
		{ir.InputNoMatch, ir.TypeRef{Kind: types.String}, "does not match its pattern"},
		{ir.InputNotMember, ir.TypeRef{Kind: types.Enum, Named: tone}, "not a member of Tone"},
		{ir.InputNotMember, ir.TypeRef{Kind: types.Int}, ""},
	} {
		if got := ir.InputReasonText(c.r, c.t); got != c.want {
			t.Errorf("InputReasonText(%d, %v) = %q, want %q", c.r, c.t.Kind, got, c.want)
		}
	}
	if got := ir.InputLine("API_KEY", "not set"); got != "API_KEY: not set" {
		t.Errorf("InputLine = %q", got)
	}
}

// A LoadInputs failure line, the same bytes in every target; a Float32 input reads Float (CODEGEN.md §5.12).
func ExampleInputReasonText() {
	f32 := ir.TypeRef{Kind: types.Float, Bits: 32}
	fmt.Println(ir.InputLine("RATE", ir.InputReasonText(ir.InputNotValid, f32)))
	// Output: RATE: not a valid Float
}

// inputPackage has the input fields of the logged slot collisions (log-2026-09-24 "gen/go runtime inputs landed"): Gen.flagX and GenFlag.x, code with a pattern and codePattern, A_b.c and A.b_c.
func inputPackage(target ir.Target) (*ir.Package, map[*ir.Field]*ir.Record) {
	in := func(name string, opt bool) *ir.Field {
		return &ir.Field{Name: name, Type: ir.TypeRef{Kind: types.String}, Optional: opt, Input: &types.Input{Env: name}}
	}
	code := in("code", false)
	code.Pattern = regexp.MustCompile("^[a-z]+$")
	recs := []*ir.Record{
		{Pkg: "a", Name: "Gen", Fields: []*ir.Field{in("flagX", true), code, in("codePattern", true)}},
		{Pkg: "a", Name: "GenFlag", Fields: []*ir.Field{in("x", true)}},
		{Pkg: "a", Name: "A_b", Fields: []*ir.Field{in("c", true)}},
		{Pkg: "a", Name: "A", Fields: []*ir.Field{in("b_c", true)}},
	}
	p := &ir.Package{Name: "a", Emits: []*ir.Emit{{Target: target, Mode: ir.ModeData, GoPackage: "a", Namespace: "a"}}}
	owner := map[*ir.Field]*ir.Record{}
	for _, r := range recs {
		p.Types = append(p.Types, r)
		for _, f := range r.Fields {
			owner[f] = r
		}
	}
	return p, owner
}

// TestInputSlotsCollisionFree is CODEGEN.md §3.5, §5.12, §7.7: two valid input fields never share a slot, the plan reports nothing, and a slot is declared where gen/go and gen/cpp write it.
func TestInputSlotsCollisionFree(t *testing.T) {
	p, owner := inputPackage(ir.TargetGo)
	gp := ir.PlanGoNames(p, p.Emits[0])
	pkg := ir.GoScopeNames(gp)["package"]
	var goSlots []string
	for f, rec := range owner { //canon:unordered collected, then sorted
		in := gp.Input(rec, f)
		for _, n := range []string{in.Value, in.OK, in.Pattern} {
			if n != "" && !pkg[n] {
				t.Errorf("go slot %s of %s.%s is not declared", n, rec.Name, f.Name)
			}
			if n != "" {
				goSlots = append(goSlots, n)
			}
		}
	}
	if slices.Sort(goSlots); len(slices.Compact(slices.Clone(goSlots))) != len(goSlots) || len(gp.Problems()) > 0 {
		t.Errorf("go slots %v, problems %+v", goSlots, gp.Problems())
	}
	names := gp.Inputs()
	if names.Func != "LoadInputs" || names.Loaded != "inputsLoaded_" || names.Errs != "errs" || !pkg[names.Func] || !pkg[names.Loaded] {
		t.Errorf("go inputs %+v", names)
	}
	p, owner = inputPackage(ir.TargetCpp)
	cp := ir.PlanCppNames(p, p.Emits[0])
	scopes, cin := ir.CppScopeNames(cp), cp.Inputs()
	if cin.Func != "LoadInputs" || cin.Namespace != "AInputs" || cin.Loaded != "AInputsLoaded" || !scopes["detail"][cin.Namespace] || !scopes["a"][cin.Func] {
		t.Errorf("cpp inputs %+v", cin)
	}
	for f, rec := range owner { //canon:unordered each slot checked alone
		class, slot := cp.InputSlot(rec, f)
		if !scopes[cin.Namespace][class] || !scopes[cin.Namespace+"::"+class][slot] || scopes[class][slot] {
			t.Errorf("cpp slot %s::%s of %s.%s", class, slot, rec.Name, f.Name)
		}
	}
	if len(cp.Problems()) > 0 {
		t.Errorf("cpp problems %+v", cp.Problems())
	}
}

// dependentPackage is a dependent type Param over an enum, its branches a ref into a load.defines table, an Int and a keyword-named String; overrides name it Q in Go and R in C++.
func dependentPackage(target ir.Target) (*ir.Package, *ir.Dependent) {
	kind := &ir.Enum{Pkg: "a", Name: "Kind", Members: []*ir.EnumMember{{Name: "monster"}, {Name: "game_mode"}, {Name: "class"}}}
	monsters := ir.TypeRef{Kind: types.Ref, Ref: &ir.RefTarget{Coll: types.CollDefines, Pkg: "a", Value: "monsters"}}
	branches := []*ir.Branch{{Name: "monster", Type: monsters}, {Name: "game_mode", Type: ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true}}, {Name: "class", Type: ir.TypeRef{Kind: types.String}}}
	d := &ir.Dependent{
		Pkg: "a", Name: "Param", Disc: &ir.TypeRef{Kind: types.Enum, Named: kind}, ByMember: []int{0, 1, 2}, Branches: branches,
		Go: ir.NameOptions{Name: "Q"}, Cpp: ir.NameOptions{Name: "R"},
	}
	emit := &ir.Emit{Target: target, Mode: ir.ModeData, GoPackage: "a", Namespace: "a"}
	return &ir.Package{Name: "a", Types: []ir.Type{kind, d}, Emits: []*ir.Emit{emit}}, d
}

// TestDependentNames is CODEGEN.md §3.3, §3.5, §5.6: the struct or class, TBranch and its members in arm order, Branch/GetBranch, As<Branch>, As<Branch>Value for a ref into a load.defines table, the storage and Decode<Alias>, an override replacing T; each name is declared where its generator writes it.
func TestDependentNames(t *testing.T) {
	p, d := dependentPackage(ir.TargetGo)
	gp := ir.PlanGoNames(p, p.Emits[0])
	g := gp.Dependent(d)
	want := ir.GoDependent{Type: "Q", Branch: "QBranch", Method: "Branch", BranchStore: "branch", ValueStore: "value", Branches: []ir.GoBranch{
		{Member: "QBranchMonster", As: "AsMonster", AsValue: "AsMonsterValue"}, {Member: "QBranchGameMode", As: "AsGameMode"}, {Member: "QBranchClass", As: "AsClass"},
	}}
	if !slices.Equal(g.Branches, want.Branches) || g.Type != want.Type || g.Branch != want.Branch || g.Method != want.Method || g.BranchStore != want.BranchStore || g.ValueStore != want.ValueStore {
		t.Errorf("go dependent %+v, want %+v", g, want)
	}
	gs := ir.GoScopeNames(gp)
	for _, n := range []string{g.Method, g.BranchStore, g.ValueStore, "AsMonsterValue", "AsGameMode"} {
		if !gs[g.Type][n] {
			t.Errorf("go %s.%s is not declared", g.Type, n)
		}
	}
	if !gs["package"]["QBranchClass"] || len(gp.Problems()) > 0 {
		t.Errorf("go package %v, problems %+v", gs["package"], gp.Problems())
	}
	p, d = dependentPackage(ir.TargetCpp)
	cp := ir.PlanCppNames(p, p.Emits[0])
	c := cp.Dependent(d)
	if c.Class != "R" || c.Branch != "RBranch" || c.GetBranch != "GetBranch" || c.Value != ir.CppVariantMember || c.Decode != "DecodeR" ||
		!slices.Equal(c.Branches, []ir.CppBranch{{Enumerator: "monster", As: "AsMonster", AsValue: "AsMonsterValue"}, {Enumerator: "game_mode", As: "AsGameMode"}, {Enumerator: "class_", As: "AsClass"}}) {
		t.Errorf("cpp dependent %+v", c)
	}
	cs := ir.CppScopeNames(cp)
	if !cs["a"]["R"] || !cs["a"]["RBranch"] || !cs["RBranch"]["class_"] || !cs["R"]["AsMonsterValue"] || !cs["detail"]["DecodeR"] || len(cp.Problems()) > 0 {
		t.Errorf("cpp scopes %v, problems %+v", cs, cp.Problems())
	}
	// Orchestrator call, round 3: the branch enum has no enum helpers, as Go's has no String, Wire or Parse.
	for _, n := range []string{"kRBranchMembers", "RBranchFromWire", "RBranchFromCode"} {
		if cs["a"][n] {
			t.Errorf("cpp plans the branch enum helper %s", n)
		}
	}
	d.Disc, p.Types = &ir.TypeRef{Kind: types.Bool}, []ir.Type{d}
	d.Branches, d.ByMember = d.Branches[1:], []int{0, 1}
	if cs = ir.CppScopeNames(ir.PlanCppNames(p, p.Emits[0])); cs["a"][ir.CppToName] || cs["a"][ir.CppToWire] {
		t.Errorf("a Bool-discriminated dependent type alone plans ToName/ToWire: %v", cs["a"])
	}
}

// TestCppPlanLookups is CODEGEN.md §3.3, §5.2, §5.8–§5.11, §7.6, CONFORMANCE.md §7.2 (log-2026-09-24 "Consumer units (A5)"): each exported lookup is the name the plan declared in its scope.
func TestCppPlanLookups(t *testing.T) {
	p, item := shop(ir.TargetCpp)
	intT := ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true}
	ref := item.Fields[1].Type
	code := &ir.Field{Name: "code", Type: intT, Stable: true}
	item.Fields = append(item.Fields, code)
	item.Methods = append(item.Methods, &ir.ExportFn{Name: "nextBy", Kind: ir.FnLookup, Result: ref})
	p.Types = append(p.Types, &ir.Enum{Pkg: "shop", Name: "Tone", Members: []*ir.EnumMember{{Name: "soft"}}, Codes: &intT})
	pl, ns := ir.PlanCppNames(p, p.Emits[0]), p.Emits[0].Namespace
	sc := ir.CppScopeNames(pl)
	by, v := pl.FindBy(code), p.Values[0]
	getter, resolved := pl.StoredGetter(item.Methods[1])
	vec, h := pl.Vector("Item", item.Methods[0]), pl.EnumHelpers("Tone")
	for _, c := range []struct{ scope, name, want string }{
		{"Items", by.Func, "FindByCode"},
		{"Items", by.Keys, "byCodeKeys_"},
		{"Items", by.Index, "byCode_"},
		{pl.SnapshotName(), pl.SnapshotGetter(v), "GetItems"},
		{ns, pl.SnapshotName(), "ShopSnapshot"},
		{ns, pl.StoreName(), "ShopStore"},
		{"Item", getter, "NextByKey"},
		{"Item", resolved, "NextBy"},
		{"Item", pl.RefMember("next"), "next_ref_"},
		{"conformance", vec.Struct, "ItemTimesVector"},
		{"conformance", vec.Table, "kItemTimes"},
		{"conformance", pl.RunConformanceName(), "RunShopConformance"},
		{ns, h.Members, "kToneMembers"},
		{ns, h.FromWire, "ToneFromWire"},
		{ns, h.FromCode, "ToneFromCode"},
		{"detail", pl.AccessName(), "ShopAccess"},
		{pl.AccessName(), pl.AccessLoader(v), "LoadItems"},
		{pl.AccessName(), pl.AccessSnapshotLoader(), "LoadSnapshot"},
		{pl.AccessName(), ir.CppResolve, "Resolve"},
		{pl.SnapshotName(), ir.CppLoad, "Load"},
		{"detail", ir.CppDecode, "Decode"},
		{ns, ir.CppToName, "ToName"},
		{ns, ir.CppToWire, "ToWire"},
	} {
		if c.name != c.want || !sc[c.scope][c.name] {
			t.Errorf("%s: %q, want %q declared", c.scope, c.name, c.want)
		}
	}
	if vec := pl.Vector("", item.Methods[0]); vec.Struct != "TimesVector" || vec.Table != "kTimes" {
		t.Errorf("package fn vector %+v", vec)
	}
}

// TestGoWalks is CODEGEN.md §5.8 (log-2026-09-24 "Consumer units (A5)"): a class a loader reads slot by slot through a pairs field is not decoded whole, yet its resolved ref is walked; NeedsWalk is Walks for a decoded class.
func TestGoWalks(t *testing.T) {
	p, item := shop(ir.TargetGo)
	stat := &ir.Record{Pkg: "shop", Name: "Stat", Fields: []*ir.Field{{Name: "to", Type: item.Fields[1].Type}}}
	item.Fields = append(item.Fields, &ir.Field{Name: "stats", Pairs: &types.Pairs{}, Type: ir.TypeRef{Kind: types.List, Elem: &ir.TypeRef{Kind: types.Record, Named: stat}}})
	p.Types = append(p.Types, stat)
	pl := ir.PlanGoNames(p, p.Emits[0])
	if pl.Decoded(stat) || pl.NeedsWalk(stat) || !pl.Walks(stat) || !pl.Walks(item) || !pl.NeedsWalk(item) {
		t.Errorf("stat decoded %v walks %v; item walks %v", pl.Decoded(stat), pl.Walks(stat), pl.Walks(item))
	}
}

// TestCppInputHelpers is CODEGEN.md §7.7, EVALUATION.md §11.3: the used helpers only; an enum needs none.
func TestCppInputHelpers(t *testing.T) {
	codes := &ir.Enum{Pkg: "a", Name: "Code", JSONCodes: true}
	plain := &ir.Enum{Pkg: "a", Name: "Tone"}
	typesOf := []ir.TypeRef{
		{Kind: types.Enum, Named: codes},
		{Kind: types.Float, Bits: 32},
		{Kind: types.Bool},
		{Kind: types.Duration},
		{Kind: types.Enum, Named: plain},
	}
	p := helperPackage(typesOf, codes, plain)
	pl := ir.PlanCppNames(p, p.Emits[0])
	want := []string{
		"EnvText", "IsDecDigit", "AllDigits", "ParseFloatLiteral", "ParseBoolLiteral",
		"DurationDigits", "ParseDurationLiteral",
	}
	if got := pl.Inputs().Helpers; !slices.Equal(got, want) {
		t.Errorf("helpers %v, want %v", got, want)
	}
	ns := ir.CppScopeNames(pl)["a"]
	for _, h := range want {
		if !ns[h] {
			t.Errorf("helper %s is not a name of the namespace", h)
		}
	}
	parsers := []string{"", "ParseFloatLiteral", "ParseBoolLiteral", "ParseDurationLiteral", ""}
	for i, ty := range typesOf {
		if got := pl.InputParser(ty); got != parsers[i] {
			t.Errorf("parser of %v: %q, want %q", ty.Kind, got, parsers[i])
		}
	}
	p = helperPackage(typesOf[:1], codes)
	p.Consts = append(p.Consts, &ir.Const{Name: "AllDigits"})
	pl = ir.PlanCppNames(p, p.Emits[0])
	if got := pl.Inputs().Helpers; !slices.Equal(got, []string{"EnvText"}) || len(pl.Problems()) > 0 {
		t.Errorf("an @json(codes) enum input alone: helpers %v, problems %+v", got, pl.Problems())
	}
	p, _ = inputPackage(ir.TargetCpp)
	if got := ir.PlanCppNames(p, p.Emits[0]).Inputs().Helpers; !slices.Equal(got, []string{"EnvText", "ParseStringLiteral"}) {
		t.Errorf("String-only helpers %v", got)
	}
}

// helperPackage is a cpp data emit of package a whose record Gen has one optional input per type.
func helperPackage(of []ir.TypeRef, enums ...*ir.Enum) *ir.Package {
	rec := &ir.Record{Pkg: "a", Name: "Gen"}
	for i, ty := range of {
		rec.Fields = append(rec.Fields, &ir.Field{
			Name: fmt.Sprintf("f%d", i), Type: ty, Optional: true,
			Input: &types.Input{Env: "E"},
		})
	}
	p := &ir.Package{Name: "a", Types: []ir.Type{rec}, Emits: []*ir.Emit{{Target: ir.TargetCpp, Mode: ir.ModeData, Namespace: "a"}}}
	for _, e := range enums {
		p.Types = append(p.Types, e)
	}
	return p
}
