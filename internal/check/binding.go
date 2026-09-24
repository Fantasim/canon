package check

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// binding holds the type parameters of one built-in call as they are bound (TYPES.md §12.2).
type binding struct {
	vars  map[*tvar]types.Type
	order []*tvar
	src   map[*tvar]syntax.Expr // the argument or receiver that bound each parameter
}

func newBinding() *binding {
	return &binding{vars: map[*tvar]types.Type{}, src: map[*tvar]syntax.Expr{}}
}

// from records e as the source of the parameters bound since the first before of order.
func (b *binding) from(before int, e syntax.Expr) {
	for _, v := range b.order[before:] {
		b.src[v] = e
	}
}

// shown is a pattern as a message prints it: bound, Seq(T) as [T], a free parameter as `_`.
func (b *binding) shown(pat types.Type) types.Type {
	if b == nil {
		b = &binding{}
	}
	s := &binding{vars: maps.Clone(b.vars), src: b.src}
	if s.vars == nil {
		s.vars = map[*tvar]types.Type{}
	}
	s.poison(pat)
	for v, t := range s.vars { //canon:unordered rewrites values only
		if t.Kind() == types.Error {
			if _, bound := b.vars[v]; !bound {
				s.vars[v] = types.AnyType
			}
		}
	}
	return concrete(s.subst(pat))
}

// bind sets v to t with its refinements dropped (refs stay refs); false when v is bound to
// another type.
func (b *binding) bind(v *tvar, t types.Type) bool {
	t = dropRefinements(t)
	if old, ok := b.vars[v]; ok {
		if j, joined := types.Join(old, t); joined && types.Identical(j, old) {
			return true
		}
		return types.Identical(old, t)
	}
	b.vars[v] = t
	b.order = append(b.order, v)
	return true
}

// dropRefinements removes the aliases and refinements of t and of its components.
func dropRefinements(t types.Type) types.Type {
	switch x := t.Base().(type) {
	case *types.ListType:
		return &types.ListType{Elem: dropRefinements(x.Elem), KeyedBy: x.KeyedBy}
	case *types.OptionalType:
		return &types.OptionalType{Elem: dropRefinements(x.Elem)}
	case *types.MapType:
		return &types.MapType{Key: dropRefinements(x.Key), Value: dropRefinements(x.Value)}
	}
	return t.Base()
}

// unify matches a pattern against a type, binding the pattern's parameters; false when they
// cannot match.
func (b *binding) unify(pat, t types.Type) bool {
	if t.Kind() == types.Error {
		b.poison(pat)
		return true
	}
	switch p := pat.(type) {
	case *tvar:
		if bound, ok := b.vars[p]; ok {
			return b.unify(bound, t) || types.Identical(bound, t)
		}
		return b.bind(p, t)
	case *seqOf:
		elem, ok := seqElem(t)
		return ok && b.unify(p.elem, elem)
	case *types.ListType:
		l, ok := t.Base().(*types.ListType)
		return ok && b.unify(p.Elem, l.Elem)
	case *types.OptionalType:
		if o, ok := t.Base().(*types.OptionalType); ok {
			return b.unify(p.Elem, o.Elem)
		}
		return b.unify(p.Elem, t)
	case *types.MapType:
		m, ok := t.Base().(*types.MapType)
		return ok && b.unify(p.Key, m.Key) && b.unify(p.Value, m.Value)
	case *types.PairType:
		q, ok := t.Base().(*types.PairType)
		return ok && b.unify(p.A, q.A) && b.unify(p.B, q.B)
	case *types.FuncType:
		return b.unifyFunc(p, t)
	}
	return b.free(pat) || types.Assignable(t, pat)
}

func (b *binding) unifyFunc(p *types.FuncType, t types.Type) bool {
	f, ok := t.Base().(*types.FuncType)
	if !ok || len(f.Params) != len(p.Params) {
		return false
	}
	for i := range p.Params {
		if !b.unify(p.Params[i], f.Params[i]) {
			return false
		}
	}
	return b.unify(p.Result, f.Result)
}

// seqElem is the element of a list, a keyed list or a table.
func seqElem(t types.Type) (types.Type, bool) {
	switch x := t.Base().(type) {
	case *types.ListType:
		return x.Elem, true
	case *types.TableType:
		return x.Elem, true
	}
	return nil, false
}

// poison binds the free parameters of pat to the error type, which silences cascades (TYPES.md §1).
func (b *binding) poison(pat types.Type) {
	switch p := pat.(type) {
	case *tvar:
		if _, ok := b.vars[p]; !ok {
			b.vars[p] = types.ErrorType
			b.order = append(b.order, p)
		}
	case *seqOf:
		b.poison(p.elem)
	case *types.ListType:
		b.poison(p.Elem)
	case *types.OptionalType:
		b.poison(p.Elem)
	case *types.MapType:
		b.poison(p.Key)
		b.poison(p.Value)
	case *types.PairType:
		b.poison(p.A)
		b.poison(p.B)
	case *types.FuncType:
		for _, q := range p.Params {
			b.poison(q)
		}
		b.poison(p.Result)
	}
}

// subst replaces the bound parameters of a pattern; unbound ones stay.
func (b *binding) subst(pat types.Type) types.Type {
	switch p := pat.(type) {
	case *tvar:
		if t, ok := b.vars[p]; ok {
			return t
		}
		return p
	case *seqOf:
		return &seqOf{elem: b.subst(p.elem)}
	case *types.ListType:
		return &types.ListType{Elem: b.subst(p.Elem)}
	case *types.OptionalType:
		elem := b.subst(p.Elem)
		if elem.Base().Kind() == types.Optional {
			return elem
		}
		return &types.OptionalType{Elem: elem}
	case *types.MapType:
		return &types.MapType{Key: b.subst(p.Key), Value: b.subst(p.Value)}
	case *types.PairType:
		return &types.PairType{A: b.subst(p.A), B: b.subst(p.B)}
	case *types.FuncType:
		f := &types.FuncType{Result: b.subst(p.Result)}
		for _, q := range p.Params {
			f.Params = append(f.Params, b.subst(q))
		}
		return f
	}
	return pat
}

// free reports a pattern that still holds an unbound parameter.
func (b *binding) free(pat types.Type) bool {
	switch p := b.subst(pat).(type) {
	case *tvar:
		return true
	case *seqOf:
		return b.free(p.elem)
	case *types.ListType:
		return b.free(p.Elem)
	case *types.OptionalType:
		return b.free(p.Elem)
	case *types.MapType:
		return b.free(p.Key) || b.free(p.Value)
	case *types.PairType:
		return b.free(p.A) || b.free(p.B)
	case *types.FuncType:
		return slices.ContainsFunc(p.Params, b.free) || b.free(p.Result)
	}
	return false
}

// typeArgs are the bound parameters in the order they were bound (Callee.TypeArgs).
func (b *binding) typeArgs() []types.Type {
	var out []types.Type
	for _, v := range b.order {
		if !v.hidden {
			out = append(out, b.vars[v])
		}
	}
	return out
}

// concrete is a bound signature type as Info records it: Seq(T) is written [T].
func concrete(t types.Type) types.Type {
	switch x := t.(type) {
	case *seqOf:
		return &types.ListType{Elem: concrete(x.elem)}
	case *types.FuncType:
		f := &types.FuncType{Result: concrete(x.Result)}
		for _, p := range x.Params {
			f.Params = append(f.Params, concrete(p))
		}
		return f
	}
	return t
}

// satisfies reports that a bound type meets a constraint (STDLIB.md §1.1).
func satisfies(t types.Type, k constraint) bool {
	if t.Kind() == types.Error {
		return true
	}
	switch k {
	case consNum:
		return t.Base().Kind() == types.Int || t.Base().Kind() == types.Float
	case consNumD:
		return isNumD(t)
	case consOrd:
		return orderable(t)
	case consEq:
		return t.Base().Kind() != types.Func
	case consKey:
		return mapKey(t)
	case consString:
		return t.Base().Kind() == types.String
	default:
	}
	return true
}
