package ir

import (
	"math"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// TestWords is CODEGEN.md §3.1: the word split, with the document's examples (decision 120).
func TestWords(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"II_WEA_AXE_ANGEL", []string{"II", "WEA", "AXE", "ANGEL"}},
		{"gm_junior", []string{"gm", "junior"}},
		{"Stage_1", []string{"Stage", "1"}},
		{"none_", []string{"none"}},
		{"stage1Rate", []string{"stage", "1", "Rate"}},
		{"minRole", []string{"min", "Role"}},
		{"series1", []string{"series", "1"}},
		{"HTTPServer", []string{"HTTP", "Server"}},
		{"__a__b", []string{"a", "b"}},
		{"_", nil},
	}
	for _, c := range cases {
		if got := Words(c.in); !slices.Equal(got, c.want) {
			t.Errorf("Words(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestGoCamel is CODEGEN.md §3.2: Go camel case with the closed initialism list (II is none of them).
func TestGoCamel(t *testing.T) {
	cases := []struct{ in, upper, lower string }{
		{"id", "ID", "id"},
		{"minRole", "MinRole", "minRole"},
		{"apiKey", "APIKey", "apiKey"},
		{"series_1", "Series1", "series1"},
		{"II_WEA_AXE_ANGEL", "IiWeaAxeAngel", "iiWeaAxeAngel"},
		{"none_", "None", "none"},
		{"url_ts_db", "URLTSDB", "urlTSDB"},
		{"hpMax", "HPMax", "hpMax"},
		{"JSONPath", "JSONPath", "jsonPath"},
		{"_", "", ""},
	}
	for _, c := range cases {
		if got := goUpperCamel(c.in); got != c.upper {
			t.Errorf("goUpperCamel(%q) = %q, want %q", c.in, got, c.upper)
		}
		if got := goLowerCamel(c.in); got != c.lower {
			t.Errorf("goLowerCamel(%q) = %q, want %q", c.in, got, c.lower)
		}
	}
}

// TestGoEscapeLower is CODEGEN.md §3.4: a reserved name in a lower-case position gets a `_` suffix.
func TestGoEscapeLower(t *testing.T) {
	cases := []struct{ in, want string }{
		{"default", "default_"},
		{"type", "type_"},
		{"len", "len_"},
		{"string", "string_"},
		{"rt", "rt_"},
		{"iter", "iter_"},
		{"self", "self_"},
		{"embed", "embed_"},
		{"json", "json_"},
		{"minRole", "minRole"},
		{"route", "route"},
	}
	for _, c := range cases {
		if got := goEscapeLower(c.in); got != c.want {
			t.Errorf("goEscapeLower(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestGoStorageName is CODEGEN.md §3.4: a storage name is lowerCamel, escaped when reserved.
func TestGoStorageName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"default", "default_"},
		{"type", "type_"},
		{"len", "len_"},
		{"string", "string_"},
		{"rt", "rt_"},
		{"iter", "iter_"},
		{"self", "self_"},
		{"embed", "embed_"},
		{"minRole", "minRole"},
		{"Default", "default_"},
		{"route", "route"},
	}
	for _, c := range cases {
		if got := goStorageName(c.in); got != c.want {
			t.Errorf("goStorageName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestGoPlanNames is CODEGEN.md §3.3 and decision 193: a type's first capital, an override as the whole name, the id type and member <Element>ID, <Element>ID<Key>.
func TestGoPlanNames(t *testing.T) {
	status := &Record{Pkg: "p", Name: "status"}
	tone := &Enum{Pkg: "p", Name: "Tone", Members: []*EnumMember{{Name: "a", Go: NameOptions{Name: "B"}}, {Name: "b"}}}
	pl := PlanGoNames(&Package{Name: "a.teamboard"}, &Emit{Target: TargetGo})
	for _, c := range []struct{ got, want string }{
		{pl.TypeName(status), "Status"},
		{pl.IDTypeName(status), "StatusID"},
		{pl.IDMemberName(status, "wont_do"), "StatusIDWontDo"},
		{pl.MemberName(tone, tone.Members[0]), "B"},
		{pl.MemberName(tone, tone.Members[1]), "ToneB"},
		{pl.Data().Type, "teamboardData"},
		{pl.Data().Build, "buildTeamboard"},
		{goExported(NameOptions{Name: "GetID"}, "id"), "GetID"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

// TestGoPlanEnumOverride is CODEGEN.md §3.3, §3.5: an enum member's override is its whole constant, so `a @go(name: "B"), b` declares B and ToneB (clean) while an override ToneB on a collides with b's own ToneB (decision 194).
func TestGoPlanEnumOverride(t *testing.T) {
	for _, c := range []struct {
		override string
		want     int
	}{{"B", 0}, {"ToneB", 1}} {
		tone := &Enum{Pkg: "p", Name: "Tone", Members: []*EnumMember{{Name: "a", Go: NameOptions{Name: c.override}}, {Name: "b"}}}
		pl := PlanGoNames(&Package{Name: "p", Types: []Type{tone}}, &Emit{Target: TargetGo})
		if got := len(pl.Problems()); got != c.want {
			t.Errorf("override %q: %d problems, want %d: %+v", c.override, got, c.want, pl.Problems())
		}
	}
}

// TestGoPlanUnselectedContainer is CODEGEN.md §5.9, decision 194: a table the emit leaves out of `values` gets no container, so its name is free; its id enum stays (decision 124).
func TestGoPlanUnselectedContainer(t *testing.T) {
	potion := &Record{Pkg: "p", Name: "Potion"}
	table := TypeRef{Kind: types.Table, Elem: &TypeRef{Kind: types.Record, Named: potion}}
	p := &Package{Name: "p", Types: []Type{potion}, Values: []*Value{
		{Name: "potion", Type: table, V: &value.Table{}},
		{Name: "on", Type: TypeRef{Kind: types.Bool}, V: &value.Bool{}},
	}}
	if got := PlanGoNames(p, &Emit{Target: TargetGo, Values: []string{"on"}}).Problems(); len(got) != 0 {
		t.Errorf("an unselected table must not claim its container name: %+v", got)
	}
	if got := PlanGoNames(p, &Emit{Target: TargetGo}).Problems(); len(got) != 1 || got[0].Name != "Potion" {
		t.Errorf("a selected table's container Potion collides with the record Potion: %+v", got)
	}
}

// TestGoPlanOverrideExported is decision 182: a @go(name:) must be an exported identifier.
func TestGoPlanOverrideExported(t *testing.T) {
	for _, c := range []struct {
		name string
		ok   bool
	}{{"Heal", true}, {"heal", false}, {"1Bad", false}, {"_X", false}, {"Ünïcode", true}} {
		if got := goValidOverride(c.name); got != c.ok {
			t.Errorf("goValidOverride(%q) = %v, want %v", c.name, got, c.ok)
		}
	}
}

// TestAccessWords is CPP-01: stage E's Access values index syntax's @cpp(access:) symbols.
func TestAccessWords(t *testing.T) {
	want := map[Access]string{AccessNone: "", AccessFields: "fields", AccessBoth: "both", AccessGetters: "getters"}
	for a, w := range want { //canon:unordered each access checked alone
		if accessWords[a] != w {
			t.Errorf("accessWords[%d] = %q, want %q", a, accessWords[a], w)
		}
	}
	if len(accessWords) != len(syntax.AccessModes())+1 {
		t.Errorf("accessWords has %d words for %d modes", len(accessWords), len(syntax.AccessModes()))
	}
}

// TestReservedSets is CODEGEN.md §3.4: the escaped words are check's keyword sets and the lists of §3.4.
func TestReservedSets(t *testing.T) {
	for _, w := range []string{"func", "type", "len", "rt", "json", "self"} {
		if !goReserved(w) {
			t.Errorf("%q must be reserved in a Go lower-case position", w)
		}
	}
	for _, w := range []string{"class", "xor_eq", "detail", "nlohmann"} {
		if !CppReserved(w) {
			t.Errorf("%q must be reserved in a C++ verbatim position", w)
		}
	}
	if goReserved("route") {
		t.Errorf("route is not reserved")
	}
}

// TestGoEffectiveStore is decision 203: a field's storage derives from its @go(name:) override when there is one, lowerCamel'd and escaped, and decision 121's interior-`_` forms follow it.
func TestGoEffectiveStore(t *testing.T) {
	intT := TypeRef{Kind: types.Int}
	pl := PlanGoNames(&Package{Name: "p"}, &Emit{Target: TargetGo})
	for _, c := range []struct {
		f                     *Field
		getter, store, okName string
	}{
		{&Field{Name: "hp", Type: intT, Go: NameOptions{Name: "HitPoints"}}, "HitPoints", "hitPoints", "hitPoints_ok"},
		{&Field{Name: "hp", Type: intT}, "HP", "hp", "hp_ok"},
		{&Field{Name: "kind", Type: intT, Go: NameOptions{Name: "Type"}}, "Type", "type_", "type__ok"},
	} {
		c.f.Optional = true
		s := pl.Slot(c.f)
		if s.Getter != c.getter || s.Store != c.store || s.OKStore != c.okName {
			t.Errorf("%s: got %s %s %s, want %s %s %s", c.f.Name, s.Getter, s.Store, s.OKStore, c.getter, c.store, c.okName)
		}
	}
}

// TestGoPlanOneProblemPerCause is decision 203: two items meeting in several names, in one scope or in two, give one problem, named after the first name they meet in, the getter.
func TestGoPlanOneProblemPerCause(t *testing.T) {
	intT, boolT := TypeRef{Kind: types.Int}, TypeRef{Kind: types.Bool}
	rule := &Record{Pkg: "p", Name: "Rule", Fields: []*Field{{Name: "fooBar", Type: intT}, {Name: "foo_bar", Type: intT}}}
	fields := &Package{Name: "p", Types: []Type{rule}}
	values := &Package{Name: "p", Values: []*Value{
		{Name: "fooBar", Type: boolT, V: &value.Bool{}}, {Name: "foo_bar", Type: boolT, V: &value.Bool{}},
	}}
	for _, c := range []struct {
		p    *Package
		name string
	}{{fields, "FooBar"}, {values, "GetFooBar"}} {
		got := PlanGoNames(c.p, &Emit{Target: TargetGo}).Problems()
		if len(got) != 1 || got[0].Kind != GoCollision || got[0].Name != c.name {
			t.Errorf("want one collision at %s: %+v", c.name, got)
		}
	}
}

// TestGoPlanMathOnlyForNegZero is decision 202 (and 181): baked Go imports math only to write a -0.0 literal, so the plan declares math only then, whatever Float types the package has; a scalar -0.0 constant is no literal gen/go writes (it refuses it), a list constant's element is.
func TestGoPlanMathOnlyForNegZero(t *testing.T) {
	floatT := TypeRef{Kind: types.Float, Bits: 64}
	listT := TypeRef{Kind: types.List, Elem: &floatT}
	negZero := math.Copysign(0, -1)
	for _, c := range []struct {
		v    float64
		list bool
		want bool
	}{{1.5, true, false}, {0, true, false}, {negZero, true, true}, {negZero, false, false}} {
		var k *Const
		if f := (&value.Float{V: c.v, T: types.FloatType}); c.list {
			k = &Const{Name: "w", Type: listT, V: &value.List{Elems: []value.Value{f}}}
		} else {
			k = &Const{Name: "w", Type: floatT, V: f}
		}
		names := GoScopeNames(PlanGoNames(&Package{Name: "p", Consts: []*Const{k}}, &Emit{Target: TargetGo}))[goScopePackage]
		if got := names[goMath]; got != c.want {
			t.Errorf("a constant %v (list %v): math declared %v, want %v", c.v, c.list, got, c.want)
		}
	}
}

// TestGoPlanIndexLocals is decision 122: a table read indexes a Bool or a @codes enum through a local <param>_i the plan names, and any other parameter through itself.
func TestGoPlanIndexLocals(t *testing.T) {
	codes := &Enum{Pkg: "p", Name: "Rank", Codes: &TypeRef{Kind: types.Int}}
	plain := &Enum{Pkg: "p", Name: "Tone"}
	fn := &ExportFn{Name: "f", Kind: FnLookup, Result: TypeRef{Kind: types.Bool}, Params: []*Param{
		{Name: "on", Type: TypeRef{Kind: types.Bool}},
		{Name: "rank", Type: TypeRef{Kind: types.Enum, Named: codes}},
		{Name: "tone", Type: TypeRef{Kind: types.Enum, Named: plain}},
	}}
	got := PlanGoNames(&Package{Name: "p", Fns: []*ExportFn{fn}}, &Emit{Target: TargetGo}).Finite(fn).Indexes
	if want := []string{"on_i", "rank_i", ""}; !slices.Equal(got, want) {
		t.Errorf("index locals %q, want %q", got, want)
	}
}

// TestGoPlanNegZeroInStoredResult is decision 202: a -0.0 that gen/go writes as a record's stored method result declares math too.
func TestGoPlanNegZeroInStoredResult(t *testing.T) {
	floatT := TypeRef{Kind: types.Float, Bits: 64}
	gem := &Record{Pkg: "p", Name: "Gem"}
	recv := &value.Record{}
	gem.Methods = []*ExportFn{{
		Name: "weight", Kind: FnPrecomputed, Result: floatT,
		Instances: []*Instance{{Recv: recv, Result: &value.Float{V: math.Copysign(0, -1), T: types.FloatType}}},
	}}
	p := &Package{Name: "p", Types: []Type{gem}, Values: []*Value{
		{Name: "gem", Type: TypeRef{Kind: types.Record, Named: gem}, V: recv},
	}}
	if names := GoScopeNames(PlanGoNames(p, &Emit{Target: TargetGo}))[goScopePackage]; !names[goMath] {
		t.Errorf("a stored -0.0 result must declare math: %v", names)
	}
}
