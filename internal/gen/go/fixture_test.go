package gogen_test

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// The fixtures of testdata/ir are emit IR written as JSON, the way stage E will build it:
// types by name ("[ref statuses]", "Role?"), values as JSON, one entry per table row.
type fixture struct {
	Module   string     `json:"module"`
	Packages []*pkgSpec `json:"packages"`
}

type pkgSpec struct {
	Name, Dir, Doc string
	Emit           emitSpec
	Imports        []string
	Consts         []constSpec
	Enums          []enumSpec
	Records        []recordSpec
	Variants       []variantSpec
	Values         []valueSpec
	Fns            []fnSpec
}

type emitSpec struct {
	Out, Dir, Import, Package, Mode string
	Values                          []string
}

type enumSpec struct {
	Name, Doc, Codes, GoName string
	Ordered                  bool
	Members                  []string
	CodeValues               []int64
	Retired                  []string
}

type constSpec struct {
	Name, Doc, Type string
	Value           json.RawMessage
}

type fieldSpec struct {
	Name, Doc, Type, GoName string
	Default                 json.RawMessage
	Stable                  bool
}

type recordSpec struct {
	Name, Doc string
	Fields    []fieldSpec
	Fns       []fnSpec
}

type variantSpec struct {
	Name, Doc string
	Cases     []recordSpec
}

type valueSpec struct {
	Name, Doc, Type string
	Value           json.RawMessage
	Entries         []entrySpec
}

type entrySpec struct {
	ID      string
	Retired bool
	Fields  map[string]json.RawMessage
}

type fnSpec struct {
	Name, Doc, Result string
	Params            []paramSpec
	Value             json.RawMessage
	Cells             []json.RawMessage
}

type paramSpec struct {
	Name, Type string
}

// named is a declared type: its IR and the checker's twin that values point at.
type named struct {
	ir      ir.Type
	enum    *types.EnumType
	rec     *types.RecordType
	variant *types.VariantType
}

// world builds the packages of one fixture; cells computes the lookup tables it does not list.
type world struct {
	t      *testing.T
	fx     *fixture
	pkgs   []*ir.Package
	byName map[string]*ir.Package
	types  map[string]*named
	values map[string]*valueSpec // "pkg.value"
	built  map[string]*ir.Value
	cur    *pkgSpec
	cells  func(w *world, fn *ir.ExportFn) []value.Value
}

func loadWorld(t *testing.T, path string, cells func(*world, *ir.ExportFn) []value.Value) *world {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	w := &world{
		t: t, fx: &fixture{}, byName: map[string]*ir.Package{}, types: map[string]*named{},
		values: map[string]*valueSpec{}, built: map[string]*ir.Value{}, cells: cells,
	}
	if err := json.Unmarshal(raw, w.fx); err != nil {
		t.Fatal(err)
	}
	for _, ps := range w.fx.Packages {
		w.cur = ps
		w.pkgs = append(w.pkgs, w.buildPackage(ps))
	}
	return w
}

func (w *world) buildPackage(ps *pkgSpec) *ir.Package {
	p := &ir.Package{Name: ps.Name, Dir: ps.Dir, Doc: ps.Doc}
	w.byName[ps.Name] = p
	for i := range ps.Values {
		w.values[ps.Name+"."+ps.Values[i].Name] = &ps.Values[i]
	}
	for _, imp := range ps.Imports {
		p.Imports = append(p.Imports, &ir.PackageRef{Name: imp, Dir: w.byName[imp].Dir, Emits: w.byName[imp].Emits})
	}
	mode := ir.ModeBaked
	if ps.Emit.Mode == "data" {
		mode = ir.ModeData
	}
	p.Emits = []*ir.Emit{{
		Target: ir.TargetGo, Out: ps.Emit.Out, Dir: ps.Emit.Dir, GoImport: ps.Emit.Import,
		Mode: mode, GoPackage: ps.Emit.Package, Values: ps.Emit.Values,
	}}
	for _, e := range ps.Enums {
		p.Types = append(p.Types, w.enum(ps.Name, e))
	}
	w.declareRecords(p, ps)
	for _, c := range ps.Consts {
		t, _ := w.typeRef(c.Type)
		p.Consts = append(p.Consts, &ir.Const{Name: c.Name, Doc: c.Doc, Type: t, V: w.toValue(t, false, c.Value)})
	}
	for _, v := range ps.Values {
		p.Values = append(p.Values, w.value(ps.Name, v))
	}
	for _, f := range ps.Fns {
		p.Fns = append(p.Fns, w.fn(f, nil))
	}
	return p
}

func (w *world) enum(pkg string, s enumSpec) *ir.Enum {
	e := &ir.Enum{Pkg: pkg, Name: s.Name, Doc: s.Doc, Ordered: s.Ordered, Go: ir.NameOptions{Name: s.GoName}}
	twin := &types.EnumType{Pkg: pkg, Name: s.Name, Ordered: s.Ordered}
	if s.Codes != "" {
		codes, _ := w.typeRef(s.Codes)
		e.Codes = &codes
	}
	for i, m := range s.Members {
		name, wire, _ := strings.Cut(m, "=")
		if wire == "" {
			wire = name
		}
		member := &ir.EnumMember{Name: name, Wire: wire, Index: i, Retired: slices.Contains(s.Retired, name)}
		if i < len(s.CodeValues) {
			member.Code = s.CodeValues[i]
		}
		e.Members = append(e.Members, member)
		twin.Members = append(twin.Members, &types.Member{Name: name, Wire: wire, Index: i, Code: member.Code})
	}
	w.types[s.Name] = &named{ir: e, enum: twin}
	return e
}

// declareRecords declares records and variants first, then fills their fields: a field may
// name a type declared after it.
func (w *world) declareRecords(p *ir.Package, ps *pkgSpec) {
	for _, r := range ps.Records {
		rec := &ir.Record{Pkg: ps.Name, Name: r.Name, Doc: r.Doc}
		w.types[r.Name] = &named{ir: rec, rec: &types.RecordType{Pkg: ps.Name, Name: r.Name}}
		p.Types = append(p.Types, rec)
	}
	for _, v := range ps.Variants {
		iv := &ir.Variant{Pkg: ps.Name, Name: v.Name, Doc: v.Doc, Tag: "kind"}
		twin := &types.VariantType{Pkg: ps.Name, Name: v.Name}
		for i, c := range v.Cases {
			iv.Cases = append(iv.Cases, &ir.Case{Name: c.Name, Wire: c.Name, Doc: c.Doc})
			twin.Cases = append(twin.Cases, &types.CaseType{Variant: twin, Name: c.Name, Wire: c.Name, Index: i})
		}
		w.types[v.Name] = &named{ir: iv, variant: twin}
		p.Types = append(p.Types, iv)
	}
	for _, r := range ps.Records {
		n := w.types[r.Name]
		rec := n.ir.(*ir.Record)
		rec.Fields, n.rec.Fields = w.fields(r.Fields)
		for _, f := range r.Fns {
			rec.Methods = append(rec.Methods, w.fn(f, rec))
		}
	}
	for _, v := range ps.Variants {
		n := w.types[v.Name]
		for i, c := range v.Cases {
			ic := n.ir.(*ir.Variant).Cases[i]
			ic.Fields, n.variant.Cases[i].Fields = w.fields(c.Fields)
		}
	}
}

func (w *world) fields(specs []fieldSpec) ([]*ir.Field, []*types.Field) {
	var fs []*ir.Field
	var twins []*types.Field
	for i, s := range specs {
		t, opt := w.typeRef(s.Type)
		fs = append(fs, &ir.Field{
			Name: s.Name, Doc: s.Doc, WirePath: []string{s.Name}, Type: t, Optional: opt,
			Stable: s.Stable, Go: ir.NameOptions{Name: s.GoName},
		})
		twins = append(twins, &types.Field{Name: s.Name, Index: i, Wire: s.Name, WirePath: []string{s.Name}})
	}
	return fs, twins
}
