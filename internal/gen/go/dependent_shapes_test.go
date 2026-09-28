package gogen_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// shapes are the declarations of shapesPkg: K, Ev, P over K with a Never arm, Q over a Bool,
// R over Ev's k with a wildcard arm, and Thing, holding one field of each shape.
type shapes struct {
	k           *ir.Enum
	ev, thing   *ir.Record
	evT, thingT *types.RecordType
	p, q, r     *ir.Dependent
	kT, evRef   ir.TypeRef
}

func newShapes(pkg string) *shapes {
	s := &shapes{k: &ir.Enum{Pkg: pkg, Name: "K", Members: []*ir.EnumMember{
		{Name: "one", Wire: "one"}, {Name: "two", Wire: "two", Index: 1}, {Name: "three", Wire: "three", Index: 2},
	}}}
	s.kT = ir.TypeRef{Kind: types.Enum, Named: s.k}
	s.ev = &ir.Record{Pkg: pkg, Name: "Ev", Fields: []*ir.Field{wired("k", "k", "", s.kT)}}
	s.evRef = ir.TypeRef{Kind: types.Record, Named: s.ev}
	s.p = &ir.Dependent{Pkg: pkg, Name: "P", Params: 1, Disc: &s.kT, ByMember: []int{0, 1, ir.NoBranch},
		Branches: []*ir.Branch{{Name: "one", Members: []int{0}, Type: intT}, {Name: "two", Members: []int{1}, Type: strT}}}
	s.q = &ir.Dependent{Pkg: pkg, Name: "Q", Params: 1, Disc: &boolT, ByMember: []int{0, 1},
		Branches: []*ir.Branch{{Name: "false", Members: []int{0}, Type: fltT}, {Name: "true", Members: []int{1}, Type: strT}}}
	s.r = &ir.Dependent{Pkg: pkg, Name: "R", Params: 1, DiscPath: []string{"k"}, Disc: &s.kT, ByMember: []int{0, 1, 1},
		Branches: []*ir.Branch{{Name: "one", Members: []int{0}, Type: i32T}, {Name: "two", Members: []int{1, 2}, Type: strT}}}
	app := func(d *ir.Dependent, from ...string) ir.TypeRef {
		return ir.TypeRef{Kind: types.TypeApp, Named: d, Args: []*ir.Source{{From: types.ArgField, WirePath: from}}}
	}
	s.thing = &ir.Record{Pkg: pkg, Name: "Thing", Fields: []*ir.Field{
		wired("k", "k", "", s.kT), wired("on", "on", "", boolT), wired("ev", "ev", "", s.evRef),
		wired("ps", "ps", "", listT(app(s.p, "k"))), wired("q", "q", "", app(s.q, "on")),
		wired("r", "r", "", app(s.r, "ev")), opt(wired("s", "s", "", app(s.p, "ev", "k"))),
	}}
	s.evT = &types.RecordType{Pkg: pkg, Name: "Ev", Fields: []*types.Field{{Name: "k"}}}
	names := []string{"k", "on", "ev", "ps", "q", "r", "s"}
	s.thingT = &types.RecordType{Pkg: pkg, Name: "Thing"}
	for i, n := range names {
		s.thingT.Fields = append(s.thingT.Fields, &types.Field{Name: n, Index: i})
	}
	return s
}

// shapesPkg is Thing's table things in mode (TYPES.md §11.1, §11.2): baked holds rows a and b, one value of each shape.
func shapesPkg(pkg string, mode ir.Mode) *ir.Package {
	s := newShapes(pkg)
	elem := ir.TypeRef{Kind: types.Record, Named: s.thing}
	things := &ir.Value{Name: "things", Schema: pkg + ".Thing@00000001", IDs: []string{"a", "b"}, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}}
	if mode == ir.ModeBaked {
		things.V = &value.Table{Entries: []*value.Record{s.row("a", 0, true, 0, []value.Value{&value.Int{V: 1}, &value.Int{V: 2}},
			&value.Str{V: "yes"}, &value.Int{V: 7}, &value.Int{V: 9}),
			s.row("b", 1, false, 2, []value.Value{&value.Str{V: "p"}}, &value.Float{V: 2.5}, &value.Str{V: "y"}, &value.None{})}}
	}
	e := &ir.Emit{Target: ir.TargetGo, Out: "out/go/", Dir: pkg + "/out/go", GoImport: dataModule + "/" + pkg + "/out/go", Mode: mode, GoPackage: pkg}
	return &ir.Package{Name: pkg, Dir: pkg, Types: []ir.Type{s.k, s.ev, s.p, s.q, s.r, s.thing}, Values: []*ir.Value{things}, Emits: []*ir.Emit{e}}
}

// row is one entry of things: k and ev.k by member index, on, then ps, q, r and s as given.
func (s *shapes) row(id string, k int, on bool, evK int, ps []value.Value, q, r, sv value.Value) *value.Record {
	ev := &value.Record{T: s.evT, Fields: []value.Value{&value.Member{Index: evK}}}
	return &value.Record{T: s.thingT, Ident: &value.Identity{Key: value.Key{S: id}}, Fields: []value.Value{
		&value.Member{Index: k}, &value.Bool{V: on}, ev, &value.List{Elems: ps}, q, r, sv,
	}}
}

// CODEGEN.md §5.6: the data loader reads a list sharing its field's discriminant, a Bool one, a match path, an argument through two fields, and refuses a value in a Never arm.
func TestDependentShapesDataCompiles(t *testing.T) {
	files := generateData(t, shapesPkg("deps", ir.ModeData))
	data := map[string][]byte{
		"good/things.json": []byte(`{"$schema": "deps.Thing@00000001", "rows": [
  {"$id": "a", "k": "one", "on": true, "ev": {"k": "one"}, "ps": [1, 2], "q": "yes", "r": 7, "s": 9},
  {"$id": "b", "k": "two", "on": false, "ev": {"k": "three"}, "ps": ["p"], "q": 2.5, "r": "y"}
]}
`),
		"bad/things.json": []byte(`{"$schema": "deps.Thing@00000001", "rows": [
  {"$id": "c", "k": "one", "on": true, "ev": {"k": "three"}, "ps": [], "q": "x", "r": "z", "s": 1}
]}
`),
	}
	runData(t, files, "deps/out/go", "testdata/smoke/dependent_shapes_test.go", data)
}

// CODEGEN.md §5.6: a baked literal holds each value in its record's branch, typed as As<Branch> returns it (an Int32 is int32).
func TestDependentShapesBakedCompiles(t *testing.T) {
	files := generateData(t, shapesPkg("depsbaked", ir.ModeBaked))
	runData(t, files, "depsbaked/out/go", "testdata/smoke/dependent_baked_test.go", nil)
}
