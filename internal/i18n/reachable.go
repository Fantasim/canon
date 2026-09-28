package i18n

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// reachableTypes is pkg's public record, variant and enum types and every such type of pkg
// reachable from a public type or value, local ones included (I18N.md K1), a broken one left
// out (VIEWMODEL.md J4). Order is arbitrary: Build sorts its catalogue entries by key (K10).
func reachableTypes(pkg *check.Package, info *check.Info) []types.Type {
	objOf := typeObjects(pkg)
	seen := map[types.Type]bool{}
	var order, queue []types.Type
	add := func(t types.Type) {
		if t == nil || seen[t] || info.Broken[objOf[t]] {
			return
		}
		seen[t] = true
		order = append(order, t)
		queue = append(queue, t)
	}
	addRoots(pkg, info, add)
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		walkFields(t, pkg.Path, add)
	}
	return order
}

// typeObjects maps each of pkg's type-name objects to itself, by its resolved type.
func typeObjects(pkg *check.Package) map[types.Type]check.Object {
	objOf := map[types.Type]check.Object{}
	for _, o := range pkg.Decls {
		if o.Kind() == check.ObjTypeName {
			objOf[o.Type()] = o
		}
	}
	return objOf
}

// addRoots seeds add with pkg's own public types and the types reachable from its public lets,
// a broken declaration skipped (VIEWMODEL.md J4).
func addRoots(pkg *check.Package, info *check.Info, add func(types.Type)) {
	for _, o := range pkg.Decls {
		if isLocalDecl(o.Decl()) || info.Broken[o] {
			continue
		}
		if o.Kind() == check.ObjTypeName {
			if t := o.Type(); named(t) {
				add(t)
			}
		}
		if o.Kind() == check.ObjLet {
			walkType(o.Type(), pkg.Path, add)
		}
	}
}

// named reports a record, variant or enum type (I18N.md K1's "type").
func named(t types.Type) bool {
	switch t.(type) {
	case *types.RecordType, *types.VariantType, *types.EnumType:
		return true
	default:
		return false
	}
}

// walkFields adds every record, variant and enum of pkg one field away from t.
func walkFields(t types.Type, pkg string, add func(types.Type)) {
	switch x := t.Base().(type) {
	case *types.RecordType:
		for _, f := range x.Fields {
			walkType(f.Type, pkg, add)
		}
	case *types.VariantType:
		for _, c := range x.Cases {
			for _, f := range c.Fields {
				walkType(f.Type, pkg, add)
			}
		}
	}
}

// walkType descends t's composite layers, adding a record, variant or enum of pkg it finds.
func walkType(t types.Type, pkg string, add func(types.Type)) {
	if t == nil {
		return
	}
	switch x := t.Base().(type) {
	case *types.RecordType:
		addSamePkg(x, x.Pkg, pkg, add)
	case *types.VariantType:
		addSamePkg(x, x.Pkg, pkg, add)
	case *types.EnumType:
		addSamePkg(x, x.Pkg, pkg, add)
	case *types.ListType:
		walkType(x.Elem, pkg, add)
	case *types.MapType:
		walkType(x.Key, pkg, add)
		walkType(x.Value, pkg, add)
	case *types.DepMapType:
		walkType(x.Value, pkg, add)
	case *types.TableType:
		walkType(x.Elem, pkg, add)
	case *types.OptionalType:
		walkType(x.Elem, pkg, add)
	case *types.LitUnionType:
		walkType(x.Of, pkg, add)
	case *types.PairType:
		walkType(x.A, pkg, add)
		walkType(x.B, pkg, add)
	case *types.AppliedRecord:
		addSamePkg(x.Rec, x.Rec.Pkg, pkg, add)
	case *types.TypeAppType:
		walkTypeFunc(x.Fn, pkg, add)
	case *types.DepUnionType:
		walkTypeFunc(x.Fn, pkg, add)
	case *types.RefType:
		if x.Target != nil {
			walkType(x.Target.Elem, pkg, add)
		}
	}
}

// walkTypeFunc descends a dependent type function's plain body, or every match arm's result
// (I18N.md K1: `type Target(g) = match g { kill => KillP … }` reaches KillP).
func walkTypeFunc(fn *types.TypeFunc, pkg string, add func(types.Type)) {
	if fn == nil {
		return
	}
	walkType(fn.Body, pkg, add)
	for _, arm := range fn.Arms {
		walkType(arm.Result, pkg, add)
	}
}

func addSamePkg(t types.Type, typePkg, pkg string, add func(types.Type)) {
	if typePkg == pkg {
		add(t)
	}
}

// isLocalDecl reports a record, variant, enum or let declared `local` (I18N.md K1).
func isLocalDecl(d syntax.Node) bool {
	switch x := d.(type) {
	case *syntax.RecordDecl:
		return x.Mods != nil && x.Mods.Local.Valid()
	case *syntax.VariantDecl:
		return x.Mods != nil && x.Mods.Local.Valid()
	case *syntax.EnumDecl:
		return x.Mods != nil && x.Mods.Local.Valid()
	case *syntax.LetDecl:
		return x.Mods != nil && x.Mods.Local.Valid()
	default:
		return false
	}
}
