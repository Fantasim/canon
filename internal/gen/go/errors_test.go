package gogen_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

var (
	boolT = ir.TypeRef{Kind: types.Bool}
	intT  = ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true}
)

func goEmit() *ir.Emit {
	return &ir.Emit{Target: ir.TargetGo, Out: "out/go", Dir: "p/out/go", GoImport: "example.com/p", Mode: ir.ModeBaked, GoPackage: "p"}
}

func pkg(types ...ir.Type) *ir.Package {
	return &ir.Package{Name: "p", Dir: "p", Types: types, Emits: []*ir.Emit{goEmit()}}
}

func record(name string, fields ...string) *ir.Record {
	r := &ir.Record{Pkg: "p", Name: name}
	for _, f := range fields {
		r.Fields = append(r.Fields, &ir.Field{Name: f, Type: boolT})
	}
	return r
}

func enum(name string, members ...string) *ir.Enum {
	e := &ir.Enum{Pkg: "p", Name: name}
	for i, m := range members {
		e.Members = append(e.Members, &ir.EnumMember{Name: m, Wire: m, Index: i})
	}
	return e
}

// generateErr runs the generator on p's emit, changed by edit when given.
func generateErr(p *ir.Package, edit func(*ir.Emit)) error {
	e := p.Emits[0]
	if edit != nil {
		edit(e)
	}
	_, err := gogen.Generate(p, e)
	return err
}

// CODEGEN.md §3.5: two generated names equal in one Go scope are a plan Problem, ErrMalformed.
func TestNameCollisions(t *testing.T) {
	strong := record("Gem", "strong")
	strong.Methods = []*ir.ExportFn{{Name: "strong", Kind: ir.FnPrecomputed, Result: boolT}}
	potion := record("Potion", "id")
	list := ir.TypeRef{Kind: types.List, Elem: &ir.TypeRef{Kind: types.Record, Named: potion}, KeyedBy: &ir.KeyField{Name: "id"}}
	withValue := pkg(potion)
	withValue.Values = []*ir.Value{{Name: "potion", Type: list, V: &value.List{}}}
	other := &ir.Enum{Pkg: "q", Name: "Q", Members: []*ir.EnumMember{{Name: "x", Wire: "x"}}}
	importsIter := pkg(enum("E", "x"), &ir.Record{Pkg: "p", Name: "R", Fields: []*ir.Field{{Name: "q", Type: ir.TypeRef{Kind: types.Enum, Named: other}}}})
	importsIter.Imports = []*ir.PackageRef{{Name: "q", Emits: []*ir.Emit{{Target: ir.TargetGo, GoImport: "example.com/iter", GoPackage: "iter"}}}}
	e := enum("E", "x")
	escaped := pkg(e)
	escaped.Imports = []*ir.PackageRef{
		{Name: "q", Emits: []*ir.Emit{{Target: ir.TargetGo, GoImport: "example.com/q", GoPackage: "q"}}},
		{Name: "q_", Emits: []*ir.Emit{{Target: ir.TargetGo, GoImport: "example.com/q_", GoPackage: "q_"}}},
	}
	escaped.Fns = []*ir.ExportFn{{
		Name: "f", Kind: ir.FnLookup, Result: boolT, Params: []*ir.Param{{Name: "q", Type: ir.TypeRef{Kind: types.Enum, Named: e}}},
		Table: &ir.LookupTable{Domains: [][]value.Value{{&value.Member{}}}, Cells: []value.Value{&value.Bool{}}},
	}}
	cases := map[string]*ir.Package{
		"a parameter q, imports q and q_":   escaped,
		"enum members series_1 and series1": pkg(enum("Tone", "series_1", "series1")),
		"fields fooBar and foo_bar":         pkg(record("R", "fooBar", "foo_bar")),
		"field and export fn strong":        pkg(strong),
		"container Potion and record":       withValue,
		"two imports named iter":            importsIter,
		"two types of one name":             pkg(record("Same"), enum("Same", "x")),
		"a const named like its enum":       {Name: "p", Dir: "p", Types: []ir.Type{enum("E", "x")}, Consts: []*ir.Const{{Name: "e", Type: boolT, V: &value.Bool{}}}, Emits: []*ir.Emit{goEmit()}},
	}
	cases["a const named like its enum"].Consts[0].Go.Name = "E"
	overridden := enum("Tone", "a", "b")
	overridden.Members[0].Go.Name = "ToneB"
	cases["a member override ToneB next to member b"] = pkg(overridden)
	for _, name := range []string{
		"a parameter q, imports q and q_", "enum members series_1 and series1", "fields fooBar and foo_bar",
		"field and export fn strong", "container Potion and record", "two imports named iter",
		"two types of one name", "a const named like its enum", "a member override ToneB next to member b",
	} {
		if err := generateErr(cases[name], nil); !errors.Is(err, gogen.ErrMalformed) {
			t.Errorf("%s: got %v, want ErrMalformed", name, err)
		}
	}
}

// CODEGEN.md §3.4, go.md §3: a Canon name with no Go spelling is a plan Problem, ErrMalformed.
func TestNameNotIdentifier(t *testing.T) {
	if err := generateErr(pkg(record("R", "_")), nil); !errors.Is(err, gogen.ErrMalformed) {
		t.Errorf("got %v, want ErrMalformed", err)
	}
}

// CODEGEN.md §2.1: this generator emits go only, in baked and data mode for now.
func TestRefusedEmits(t *testing.T) {
	cases := []struct {
		name string
		edit func(*ir.Emit)
		want error
	}{
		{"ts emit", func(e *ir.Emit) { e.Target = ir.TargetTS }, gogen.ErrTarget},
		{"types mode", func(e *ir.Emit) { e.Mode = ir.ModeTypes }, gogen.ErrUnsupported},
		{"embedded mode", func(e *ir.Emit) { e.Mode = ir.ModeEmbedded }, gogen.ErrUnsupported},
		{"no package", func(e *ir.Emit) { e.GoPackage = "" }, gogen.ErrMalformed},
		{"no import path", func(e *ir.Emit) { e.GoImport = "" }, gogen.ErrMalformed},
	}
	for _, c := range cases {
		if err := generateErr(pkg(), c.edit); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

// Constructs baked Go does not emit yet are refused, never skipped.
func TestUnsupportedConstructs(t *testing.T) {
	input := record("Gen", "apiKey")
	input.Fields[0].Input = &types.Input{Env: "KEY"}
	negZero := pkg()
	negZero.Consts = []*ir.Const{{Name: "Z", Type: ir.TypeRef{Kind: types.Float, Bits: 64}, V: &value.Float{V: negativeZero()}}}
	cases := map[string]*ir.Package{
		"dependent type": pkg(&ir.Dependent{Pkg: "p", Name: "Param"}),
		"input field":    pkg(input),
		"-0.0 constant":  negZero,
	}
	for _, name := range []string{"dependent type", "input field", "-0.0 constant"} {
		if err := generateErr(cases[name], nil); !errors.Is(err, gogen.ErrUnsupported) {
			t.Errorf("%s: got %v, want ErrUnsupported", name, err)
		}
	}
}

// An IR that contradicts itself is a stage E defect, reported as such.
func TestMalformedIR(t *testing.T) {
	status := record("Status", "open")
	table := ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: status}}
	ids := pkg(status)
	ids.Values = []*ir.Value{{Name: "statuses", Type: table, IDs: []string{"a", "b"}, V: &value.Table{}}}
	wrongValue := pkg()
	wrongValue.Consts = []*ir.Const{{Name: "N", Type: intT, V: &value.Bool{}}}
	width := pkg()
	width.Consts = []*ir.Const{{Name: "N", Type: ir.TypeRef{Kind: types.Int, Bits: 12}, V: &value.Int{}}}
	noElem := pkg()
	noElem.Consts = []*ir.Const{{Name: "L", Type: ir.TypeRef{Kind: types.List}, V: &value.List{}}}
	cases := map[string]*ir.Package{"ids without entries": ids, "a Bool for an Int": wrongValue, "a 12-bit Int": width, "a list without its element": noElem}
	for _, name := range []string{"ids without entries", "a Bool for an Int", "a 12-bit Int", "a list without its element"} {
		if err := generateErr(cases[name], nil); !errors.Is(err, gogen.ErrMalformed) {
			t.Errorf("%s: got %v, want ErrMalformed", name, err)
		}
	}
}

func negativeZero() float64 {
	z := 0.0
	return -z
}

// CODEGEN.md §5.3, §5.10: an empty table's id enum and a lookup over it still format.
func TestEmptyDomain(t *testing.T) {
	status := record("Status", "open")
	table := ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: status}}
	p := pkg(status)
	p.Values = []*ir.Value{{Name: "statuses", Type: table, V: &value.Table{}}}
	target := &ir.RefTarget{Coll: types.CollLet, Pkg: "p", Value: "statuses", Elem: status}
	p.Fns = []*ir.ExportFn{{
		Name: "f", Kind: ir.FnLookup, Result: boolT,
		Params: []*ir.Param{{Name: "s", Type: ir.TypeRef{Kind: types.Ref, Ref: target}}, {Name: "b", Type: boolT}},
		Table:  &ir.LookupTable{Domains: [][]value.Value{nil, {&value.Bool{}, &value.Bool{V: true}}}},
	}}
	files, err := gogen.Generate(p, p.Emits[0])
	if err != nil || !strings.Contains(string(files[1].Content), "var fTable = [0][2]bool{}") {
		t.Errorf("err = %v", err)
	}
}

// CODEGEN.md §5.8, decision 180: a ref into a load.defines table is refused.
func TestDefineRefsRefused(t *testing.T) {
	target := &ir.RefTarget{Coll: types.CollDefines, Pkg: "p", Value: "itemKinds"}
	str := ir.TypeRef{Kind: types.String}
	field := record("Item")
	field.Fields = []*ir.Field{{Name: "kind", Type: ir.TypeRef{Kind: types.Ref, Ref: target, Key: &str}}}
	table := pkg()
	table.Defines = []*ir.DefineTable{{Pkg: "p", Value: "itemKinds", Names: []string{"IK1_WEAPON"}, Values: []int64{1}}}
	for _, p := range []*ir.Package{pkg(field), table} {
		err := generateErr(p, nil)
		var d *gogen.DetailError
		if !errors.Is(err, gogen.ErrUnsupported) || !errors.As(err, &d) || d.Subject != "p.itemKinds" {
			t.Errorf("got %v, want ErrUnsupported naming the define table p.itemKinds", err)
		}
	}
}

// CODEGEN.md §1.3, §3.5: a @go(name:) override names public API, so it must be exported, or ErrMalformed.
func TestUnexportedOverride(t *testing.T) {
	field := record("R", "heal")
	field.Fields[0].Go.Name = "heal"
	member := enum("E", "x")
	member.Members[0].Go.Name = "eX"
	fn := pkg()
	fn.Fns = []*ir.ExportFn{{Name: "f", Kind: ir.FnPrecomputed, Result: boolT, Value: &value.Bool{}, Go: ir.NameOptions{Name: "f"}}}
	for _, p := range []*ir.Package{pkg(field), pkg(member), fn} {
		err := generateErr(p, nil)
		var d *gogen.DetailError
		if !errors.Is(err, gogen.ErrMalformed) || !errors.As(err, &d) {
			t.Errorf("got %v, want ErrMalformed for an unexported override", err)
		}
	}
}

// CODEGEN.md §3.5: a getter override equal to a generated method of the struct collides.
func TestOverrideCollidesWithMethod(t *testing.T) {
	status := record("Status", "open")
	status.Fields[0].Go.Name = "Retired"
	table := ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: status}}
	p := pkg(status)
	p.Values = []*ir.Value{{Name: "statuses", Type: table, V: &value.Table{}}}
	if err := generateErr(p, nil); !errors.Is(err, gogen.ErrMalformed) {
		t.Errorf("got %v, want ErrMalformed", err)
	}
}

// LOCK.md E6003, CODEGEN.md §5.9: an optional @stable field is refused in every table, emitted or not.
func TestStableOptionalFieldRefused(t *testing.T) {
	for _, values := range [][]string{nil, {"on"}} {
		status := record("Status", "code")
		status.Fields[0].Stable = true
		status.Fields[0].Optional = true
		table := ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: status}}
		p := pkg(status)
		p.Values = []*ir.Value{{Name: "statuses", Type: table, IDs: []string{"a"},
			V: &value.Table{Entries: []*value.Record{{Ident: &value.Identity{Key: value.Key{S: "a"}}}}}},
			{Name: "on", Type: boolT, V: &value.Bool{}}}
		err := generateErr(p, func(e *ir.Emit) { e.Values = values })
		var d *gogen.DetailError
		if !errors.Is(err, gogen.ErrMalformed) || !errors.As(err, &d) || d.Subject != "statuses.code" {
			t.Errorf("values %v: got %v, want ErrMalformed naming statuses.code", values, err)
		}
	}
}

// CODEGEN.md §5.3, §5.9: a table's rows are indexed by its id enum, numbered in IDs order.
func TestTableIDsOutOfOrder(t *testing.T) {
	status := record("Status")
	table := ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: status}}
	entry := func(key string) *value.Record {
		return &value.Record{Ident: &value.Identity{Key: value.Key{S: key}}}
	}
	p := pkg(status)
	p.Values = []*ir.Value{{Name: "statuses", Type: table, IDs: []string{"b", "a"}, V: &value.Table{Entries: []*value.Record{entry("a"), entry("b")}}}}
	err := generateErr(p, nil)
	var d *gogen.DetailError
	if !errors.Is(err, gogen.ErrMalformed) || !errors.As(err, &d) || d.Subject != "statuses" || d.Index != 0 {
		t.Errorf("got %v, want ErrMalformed for entry 0 of statuses", err)
	}
}

// CODEGEN.md §4.4, §5.1, §5.5, decision 180: no Go form; the refusal names the refused item.
func TestTypesWithoutGo(t *testing.T) {
	shape := &ir.Variant{Pkg: "p", Name: "Shape", Cases: []*ir.Case{{Name: "none", Wire: "none"}}}
	caseField := record("R")
	caseField.Fields = []*ir.Field{{Name: "s", Type: ir.TypeRef{Kind: types.Case, Named: shape, Case: shape.Cases[0]}}}
	never := record("N")
	never.Fields = []*ir.Field{{Name: "n", Type: ir.TypeRef{Kind: types.Never}}}
	constant := pkg(never)
	constant.Consts = []*ir.Const{{Name: "origin", Type: ir.TypeRef{Kind: types.Record, Named: never}}}
	cases := []struct {
		name string
		p    *ir.Package
		want string
	}{
		{"a field of a case without fields", pkg(shape, caseField), "p.Shape.none"},
		{"a Never field", pkg(never), "p.N.n"},
		{"a record constant", constant, "origin"},
	}
	for _, c := range cases {
		err := generateErr(c.p, nil)
		var d *gogen.DetailError
		if !errors.Is(err, gogen.ErrUnsupported) || !errors.As(err, &d) || d.Subject != c.want {
			t.Errorf("%s: got %v, want ErrUnsupported naming %s", c.name, err, c.want)
		}
	}
}

// CODEGEN.md §3.3, §3.5, decision 194: an enum member's override is its whole constant, so `a @go(name: "B"), b` generates B next to ToneB, as stage E's name plan accepts it.
func TestEnumOverrideGenerates(t *testing.T) {
	tone := enum("Tone", "a", "b")
	tone.Members[0].Go.Name = "B"
	files, err := gogen.Generate(pkg(tone), goEmit())
	if err != nil {
		t.Fatal(err)
	}
	src := string(files[len(files)-1].Content)
	if !regexp.MustCompile(`\tB +Tone = 0\n`).MatchString(src) || !strings.Contains(src, "\tToneB Tone = 1\n") {
		t.Errorf("want the constants B and ToneB:\n%s", src)
	}
}
