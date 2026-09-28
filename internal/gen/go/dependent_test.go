package gogen_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// dependentField adds a valid dependent-type field p to Thing, with its own enum k (CODEGEN.md §5.6).
func dependentField(p *ir.Package) (*ir.Dependent, *ir.Field) {
	k := &ir.Enum{Pkg: "demo", Name: "K", Members: []*ir.EnumMember{
		{Name: "one", Wire: "one"}, {Name: "two", Wire: "two", Index: 1},
	}}
	d := &ir.Dependent{
		Pkg: "demo", Name: "P", Params: 1, DiscParam: 0, Disc: &ir.TypeRef{Kind: types.Enum, Named: k},
		Branches: []*ir.Branch{{Name: "one", Members: []int{0}, Type: intT}, {Name: "two", Members: []int{1}, Type: strT}},
	}
	p.Types = append(p.Types, k, d)
	addField(p, wired("k", "k", "", ir.TypeRef{Kind: types.Enum, Named: k}))
	f := &ir.Field{Name: "p", WirePath: []string{"p"}, Type: ir.TypeRef{
		Kind: types.TypeApp, Named: d, Args: []*ir.Source{{From: types.ArgField, WirePath: []string{"k"}}},
	}}
	addField(p, f)
	return d, f
}

// optional marks rec's field name optional.
func optionalField(rec *ir.Record, name string) {
	for _, f := range rec.Fields {
		if f.Name == name {
			f.Optional = true
		}
	}
}

// dependentPkg is a table Thing whose p field is a dependent type P(k), the discriminant k an earlier field of the same record (CODEGEN.md §5.6).
func dependentPkg() *ir.Package {
	k := &ir.Enum{Pkg: "dep", Name: "K", Members: []*ir.EnumMember{
		{Name: "one", Wire: "one", Index: 0}, {Name: "two", Wire: "two", Index: 1},
		{Name: "three", Wire: "three", Index: 2}, // no branch covers it: TestDependentUnknownCase
	}}
	p := &ir.Dependent{
		Pkg: "dep", Name: "P", Params: 1, DiscParam: 0,
		Disc: &ir.TypeRef{Kind: types.Enum, Named: k},
		Branches: []*ir.Branch{
			{Name: "one", Members: []int{0}, Type: intT},
			{Name: "two", Members: []int{1}, Type: strT},
		},
	}
	thing := &ir.Record{Pkg: "dep", Name: "Thing", Fields: []*ir.Field{
		wired("k", "k", "", ir.TypeRef{Kind: types.Enum, Named: k}),
		{
			Name: "p", WirePath: []string{"p"},
			Type: ir.TypeRef{Kind: types.TypeApp, Named: p, Args: []*ir.Source{{From: types.ArgField, WirePath: []string{"k"}}}},
		},
	}}
	elem := ir.TypeRef{Kind: types.Record, Named: thing}
	things := &ir.Value{Name: "things", Schema: "dep.Thing@00000001", Type: ir.TypeRef{Kind: types.Table, Elem: &elem}}
	return &ir.Package{
		Name: "dep", Dir: "dep", Types: []ir.Type{k, p, thing}, Values: []*ir.Value{things},
		Emits: []*ir.Emit{goData("dep", "dep")},
	}
}

// CODEGEN.md §5.6, §2.7: the branch enum between the kind enums and the id enums, the struct and its accessors.
func TestDependentDeclarations(t *testing.T) {
	files := generateData(t, dependentPkg())
	src := string(files["dep/out/go/dep.gen.go"])
	for _, want := range []string{
		"type PBranch uint8", "PBranchOne PBranch = 0", "PBranchTwo PBranch = 1",
		"type P struct", "branch PBranch", "value  any",
		"func (self *P) Branch() PBranch { return self.branch }",
		"func (self *P) AsOne() (int64, bool)", "func (self *P) AsTwo() (string, bool)",
		"func decodeP(", "func decodeThing(",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("want %q in generated source:\n%s", want, src)
		}
	}
	if i, j := strings.Index(src, "type K "), strings.Index(src, "type PBranch"); i < 0 || j < i {
		t.Errorf("the branch enum PBranch must follow the plain enums section (CODEGEN.md §2.7)")
	}
	if i, j := strings.Index(src, "type PBranch"), strings.Index(src, "type ThingID"); i < 0 || j < i {
		t.Errorf("the branch enum PBranch must precede the id enums (CODEGEN.md §2.7)")
	}
	for _, unwanted := range []string{"func (self PBranch) String()", "func (self PBranch) Wire()", "func ParsePBranch("} {
		if strings.Contains(src, unwanted) {
			t.Errorf("the branch enum has no String, Wire or Parse<…> (CODEGEN.md §5.6): found %q", unwanted)
		}
	}
}

// CODEGEN.md §5.6, §6.1: decode<T> picks the branch the already-decoded discriminant selects.
func TestDependentDataCompiles(t *testing.T) {
	files := generateData(t, dependentPkg())
	data := map[string][]byte{
		"good/things.json": []byte(`{
  "$schema": "dep.Thing@00000001",
  "rows": [
    {"$id": "a", "k": "one", "p": 42},
    {"$id": "b", "k": "two", "p": "hi"}
  ]
}
`),
		"bad/things.json": []byte(`{
  "$schema": "dep.Thing@00000001",
  "rows": [
    {"$id": "c", "k": "three", "p": 0}
  ]
}
`),
	}
	runData(t, files, "dep/out/go", "testdata/smoke/dependent_test.go", data)
}

// dependentRefusal is one dependent-type construct stage E refuses (E8019 DependentType, E8012, E3806), which the generator then finds malformed with its own text naming subject.
type dependentRefusal struct {
	edit    func(*ir.Package)
	cause   error
	subject string
}

// dependentRefusals: one row per dependent-type construct stage E refuses first (CODEGEN.md §5.6; decision 37).
func dependentRefusals() map[string]dependentRefusal {
	return map[string]dependentRefusal{
		"a discriminant of another type than the field's": {func(p *ir.Package) {
			d, _ := dependentField(p)
			d.Disc = &boolT
		}, gogen.ErrDependentDisc, "demo.Thing"},
		"a match path through a field that holds no record": {func(p *ir.Package) {
			d, _ := dependentField(p)
			d.DiscPath = []string{"x"}
		}, gogen.ErrDependentDisc, "demo.Thing"},
		"an argument from a record's own parameter (CODEGEN.md §5.7)": {func(p *ir.Package) {
			_, f := dependentField(p)
			f.Type.Args[0].From = types.ArgParam
		}, gogen.ErrDependentDisc, "demo.Thing"},
		"an argument from a dependent map's binder (CODEGEN.md §4.2)": {func(p *ir.Package) {
			_, f := dependentField(p)
			f.Type.Args[0].From = types.ArgKey
		}, gogen.ErrDependentDisc, "demo.Thing"},
		"an argument read through a field that holds no record": {func(p *ir.Package) {
			_, f := dependentField(p)
			f.Type.Args[0].WirePath = []string{"k", "sub"}
		}, gogen.ErrDependentDisc, "demo.Thing"},
		"an optional discriminant field, which check refuses": {func(p *ir.Package) {
			dependentField(p)
			optionalField(thing(p), "k")
		}, gogen.ErrDependentDisc, "demo.Thing"},
		"a discriminant of neither Bool nor enum type": {func(p *ir.Package) {
			d, _ := dependentField(p)
			d.Disc = &intT
			thing(p).Fields[1].Type = intT
		}, gogen.ErrDependentNoDisc, "demo.P"},
		"another package's dependent type": {func(p *ir.Package) {
			d, _ := dependentField(p)
			d.Pkg = "other"
			p.Types = p.Types[:len(p.Types)-1]
			p.Imports = []*ir.PackageRef{{Name: "other", Emits: []*ir.Emit{{Target: ir.TargetGo, GoImport: "example.com/other", GoPackage: "other"}}}}
		}, gogen.ErrDependentForeign, "demo.Thing"},
		"a ref into a load.defines table's define value getter": {func(p *ir.Package) {
			d, _ := dependentField(p)
			d.Branches[0].Type = ir.TypeRef{Kind: types.Ref, Ref: &ir.RefTarget{Coll: types.CollDefines, Pkg: "demo", Value: "defs"}}
		}, gogen.ErrDependentValue, "demo.P"},
		"a dependent type every arm of which is Never (log-2026-09-28)": {func(p *ir.Package) {
			d, _ := dependentField(p)
			d.Branches = nil
		}, gogen.ErrDependentNoBranch, "demo.P"},
		"a literal union over a dependent type": {func(p *ir.Package) {
			d, f := dependentField(p)
			d.Branches[0].Type = strT
			app := f.Type
			f.Type = ir.TypeRef{Kind: types.LitUnion, Elem: &app, Literals: []string{"none"}}
		}, gogen.ErrDependentUnion, "demo.Thing"},
	}
}

// Each dependent-type construct stage E refuses is ErrMalformed through its own cause, naming its subject (CODEGEN.md §5.6; decision 37).
func TestDependentRefusals(t *testing.T) {
	refusals := dependentRefusals()
	for _, name := range slices.Sorted(maps.Keys(refusals)) {
		p, r := dataThing(), refusals[name]
		r.edit(p)
		_, err := gogen.Generate(p, p.Emits[0])
		checkRefusal(t, name, err, r.cause, r.subject)
	}
}

// checkRefusal requires err to be a *DetailError about subject whose cause is cause, itself ErrMalformed (go.md §3: fields, never the message).
func checkRefusal(t *testing.T, name string, err, cause error, subject string) {
	t.Helper()
	var detail *gogen.DetailError
	if !errors.As(err, &detail) || detail.Subject != subject || !errors.Is(err, cause) || !errors.Is(err, gogen.ErrMalformed) {
		t.Errorf("%s: got %v, want a malformed-IR refusal of %s through its own cause", name, err, subject)
	}
}

// CODEGEN.md §5.6: baked gen/go writes a dependent value only as a field of its record, in the branch its discriminant selects.
func TestDependentBakedLiteralRefused(t *testing.T) {
	k := &ir.Enum{Pkg: "demo", Name: "K", Members: []*ir.EnumMember{{Name: "one", Wire: "one"}}}
	d := &ir.Dependent{
		Pkg: "demo", Name: "P", Params: 1, DiscParam: 0, Disc: &ir.TypeRef{Kind: types.Enum, Named: k},
		Branches: []*ir.Branch{{Name: "one", Members: []int{0}, Type: intT}},
	}
	v := &ir.Value{Name: "v", Type: ir.TypeRef{Kind: types.TypeApp, Named: d}, V: &value.Int{V: 1}}
	p := &ir.Package{
		Name: "demo", Dir: "demo", Types: []ir.Type{k, d}, Values: []*ir.Value{v},
		Emits: []*ir.Emit{{Target: ir.TargetGo, Out: "out/go/", Dir: "demo/out/go", GoImport: dataModule + "/demo/out/go", Mode: ir.ModeBaked, GoPackage: "demo"}},
	}
	_, err := gogen.Generate(p, p.Emits[0])
	checkRefusal(t, "a value of a dependent type", err, gogen.ErrDependentNested, "v")
	never := shapesPkg("depsbaked", ir.ModeBaked)
	never.Values[0].V.(*value.Table).Entries[1].Fields[6] = &value.Int{V: 1} // b's ev.k selects P's Never arm
	_, err = gogen.Generate(never, never.Emits[0])
	checkRefusal(t, "a value in a Never arm", err, gogen.ErrDependentNever, "depsbaked.Thing.s")
}

// CODEGEN.md §4.3: an optional dependent field's getter returns *T, nil for none, with no presence flag.
func TestOptionalDependentGetter(t *testing.T) {
	p := dataThing()
	dependentField(p)
	optionalField(thing(p), "p")
	src := string(generateData(t, p)["demo/out/go/demo.gen.go"])
	if !strings.Contains(src, "func (self *Thing) P() *P {") || strings.Contains(src, "p_ok") {
		t.Errorf("want P() *P and no p_ok flag:\n%s", src)
	}
}

// CODEGEN.md §5.6, §6.1: decode<T> is written only for a dependent type a decoded class holds.
func TestDependentNotDecodedNoDecoder(t *testing.T) {
	k := &ir.Enum{Pkg: "demo", Name: "K", Members: []*ir.EnumMember{{Name: "one", Wire: "one"}}}
	d := &ir.Dependent{
		Pkg: "demo", Name: "P", Params: 1, DiscParam: 0, Disc: &ir.TypeRef{Kind: types.Enum, Named: k},
		Branches: []*ir.Branch{{Name: "one", Members: []int{0}, Type: intT}},
	}
	p := dataThing()
	p.Types = append(p.Types, k, d)
	files, err := gogen.Generate(p, p.Emits[0])
	if err != nil {
		t.Fatalf("got %v, want success", err)
	}
	src := string(files[len(files)-1].Content)
	if !strings.Contains(src, "type P struct") {
		t.Error("want the dependent type declared")
	}
	if strings.Contains(src, "func decodeP(") {
		t.Error("decode<T> only when a decoded class holds the dependent type (CODEGEN.md §5.6)")
	}
}
