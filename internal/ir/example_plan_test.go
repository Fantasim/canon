package ir_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

func shop(target ir.Target) (*ir.Package, *ir.Record) {
	intT := ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true}
	item := &ir.Record{Pkg: "shop", Name: "Item"}
	ref := ir.TypeRef{Kind: types.Ref, Key: &intT, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: "shop", Value: "items", Elem: item, Keyed: true}}
	times := &ir.ExportFn{
		Name: "times", Kind: ir.FnTranslated, Result: intT, Params: []*ir.Param{{Name: "int", Type: intT}},
		Reads: []*ir.Read{{Name: "price", Path: []string{"price"}, Type: intT}},
	}
	item.Fields = []*ir.Field{{Name: "price", Type: intT}, {Name: "next", Type: ref}}
	item.Methods = []*ir.ExportFn{times}
	items := &ir.Value{Name: "items", Reload: true, Type: ir.TypeRef{Kind: types.List, Elem: &ir.TypeRef{Kind: types.Record, Named: item}, KeyedBy: &ir.KeyField{Name: "price"}}}
	emit := &ir.Emit{Target: target, Mode: ir.ModeData, GoPackage: "shop", Namespace: "sov::shop"}
	return &ir.Package{Name: "shop", Types: []ir.Type{item}, Values: []*ir.Value{items}, Emits: []*ir.Emit{emit}}, item
}

// The Go name plan of a data-mode emit: what its loaders, snapshot and translated fns are called (CODEGEN.md §3.3, §5.9–§5.11).
func ExamplePlanGoNames() {
	p, item := shop(ir.TargetGo)
	pl := ir.PlanGoNames(p, p.Emits[0])
	v, snap := p.Values[0], pl.Snapshot()
	fmt.Print(pl.SchemaName(v), " ", pl.LoaderName(v), " ", pl.LoadFunc(v), " ", snap.Type, " ", snap.Load, " ", snap.Store, " ", snap.Var, " ", pl.DataLocal("name"), " ")
	fmt.Println(pl.Decoded(item), pl.NeedsWalk(item), len(pl.Holders(item)), pl.DecodeFunc(item), pl.ResolveFunc(item), len(pl.Problems()))
	// Output:
	// ItemsSchema LoadItems loadItems ShopSnapshot LoadShopSnapshot ShopStore Store name true true 1 decodeItem resolveItem 0
}

// A translated method's Go names: its public method, its pure function, its test, its locals and vector fields, `int` escaped (CODEGEN.md §3.4, §5.10).
func ExampleGoNamePlan_Pure() {
	p, item := shop(ir.TargetGo)
	pure := ir.PlanGoNames(p, p.Emits[0]).Pure(item.Methods[0])
	fmt.Println(pure.Public, pure.Pure, pure.Test, pure.Locals["int"], pure.Fields, pure.TestLocal)
	// Output: Times itemTimes TestItemTimesConformance int_ [price int_] t
}

// The C++ name plan of a data-mode emit (CODEGEN.md §3.3, §5.8, §7.2).
func ExamplePlanCppNames() {
	p, item := shop(ir.TargetCpp)
	pl := ir.PlanCppNames(p, p.Emits[0])
	getter, resolved := pl.FieldGetter(item.Fields[1])
	fmt.Println(pl.TypeName(item), pl.ContainerName(p.Values[0]), pl.SchemaName(p.Values[0]), pl.Upper(), len(pl.Problems()))
	fmt.Println(getter, resolved, pl.Member("next"), pl.Resolves(item.Fields[1].Type, item), pl.FnName(item.Methods[0]), pl.PureName("Item", item.Methods[0]))
	// Output:
	// Item Items kItemsSchema Shop 0
	// GetNextKey GetNext next_ true Times Item_times
}

// C++ enumerators, constants and kind members are verbatim, `_` added to a reserved word; a case's class and accessor are UpperCamel (CODEGEN.md §3.3, §3.4).
func ExampleCppNamePlan_Enumerator() {
	p, _ := shop(ir.TargetCpp)
	pl := ir.PlanCppNames(p, p.Emits[0])
	shape := &ir.Variant{Name: "Shape", Cases: []*ir.Case{{Name: "big_dot"}}}
	fmt.Println(pl.Enumerator(&ir.EnumMember{Name: "class"}), pl.ConstName(&ir.Const{Name: "MAX"}), pl.KindName(shape), pl.KindMember(shape.Cases[0]),
		pl.CaseName(shape, shape.Cases[0]), pl.AsName(shape.Cases[0]))
	// Output: class_ MAX ShapeKind big_dot ShapeBigDot AsBigDot
}

// A ref into another package's value names that value's accessor as its owner's plan does: Get<V>, or the value's @cpp(name:) (CODEGEN.md §3.3, §5.9).
func ExampleCppNamePlan_OwnerAccessor() {
	p, _ := shop(ir.TargetCpp)
	pl := ir.PlanCppNames(p, p.Emits[0])
	plain := &ir.RefTarget{Pkg: "lib", Value: "items"}
	named := &ir.RefTarget{Pkg: "lib", Value: "monsters", Cpp: ir.NameOptions{Name: "AllMonsters"}}
	fmt.Println(pl.OwnerAccessor(plain), pl.OwnerAccessor(named))
	// Output: GetItems AllMonsters
}
