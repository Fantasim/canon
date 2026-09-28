package rules

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// viewOf is the view d declares in pkg, nil for none to check, and its target's finding (G4–G6).
func (c *checker) viewOf(pkg string, f *syntax.File, d *syntax.ViewDecl) (*view, *diag.Builder) {
	o := c.info.NameUses[d.Type]
	if o == nil || o.Kind() == check.ObjConst || o.Kind() == check.ObjFn || isError(o.Type()) {
		return nil, nil
	}
	var cs check.Object
	if d.Case != nil {
		if cs = c.info.NameUses[d.Case]; cs == nil {
			return nil, nil
		}
	}
	at, name := f.Span(d.Type), d.Type.Name
	if d.Case != nil {
		name += dot + d.Case.Name
	}
	v := newView(o, cs)
	switch {
	case v == nil && o.Kind() == check.ObjLet:
		return nil, diag.E1626.AtLet(at, name)
	case v == nil:
		return nil, diag.E1626.AtType(at, name)
	case v.owner != pkg:
		return nil, diag.E1603.At(at, v.full, v.owner)
	}
	if first, dup := c.first[v.key]; dup {
		return nil, diag.E1607.At(at, name, first)
	}
	c.first[v.key] = at
	v.c, v.pkg, v.file, v.decl, v.name = c, pkg, f, d, name
	return v, nil
}

// newView is the view of o, or of its case cs; nil for another target (VIEWMODEL.md §3.2).
func newView(o, cs check.Object) *view {
	v := &view{target: o, placed: map[string]source.Span{}, items: map[string]source.Span{}, hides: map[string]bool{}}
	for i := range v.ids {
		v.ids[i] = map[string]source.Span{}
	}
	if o.Kind() == check.ObjLet {
		if !definesTable(o.Type()) {
			return nil
		}
		v.kind, v.fields, v.key, v.owner, v.full = targetDefine, types.DefineType.Fields, o, o.Pkg(), o.Pkg()+dot+o.Name()
		return v
	}
	if o.Kind() != check.ObjTypeName {
		return nil
	}
	t := shape.Unalias(o.Type())
	if cs != nil {
		t = cs.Type()
	}
	switch x := t.(type) {
	case *types.RecordType:
		v.kind, v.fields, v.methods, v.owner = targetRecord, x.Fields, x.Methods, x.Pkg
	case *types.CaseType:
		v.kind, v.fields, v.methods, v.owner = targetCase, x.Fields, x.Methods, x.Variant.Pkg
	case *types.VariantType:
		v.kind, v.variant, v.owner = targetVariant, x, x.Pkg
	case *types.EnumType:
		v.kind, v.owner = targetEnum, x.Pkg
	default:
		return nil
	}
	v.key, v.full = t, t.String()
	return v
}

// definesTable reports the type of a `load.defines` table (VIEWMODEL.md §3.2).
func definesTable(t types.Type) bool {
	if t == nil {
		return false
	}
	tt, ok := t.Base().(*types.TableType)
	return ok && tt.Elem == types.DefineType
}

// broken reports a view holding an error, or on a broken target (TYPES.md §1, VIEWMODEL.md J4).
func (c *checker) broken(v *view, errs []source.Span) bool {
	if c.info.Broken[v.target] {
		return true
	}
	within := v.span(v.decl)
	if slices.ContainsFunc(errs, func(s source.Span) bool {
		return s.File == within.File && s.Start >= within.Start && s.Start < within.End
	}) {
		return true
	}
	bad := false
	syntax.Inspect(v.decl, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.BadExpr, *syntax.BadDecl:
			bad = true
		}
		if e, ok := n.(syntax.Expr); ok && isError(c.info.Types[e]) {
			bad = true
		}
		return !bad
	})
	return bad
}

func isError(t types.Type) bool { return t != nil && t.Kind() == types.Error }
