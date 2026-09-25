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

// dependentRefusals: one row per dependent-type construct data mode refuses (CODEGEN.md §5.6).
func dependentRefusals() map[string]func(*ir.Package) {
	return map[string]func(*ir.Package){
		"a dependent type with a Bool discriminant": func(p *ir.Package) {
			d, _ := dependentField(p)
			d.Disc = &ir.TypeRef{Kind: types.Bool}
		},
		"a dependent type whose match reads further than its parameter": func(p *ir.Package) {
			d, _ := dependentField(p)
			d.DiscPath = []string{"x"}
		},
		"a dependent type argument from a record's own parameter": func(p *ir.Package) {
			_, f := dependentField(p)
			f.Type.Args[0].From = types.ArgParam
		},
		"a dependent type argument from a dependent map's binder": func(p *ir.Package) {
			_, f := dependentField(p)
			f.Type.Args[0].From = types.ArgKey
		},
		"a dependent type argument read through more than one field": func(p *ir.Package) {
			_, f := dependentField(p)
			f.Type.Args[0].WirePath = []string{"k", "sub"}
		},
		"a dependent type inside a list field": func(p *ir.Package) {
			_, f := dependentField(p)
			f.Type = listT(f.Type)
		},
		"a dependent type discriminant field that is optional": func(p *ir.Package) {
			dependentField(p)
			optionalField(thing(p), "k")
		},
		"a ref into a load.defines table's define value getter": func(p *ir.Package) {
			d, _ := dependentField(p)
			d.Branches[0].Type = ir.TypeRef{Kind: types.Ref, Ref: &ir.RefTarget{Coll: types.CollDefines, Pkg: "demo", Value: "defs"}}
		},
	}
}

// Each dependent-type refusal names its cause through ErrUnsupported (CODEGEN.md §5.6).
func TestDependentRefusals(t *testing.T) {
	refusals := dependentRefusals()
	for _, name := range slices.Sorted(maps.Keys(refusals)) {
		p := dataThing()
		refusals[name](p)
		_, err := gogen.Generate(p, p.Emits[0])
		if !errors.Is(err, gogen.ErrUnsupported) {
			t.Errorf("%s: got %v, want ErrUnsupported", name, err)
		}
	}
}

// CODEGEN.md §5.6: a baked or embedded literal of a dependent type has its own refusal text.
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
	if !errors.Is(err, gogen.ErrUnsupported) {
		t.Errorf("got %v, want ErrUnsupported", err)
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
