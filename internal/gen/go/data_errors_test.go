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

// dataThing is a data-mode package holding record Thing {a: Int} in the keyed list things.
func dataThing() *ir.Package {
	rec := &ir.Record{Pkg: "demo", Name: "Thing", Fields: []*ir.Field{wired("a", "a", "", intT)}}
	p := &ir.Package{Name: "demo", Dir: "demo", Types: []ir.Type{rec}, Emits: []*ir.Emit{goData("demo", "demo")}}
	thingValues(p, false, "things")
	return p
}

func thing(p *ir.Package) *ir.Record { return p.Types[0].(*ir.Record) }

// thingValues adds keyed lists of Thing, keyed by a, named names.
func thingValues(p *ir.Package, reload bool, names ...string) {
	elem := typed(thing(p), types.Record)
	for _, n := range names {
		p.Values = append(p.Values, &ir.Value{Name: n, Reload: reload, Schema: "demo.Thing@00000001", Type: ir.TypeRef{
			Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "a", WirePath: []string{"a"}},
		}})
	}
}

func addField(p *ir.Package, f *ir.Field)      { thing(p).Fields = append(thing(p).Fields, f) }
func addMethod(p *ir.Package, fn *ir.ExportFn) { thing(p).Methods = append(thing(p).Methods, fn) }

// dataRefusals: one row per construct data mode refuses rather than emits wrongly (decision 37: each a stage-E follow-up).
func dataRefusals() map[string]func(*ir.Package) {
	intKey := intT
	thingRef := func(p *ir.Package, value string) ir.TypeRef {
		return ir.TypeRef{Kind: types.Ref, Key: &intKey, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: "demo", Value: value, Elem: thing(p), Keyed: true}}
	}
	return map[string]func(*ir.Package){
		"a package-level stored fn": func(p *ir.Package) { p.Fns = []*ir.ExportFn{{Name: "f", Kind: ir.FnPrecomputed, Result: intT}} },
		"a lookup over a table's ref": func(p *ir.Package) {
			elem := typed(thing(p), types.Record)
			p.Values = append(p.Values, &ir.Value{Name: "ts", Schema: "s", IDs: []string{"a"}, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}})
			param := ir.TypeRef{Kind: types.Ref, Key: &strT, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: "demo", Value: "ts", Elem: thing(p)}}
			addMethod(p, &ir.ExportFn{Name: "f", Kind: ir.FnLookup, Result: intT, Params: []*ir.Param{{Name: "t", Type: param}}})
		},
		"a lookup whose record result holds a resolved ref": func(p *ir.Package) {
			addField(p, wired("peer", "peer", "", thingRef(p, "things")))
			addMethod(p, &ir.ExportFn{Name: "f", Kind: ir.FnLookup, Result: typed(thing(p), types.Record), Params: []*ir.Param{{Name: "b", Type: boolT}}})
		},
		"a resolvable ref in a record two values hold": func(p *ir.Package) {
			p.Values = nil
			thingValues(p, true, "left", "right")
			addField(p, wired("peer", "peer", "", thingRef(p, "left")))
		},
		"a data value that is a plain list": func(p *ir.Package) {
			p.Values = append(p.Values, &ir.Value{Name: "l", Schema: "s", Type: listT(intT)})
		},
		"a map field": func(p *ir.Package) {
			addField(p, wired("m", "m", "", ir.TypeRef{Kind: types.Map, Key: &strT, Elem: &intT}))
		},
		"a list of optionals": func(p *ir.Package) {
			addField(p, wired("o", "o", "", listT(optT(intT))))
		},
		"a record of another package": func(p *ir.Package) {
			other := &ir.Record{Pkg: "other", Name: "O"}
			p.Imports = []*ir.PackageRef{{Name: "other", Emits: []*ir.Emit{goData("other", "other")}}}
			addField(p, wired("o", "o", "", typed(other, types.Record)))
		},
		"a non-string literal union": func(p *ir.Package) {
			addField(p, wired("u", "u", "", ir.TypeRef{Kind: types.LitUnion, Elem: &intT, Literals: []string{"none"}}))
		},
		"an optional inline variant": func(p *ir.Package) {
			v := &ir.Variant{Pkg: "demo", Name: "V", Tag: "k", Cases: []*ir.Case{{Name: "a", Wire: "a"}}}
			p.Types = append(p.Types, v)
			addField(p, &ir.Field{Name: "v", Type: typed(v, types.Variant), Inline: true, Optional: true})
		},
		"keys equal but for letter case": func(p *ir.Package) { addField(p, wired("b", "A", "", intT)) },
		"a table-typed field":            func(p *ir.Package) { addField(p, wired("t", "t", "", ir.TypeRef{Kind: types.Table, Elem: &intT})) },
	}
}

// Each refusal names its cause through ErrUnsupported (decision 124).
func TestDataRefusals(t *testing.T) {
	refusals := dataRefusals()
	for _, name := range slices.Sorted(maps.Keys(refusals)) {
		p := dataThing()
		refusals[name](p)
		_, err := gogen.Generate(p, p.Emits[0])
		if !errors.Is(err, gogen.ErrUnsupported) {
			t.Errorf("%s: got %v, want ErrUnsupported", name, err)
		}
	}
}

// CODEGEN.md §3.5, decision 203: `const STORE` is Store, the store variable, refused though ir plans baked names only.
func TestDataNameCollision(t *testing.T) {
	p := dataThing()
	p.Values[0].Reload = true
	p.Consts = []*ir.Const{{Name: "STORE", Type: intT, V: &value.Int{V: 1}}}
	_, err := gogen.Generate(p, p.Emits[0])
	var d *gogen.DetailError
	if !errors.Is(err, gogen.ErrNameCollision) || !errors.As(err, &d) || d.Subject != "Store" {
		t.Errorf("got %v, want ErrNameCollision naming Store", err)
	}
}

// CODEGEN.md §3.4, decision 182: a loader's local named like an imported package is escaped.
func TestDataLocalsEscaped(t *testing.T) {
	p := dataThing()
	role := &ir.Enum{Pkg: "names", Name: "Role", Members: []*ir.EnumMember{{Name: "a", Wire: "a"}}}
	p.Imports = []*ir.PackageRef{{Name: "names", Emits: []*ir.Emit{{Target: ir.TargetGo, GoImport: "example.com/name", GoPackage: "name", Mode: ir.ModeBaked}}}}
	addField(p, wired("role", "role", "", typed(role, types.Enum)))
	files, err := gogen.Generate(p, p.Emits[0])
	if err != nil {
		t.Fatal(err)
	}
	src := string(files[len(files)-1].Content)
	for _, want := range []string{"func decodeThing(name_, path string", "name.ParseRole(*w.Role)"} {
		if !strings.Contains(src, want) {
			t.Errorf("no %q in\n%s", want, src)
		}
	}
}

// CODEGEN.md §3.4, §3.5: a data-mode method never declares a parameter twice, nor one named like an import.
func TestDataParamCollisions(t *testing.T) {
	twice := dataThing()
	addMethod(twice, &ir.ExportFn{Name: "f", Kind: ir.FnLookup, Result: intT, Params: []*ir.Param{{Name: "a_b", Type: boolT}, {Name: "aB", Type: boolT}}})
	onImport := dataThing()
	q := &ir.Enum{Pkg: "q", Name: "Q", Members: []*ir.EnumMember{{Name: "x", Wire: "x"}}}
	q2 := &ir.Enum{Pkg: "q_", Name: "R", Members: []*ir.EnumMember{{Name: "y", Wire: "y"}}}
	onImport.Imports = []*ir.PackageRef{
		{Name: "q", Emits: []*ir.Emit{{Target: ir.TargetGo, GoImport: "example.com/q", GoPackage: "q", Mode: ir.ModeBaked}}},
		{Name: "q_", Emits: []*ir.Emit{{Target: ir.TargetGo, GoImport: "example.com/q_", GoPackage: "q_", Mode: ir.ModeBaked}}},
	}
	addMethod(onImport, &ir.ExportFn{Name: "f", Kind: ir.FnLookup, Result: intT, Params: []*ir.Param{
		{Name: "q", Type: typed(q, types.Enum)}, {Name: "r", Type: typed(q2, types.Enum)},
	}})
	cases := []struct {
		name, subject string
		p             *ir.Package
	}{{"parameters a_b and aB", "aB", twice}, {"a parameter q escaped onto the import q_", "q_", onImport}}
	for _, c := range cases {
		_, err := gogen.Generate(c.p, c.p.Emits[0])
		var d *gogen.DetailError
		if !errors.Is(err, gogen.ErrNameCollision) || !errors.As(err, &d) || d.Subject != c.subject {
			t.Errorf("%s: got %v, want ErrNameCollision naming %s", c.name, err, c.subject)
		}
	}
}
