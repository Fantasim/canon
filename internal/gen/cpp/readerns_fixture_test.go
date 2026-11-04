package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	rnsLibPkg      = "demo.rdlib"
	rnsOwnPkg      = "demo.rdown"
	rnsDataPkg     = "demo.rddata"
	rnsTypesPkg    = "demo.rdtypes"
	rnsSchema      = "demo.rddata.H@00000001"
	rnsItemsSchema = "demo.rdlib.items@00000001"
)

// readerNS is the IR of four packages: rdlib, a data-mode library (its tables' ids are strings) holding a Shape enum and a table of items, each with a shape; rdown, a baked package whose class H holds a ref into that table and a Param(ev.shape) decided through it, so H holds no Shape; and rddata (data) and rdtypes (types), which read H. The branch table of their reader names rdlib's Shape (CODEGEN.md §2.8, §5.6, DECISIONS 350, 456).
type readerNS struct {
	lib, own, data, typed *ir.Package
}

func newReaderNS() *readerNS {
	libEmit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "rd/lib", Mode: ir.ModeData, Namespace: "demo::rdlib"}
	ownEmit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "rd/own", Mode: ir.ModeBaked, Namespace: "demo::rdown"}
	shape := &ir.Enum{Pkg: rnsLibPkg, Name: "Shape", Members: []*ir.EnumMember{
		{Name: "count", Wire: "count"}, {Name: "place", Wire: "place", Index: 1}, {Name: "none_", Wire: "none", Index: 2},
	}}
	tShape := ir.TypeRef{Kind: types.Enum, Named: shape}
	item := &ir.Record{Pkg: rnsLibPkg, Name: "Item", Fields: []*ir.Field{field("shape", "shape", "", tShape)}}
	itemElem := ir.TypeRef{Kind: types.Record, Named: item}
	items := &ir.Value{Name: "items", Schema: rnsItemsSchema, Type: ir.TypeRef{Kind: types.Table, Elem: &itemElem}}
	lib := &ir.Package{Name: rnsLibPkg, Dir: "rd/lib", Types: []ir.Type{shape, item}, Values: []*ir.Value{items}, Emits: []*ir.Emit{libEmit}}
	libRef := &ir.PackageRef{Name: rnsLibPkg, Dir: "rd/lib", Emits: lib.Emits}

	param := &ir.Dependent{
		Pkg: rnsOwnPkg, Name: "Param", Params: 1, Disc: &tShape, ByMember: []int{0, 1, ir.NoBranch},
		Branches: []*ir.Branch{{Name: "count", Members: []int{0}, Type: tInt}, {Name: "place", Members: []int{1}, Type: tString}},
	}
	key := tString
	ref := ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: rnsLibPkg, Value: "items", Elem: item}}
	paramField := field("param", "param", "", ir.TypeRef{Kind: types.TypeApp, Named: param, Args: []*ir.Source{{From: types.ArgField, WirePath: []string{"ev", "shape"}}}})
	paramField.Optional, paramField.NoneWire = true, []byte("0")
	paramField.Branches = &ir.BranchTable{Keys: []value.Key{{S: "shield"}, {S: "sword"}}, Branches: []int{1, 0}}
	h := &ir.Record{Pkg: rnsOwnPkg, Name: "H", Fields: []*ir.Field{field("ev", "ev", "", ref), paramField}}
	own := &ir.Package{
		Name: rnsOwnPkg, Dir: "rd/own", Types: []ir.Type{param, h}, Emits: []*ir.Emit{ownEmit},
		Imports: []*ir.PackageRef{libRef},
	}
	ownRef := &ir.PackageRef{Name: rnsOwnPkg, Dir: "rd/own", Emits: own.Emits}
	return &readerNS{lib: lib, own: own, data: rnsData(h, ownRef, libRef), typed: rnsTypes(h, ownRef, libRef)}
}

// rnsData is rddata, the data-mode package of one value of H, whose imports are the owner's and the library's.
func rnsData(h *ir.Record, own, lib *ir.PackageRef) *ir.Package {
	hT := pkgRecordType(rnsOwnPkg, "H", "ev", "param")
	v := &ir.Value{Name: "h", Schema: rnsSchema, Type: ir.TypeRef{Kind: types.Record, Named: h}, V: &value.Record{T: hT, Fields: []value.Value{key("sword"), num(3)}}}
	return &ir.Package{
		Name: rnsDataPkg, Dir: "rd/data", Values: []*ir.Value{v}, Imports: []*ir.PackageRef{own, lib},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "rd/data", Mode: ir.ModeData, Namespace: "demo::rddata"}},
	}
}

// rnsTypes is rdtypes, the types-mode package of a record W that holds an H, whose imports are the owner's and the library's.
func rnsTypes(h *ir.Record, own, lib *ir.PackageRef) *ir.Package {
	w := &ir.Record{Pkg: rnsTypesPkg, Name: "W", Fields: []*ir.Field{field("h", "h", "", ir.TypeRef{Kind: types.Record, Named: h})}}
	return &ir.Package{
		Name: rnsTypesPkg, Dir: "rd/typed", Types: []ir.Type{w}, Imports: []*ir.PackageRef{own, lib},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "rd/typed", Mode: ir.ModeTypes, Namespace: "demo::rdtypes"}},
	}
}

func readerNSData() *ir.Package { return newReaderNS().data }

func readerNSTypes() *ir.Package { return newReaderNS().typed }
