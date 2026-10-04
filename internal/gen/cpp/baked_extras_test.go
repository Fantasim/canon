package cppgen_test

import (
	"strings"
	"testing"

	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// extras adds a map of records, a table field, a keyed-list lookup a translated fn calls, a translated table id, a literal union result (decision 296).
func (b *board) extras(p *ir.Package) {
	slot := &ir.Record{Pkg: boardPkg, Name: "Slot", Fields: []*ir.Field{field("n", "n", "", tInt)}}
	shelf := &ir.Record{Pkg: boardPkg, Name: "Shelf", Fields: []*ir.Field{field("slots", "slots", "", ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: slot}})}}
	crew := &ir.Record{Pkg: boardPkg, Name: "Crew", Fields: []*ir.Field{field("name", "name", "", tString), field("size", "size", "", tInt)}}
	p.Types = append(p.Types, slot, shelf, crew)
	b.crew = crew
	slotT, shelfT, crewT := recordType("Slot", "n"), recordType("Shelf", "slots"), recordType("Crew", "name", "size")
	slotColl := &types.Collection{Kind: types.CollField, Pkg: boardPkg, Name: "slots"}
	slots := &value.Table{Entries: []*value.Record{
		{T: slotT, Fields: []value.Value{num(1)}, Ident: &value.Identity{Coll: slotColl, Key: value.Key{S: "a"}}},
		{T: slotT, Fields: []value.Value{num(2)}, Ident: &value.Identity{Coll: slotColl, Key: value.Key{S: "b"}, Retired: true}},
	}}
	crewColl := &types.Collection{Kind: types.CollLet, Pkg: boardPkg, Name: "crews"}
	crewRow := func(name string, size int64) value.Value {
		return &value.Record{T: crewT, Fields: []value.Value{str(name), num(size)}, Ident: &value.Identity{Coll: crewColl, Key: value.Key{S: name}}}
	}
	pointT := ir.TypeRef{Kind: types.Record, Named: b.point}
	crewElem := ir.TypeRef{Kind: types.Record, Named: crew}
	p.Values = append(p.Values,
		&ir.Value{Name: "points", Type: ir.TypeRef{Kind: types.Map, Key: &tString, Elem: &pointT},
			V: &value.Map{Keys: []value.Value{str("far"), str("near")}, Vals: []value.Value{&value.Record{T: b.pointT, Fields: []value.Value{num(9), num(8)}}, &value.Record{T: b.pointT, Fields: []value.Value{num(1), num(0)}}}}},
		&ir.Value{Name: "shelf", Type: ir.TypeRef{Kind: types.Record, Named: shelf}, V: &value.Record{T: shelfT, Fields: []value.Value{slots}}},
		&ir.Value{Name: "crews", Type: ir.TypeRef{Kind: types.List, Elem: &crewElem, KeyedBy: &ir.KeyField{Name: "name", WirePath: []string{"name"}}}, V: list(crewRow("ann", 2), crewRow("bob", 5))},
	)
	p.Fns = append(p.Fns, b.extraFns()...)
}

func (b *board) extraFns() []*ir.ExportFn {
	tTone := ir.TypeRef{Kind: types.Enum, Named: b.tone}
	byTone := []*ir.Param{{Name: "t", Type: tTone}}
	crewRef := ir.TypeRef{Kind: types.Ref, Key: &tString, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: boardPkg, Value: "crews", Elem: b.crew, Keyed: true}}
	crewOf := lookup("crewOf", "", crewRef, byTone, key("ann"), key("bob"), key("ann"))
	colRef := boardRef("columns", b.column)
	mood := ir.TypeRef{Kind: types.LitUnion, Literals: []string{"calm", "angry"}, Elem: &tString}
	tToneParam := &ir.ParamRef{T: tTone, Index: 0}
	return []*ir.ExportFn{
		crewOf,
		lookup("moodOf", "", mood, byTone, str("calm"), str("angry"), str("calm")),
		{
			Name: "crewName", File: "board.canon", Kind: ir.FnTranslated, Result: crewRef, Order: 2, Params: byTone,
			Body:    &ir.CallFn{T: crewRef, Fn: crewOf, Args: []ir.PExpr{tToneParam}},
			Vectors: []*ir.Vector{vec(nil, []value.Value{member(1)}, key("bob"), "")},
		},
		{
			Name: "pickColumn", File: "board.canon", Kind: ir.FnTranslated, Result: colRef, Order: 3, Params: []*ir.Param{{Name: "n", Type: tInt}},
			Body: &ir.If{T: colRef, Cond: &ir.Binary{T: tBool, Op: ir.OpGt, X: &ir.ParamRef{T: tInt, Index: 0}, Y: lit(tInt, num(0))},
				Then: lit(colRef, key("todo")), Else: lit(colRef, key("done"))},
			Vectors: []*ir.Vector{vec(nil, []value.Value{num(1)}, key("todo"), ""), vec(nil, []value.Value{num(0)}, key("done"), "")},
		},
	}
}

// CODEGEN.md §5.10, DECISIONS 296: a ref result into a table with no id enum in the emit (an import's data-mode table, keyed by String) keeps the split.
func TestBakedForeignTableResultSplit(t *testing.T) {
	far := &ir.Record{Pkg: "far", Name: "Item", Fields: []*ir.Field{field("n", "n", "", tInt)}}
	tone := enumOf("Tone", "a", "b")
	farRef := ir.TypeRef{Kind: types.Ref, Key: &tString, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: "far", Value: "items", Elem: far}}
	fn := lookup("farOf", "", farRef, []*ir.Param{{Name: "t", Type: ir.TypeRef{Kind: types.Enum, Named: tone}}}, key("x"), key("y"))
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "demo/out", Mode: ir.ModeBaked, Namespace: "demo"}
	p := &ir.Package{Name: "demo", Dir: "demo", Types: []ir.Type{tone}, Fns: []*ir.ExportFn{fn}, Emits: []*ir.Emit{emit},
		Imports: []*ir.PackageRef{{Name: "far", Dir: "far", Emits: []*ir.Emit{{Target: ir.TargetCpp, Dir: "far/out", Mode: ir.ModeData, Namespace: "far"}}}}}
	files, err := cppgen.Generate(p, emit)
	if err != nil {
		t.Fatal(err)
	}
	header, source := string(files[1].Content), string(files[2].Content)
	if !strings.Contains(header, "\nconst std::string& FarOf(Tone t);\n") || strings.Contains(header, "kFarOfCells") || !strings.Contains(source, "const std::string& FarOf(Tone t) {") {
		t.Errorf("FarOf is not declared in the header and defined in the source:\n%s\n%s", header, source)
	}
}
