package typedef

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// declared is a type definition to write: a record, variant or enum, or a type function.
type declared struct {
	name string
	t    types.Type // nil for a type function
	fn   *types.TypeFunc
}

// reachable are the package's types that are public, or reachable from a public type or value,
// in the order met (VIEWMODEL.md 12.3, I18N.md K1); a broken declaration is none of them (J4).
func (s *Types) reachable(p *check.Package) []declared {
	var roots []types.Type
	for _, o := range p.Decls {
		if o.Kind() != check.ObjTypeName && o.Kind() != check.ObjLet || s.broken(o) {
			continue
		}
		if k := key(o.Type()); o.Kind() == check.ObjTypeName && declares(o, k) {
			s.objects[k] = o
		}
		if !local(o.Decl()) {
			roots = append(roots, o.Type())
		}
	}
	w := &walker{s: s, seen: map[any]bool{}}
	for _, t := range roots {
		w.walk(t)
	}
	return w.out
}

// broken reports a declaration left out of the view model (TYPES.md §1, J4).
func (s *Types) broken(o check.Object) bool {
	return s.in.Program.Info.Broken[o] || o.Type() == nil || o.Type().Kind() == types.Error
}

// key is what a declared type is known by: its record, variant or enum, or its type function.
func key(t types.Type) any {
	if d, ok := t.Base().(*types.DepUnionType); ok {
		return d.Fn
	}
	return shape.Unalias(t)
}

// declares reports that o is the declaration of k, not an alias of it.
func declares(o check.Object, k any) bool {
	switch x := k.(type) {
	case *types.RecordType:
		return x.Decl == o.Decl()
	case *types.VariantType:
		return x.Decl == o.Decl()
	case *types.EnumType:
		return x.Decl == o.Decl()
	case *types.TypeFunc:
		return x.Decl == o.Decl()
	}
	return false
}

// local reports a declaration written `local`.
func local(d syntax.Node) bool {
	var m *syntax.Modifiers
	switch x := d.(type) {
	case *syntax.RecordDecl:
		m = x.Mods
	case *syntax.VariantDecl:
		m = x.Mods
	case *syntax.EnumDecl:
		m = x.Mods
	case *syntax.TypeDecl:
		m = x.Mods
	case *syntax.LetDecl:
		m = x.Mods
	}
	return m != nil && m.Local.Valid()
}

// walker visits the types reachable from the roots, keeping the package's own declarations.
type walker struct {
	s    *Types
	seen map[any]bool
	out  []declared
}

// walk visits t and every type it holds or names.
func (w *walker) walk(t types.Type) {
	encode.Walk(t, func(x types.Type) bool {
		switch y := x.Base().(type) {
		case *types.RecordType:
			w.record(y)
		case *types.AppliedRecord:
			w.record(y.Rec)
		case *types.VariantType:
			w.variant(y)
		case *types.CaseType:
			w.variant(y.Variant)
		case *types.EnumType:
			w.own(y, declared{name: y.String(), t: y})
		case *types.RefType:
			w.walk(y.Target.Elem)
		case *types.DepMapType:
			w.walk(y.Coll.Elem)
		case *types.TypeAppType:
			w.fn(y.Fn)
		case *types.DepUnionType:
			w.fn(y.Fn)
		}
		return true
	})
}

// own records d the first time the package's declaration k is met; false when k is another
// package's, broken, or met already.
func (w *walker) own(k any, d declared) bool {
	if w.seen[k] || w.s.objects[k] == nil {
		return false
	}
	w.seen[k] = true
	if d.name != "" {
		w.out = append(w.out, d)
	}
	return true
}

func (w *walker) record(r *types.RecordType) {
	if !w.own(r, declared{name: r.String(), t: r}) {
		return
	}
	for _, p := range r.Params {
		w.walk(p.Type)
	}
	w.body(r, r.Fields, r.Methods)
}

func (w *walker) variant(v *types.VariantType) {
	if !w.own(v, declared{name: v.String(), t: v}) {
		return
	}
	for _, c := range v.Cases {
		w.body(c, c.Fields, c.Methods)
	}
}

// body visits the fields of a record or case and the methods a view names.
func (w *walker) body(owner types.Type, fields []*types.Field, methods []*types.Method) {
	for _, f := range fields {
		w.walk(f.Type)
	}
	for _, m := range w.s.named(owner, methods) {
		w.walk(m.Type.Result)
	}
}

// fn visits a type function: kept as a definition when its body is a `match` (J11).
func (w *walker) fn(fn *types.TypeFunc) {
	d := declared{}
	if control.Matches(fn) {
		d = declared{name: encode.FuncName(fn), fn: fn}
	}
	if !w.own(fn, d) {
		return
	}
	for _, p := range fn.Params {
		w.walk(p.Type)
	}
	for _, a := range fn.Arms {
		w.walk(a.Result)
	}
	if fn.Body != nil {
		w.walk(fn.Body)
	}
}
