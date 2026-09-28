package views

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// studioSections are `units`, `widgets` and, in the studio package's model, `studio`
// (VIEWMODEL.md 12.9): the units and widgets this package's controls name, a default widget
// that applies included.
func (b *builder) studioSections() {
	r := collect(b.m)
	all := b.studioUnits()
	//canon:unordered each unit written by name into a map
	for name := range r.units {
		if u, ok := all[name]; ok {
			b.m.Units[name] = u
		}
	}
	widgets := b.studioWidgets()
	//canon:unordered each widget written by name into a map
	for name := range r.widgets {
		if w, ok := widgets[name]; ok {
			b.m.Widgets[name] = w
		}
	}
	if b.in.Studio != "" && b.pkg.Path == b.in.Studio {
		b.m.Studio = &vm.Studio{
			Icons: memberNames(b.studioEnum(syntax.StudioIcon)), Tones: memberNames(b.studioEnum(syntax.StudioTone)),
			Units: all, Widgets: widgets,
		}
		if menus := b.studioEnum(syntax.StudioMenu); menus != nil {
			b.m.Studio.Menus = menus.String() // omitted without a sound Menu enum (12.9)
		}
	}
}

// studioUnits are the entries of the studio package's `units` table, by name.
func (b *builder) studioUnits() map[string]vm.Unit {
	out := map[string]vm.Unit{}
	t, ok := b.colls.Let(b.in.Studio, syntax.StudioUnits)
	table, isTable := t.(*value.Table)
	if b.in.Studio == "" || !ok || !isTable {
		return out
	}
	for _, e := range table.Entries {
		if e.Ident != nil {
			out[e.Ident.Key.Text()] = b.unit(e)
		}
	}
	return out
}

// unit is one entry of the units table: its suffix (the text reference `units.<u>.suffix`,
// absent when empty), scale, thousands and decimals.
func (b *builder) unit(e *value.Record) vm.Unit {
	u := vm.Unit{Scale: vm.Int(1)}
	for i, f := range encode.FieldsOf(e.T) {
		if i >= len(e.Fields) {
			break
		}
		switch v := e.Fields[i]; f.Name {
		case unitSuffix:
			if s, ok := v.(*value.Str); ok {
				u.Suffix = b.texts.Text(b.in.Studio, s.V, syntax.StudioUnits, e.Ident.Key.Text(), unitSuffix)
			}
		case unitScale:
			u.Scale = vm.Number{Text: string(encode.Value(v))}
		case unitThousands:
			yes, ok := v.(*value.Bool)
			u.Thousands = ok && yes.V
		case unitDecimals:
			if n, ok := v.(*value.Int); ok {
				u.Decimals = int(n.V)
			}
		}
	}
	return u
}

// studioWidgets are the widgets of the studio package, by name: their parameter types, whether
// they are default, and their doc comment as a plain string (12.9).
func (b *builder) studioWidgets() map[string]vm.Widget {
	out := map[string]vm.Widget{}
	sp := shape.Package(b.in.Program, b.in.Studio)
	if sp == nil {
		return out
	}
	for _, o := range sp.Decls {
		d, ok := o.Decl().(*syntax.WidgetDecl)
		if !ok || o.Kind() != check.ObjWidget || b.broken(o) || len(d.Params) == 0 {
			continue
		}
		w := vm.Widget{Value: b.defs.Expr(nil, o.Type()), Default: d.Default.Valid()}
		if len(d.Params) > 1 {
			if t := b.in.Program.Info.TypeExprs[d.Params[1].Type]; t != nil {
				sib := b.defs.Expr(nil, t)
				w.Siblings = &sib
			}
		}
		if d.Doc != nil {
			w.Help = &d.Doc.Text
		}
		out[o.Name()] = w
	}
	return out
}

// studioEnum is the studio package's enum name, nil when it declares no sound one (J4).
func (b *builder) studioEnum(name string) *types.EnumType {
	sp := shape.Package(b.in.Program, b.in.Studio)
	if sp == nil {
		return nil
	}
	for _, o := range sp.Decls {
		if o.Name() == name && o.Kind() == check.ObjTypeName && !b.broken(o) {
			e, _ := shape.Unalias(o.Type()).(*types.EnumType)
			return e
		}
	}
	return nil
}

// memberNames are an enum's member names in declaration order, none for no enum.
func memberNames(e *types.EnumType) []string {
	out := []string{}
	if e == nil {
		return out
	}
	for _, m := range e.Members {
		out = append(out, m.Name)
	}
	return out
}

// assets is the `assets` section (12.9): each asset root the package's types use, by display
// path, with its directory relative to the project directory.
func (b *builder) assets() {
	//canon:unordered each root written by name into a map
	for root := range collect(b.m).roots {
		if dir, ok := b.roots.Dir(root); ok {
			b.m.Assets[root] = vm.Asset{Dir: dir}
		}
	}
}
