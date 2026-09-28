package control

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// Index holds what the views of a program say about each field, method, member and case, on
// whichever of their lines, and the studio's default widgets (VIEWMODEL.md G22).
type Index struct {
	info     *check.Info
	fields   map[*types.Field]Props
	items    map[item]Props
	named    map[item]bool // methods a view names
	viewed   map[types.Type]bool
	defaults []defaultWidget
}

// item is a method, member or case of the record, case, enum or variant owner.
type item struct {
	owner types.Type
	name  string
}

// NewIndex reads every view of prog's source files, in package and file order (the first line
// giving a property wins, E1613 being the checker's), and project.studio's default widgets.
func NewIndex(prog *check.Program, studio string) *Index {
	x := &Index{
		info:   prog.Info,
		fields: map[*types.Field]Props{},
		items:  map[item]Props{},
		named:  map[item]bool{},
		viewed: map[types.Type]bool{},
	}
	for _, p := range prog.Packages {
		x.files(p.Path, p.Files)
		if studio != "" && p.Path == studio {
			x.defaults = defaultWidgets(prog.Info, p)
		}
	}
	return x
}

// files reads the views of pkg's source files among files.
func (x *Index) files(pkg string, files []*syntax.File) {
	for _, f := range files {
		if f.FileKind != syntax.FileSource {
			continue
		}
		for _, d := range f.Decls {
			if vd, ok := d.(*syntax.ViewDecl); ok {
				x.view(pkg, vd)
			}
		}
	}
}

// Field is the properties the views give f.
func (x *Index) Field(f *types.Field) Props { return x.fields[f] }

// Item is the properties the views give the method, member or case name of owner.
func (x *Index) Item(owner types.Type, name string) Props { return x.items[item{owner, name}] }

// Named reports a method of the record or case owner that a view names.
func (x *Index) Named(owner types.Type, method string) bool { return x.named[item{owner, method}] }

// view reads the member items of a record, case, variant or enum view of pkg, in groups or not;
// a view the checker skips (E1603 in another package, E1607 a second one) gives nothing (J4).
func (x *Index) view(pkg string, d *syntax.ViewDecl) {
	t := x.target(d)
	if t == nil || x.viewed[t] || ownerPkg(t) != pkg {
		return
	}
	x.viewed[t] = true
	for _, it := range d.Items {
		switch it := it.(type) {
		case *syntax.ViewField:
			x.member(pkg, t, it)
		case *syntax.ViewGroup:
			x.group(pkg, t, it)
		}
	}
}

// group reads the member items of a group of the view of t.
func (x *Index) group(pkg string, t types.Type, g *syntax.ViewGroup) {
	for _, m := range g.Members {
		if f, ok := m.(*syntax.ViewField); ok {
			x.member(pkg, t, f)
		}
	}
}

// ownerPkg is the package declaring a view's target (VIEWMODEL.md G4).
func ownerPkg(t types.Type) string {
	switch x := t.(type) {
	case *types.RecordType:
		return x.Pkg
	case *types.VariantType:
		return x.Pkg
	case *types.EnumType:
		return x.Pkg
	case *types.CaseType:
		return x.Variant.Pkg
	}
	return ""
}

// target is the type a view describes, its case for `view V.c`; nil for none (a define
// table's view gives no property).
func (x *Index) target(d *syntax.ViewDecl) types.Type {
	o := x.info.NameUses[d.Type]
	if o == nil || o.Kind() != check.ObjTypeName || o.Type() == nil {
		return nil
	}
	if d.Case == nil {
		return shape.Unalias(o.Type())
	}
	if cs := x.info.NameUses[d.Case]; cs != nil && cs.Kind() == check.ObjCase {
		return cs.Type()
	}
	return nil
}

// member records the properties of a member item of pkg's view of t (VIEWMODEL.md G8, L18).
func (x *Index) member(pkg string, t types.Type, f *syntax.ViewField) {
	if f.Name == nil {
		return
	}
	o := x.info.NameUses[f.Name]
	if o == nil {
		return
	}
	switch o.Kind() {
	case check.ObjField:
		for _, fd := range fieldsNamed(t, f.Name.Name) {
			x.fields[fd] = merge(x.fields[fd], f.Props, pkg)
		}
	case check.ObjMethod:
		x.named[item{t, f.Name.Name}] = true
		x.items[item{t, f.Name.Name}] = merge(x.items[item{t, f.Name.Name}], f.Props, pkg)
	case check.ObjCase, check.ObjMember:
		x.items[item{t, f.Name.Name}] = merge(x.items[item{t, f.Name.Name}], f.Props, pkg)
	default: // a name of another kind is the checker's E1602
	}
}

// fieldsNamed is t's field name, else every field of that name of the cases of its inline
// variant fields, depth first (L18); none when those differ in type (E1628).
func fieldsNamed(t types.Type, name string) []*types.Field {
	fields := encode.FieldsOf(t)
	if i := slices.IndexFunc(fields, func(f *types.Field) bool { return f.Name == name }); i >= 0 {
		return fields[i : i+1]
	}
	var out []*types.Field
	inlineFields(fields, name, map[*types.VariantType]bool{}, &out)
	for _, f := range out {
		if !shape.SameType(f.Type, out[0].Type) {
			return nil
		}
	}
	return out
}

// inlineFields adds the fields named name of the cases of each @json(inline) variant field of
// fields, in case order, nested ones depth first; seen stops a variant inlining itself.
func inlineFields(fields []*types.Field, name string, seen map[*types.VariantType]bool, out *[]*types.Field) {
	for _, f := range fields {
		v, ok := f.Type.Base().(*types.VariantType)
		if !ok || !f.Inline || seen[v] {
			continue
		}
		seen[v] = true
		for _, c := range v.Cases {
			if i := slices.IndexFunc(c.Fields, func(cf *types.Field) bool { return cf.Name == name }); i >= 0 {
				*out = append(*out, c.Fields[i])
			}
			inlineFields(c.Fields, name, seen, out)
		}
	}
}
