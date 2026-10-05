package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// typeMember is `E.members` or `E.typeName`, E an enum or the one `F(a…)` selects (STDLIB.md §3, TYPES.md §4.3).
func (r *run) typeMember(obj check.Object, at syntax.Expr) value.Value {
	s, ok := at.(*syntax.SelectorExpr)
	if !ok {
		r.bug(at)
		return nil
	}
	e := r.enumOf(s.X)
	if e == nil {
		return nil
	}
	switch obj.Name() {
	case memberMembers:
		return r.enumMembers(e, at)
	case memberTypeName:
		return &value.Str{V: e.Name, T: types.StringType, P: r.prov(at, value.ProvLiteral)}
	}
	r.bug(at)
	return nil
}

// enumOf is the enum x names, or the arm of `F(a…)` its arguments select (DEP-02); nil once the run stopped.
func (r *run) enumOf(x syntax.Expr) *types.EnumType {
	var t types.Type
	if call, isCall := syntax.Unparen(x).(*syntax.CallExpr); isCall {
		t = r.applied(call)
	} else if obj := r.typeName(x); obj != nil {
		t = obj.Type()
	}
	if t == nil {
		r.bug(x)
		return nil
	}
	e, ok := t.Base().(*types.EnumType)
	if !ok {
		r.bug(x)
		return nil
	}
	return e
}

// typeName is the type name x writes, `E` or `pkg.E`; nil for another expression.
func (r *run) typeName(x syntax.Expr) check.Object {
	var obj check.Object
	switch n := syntax.Unparen(x).(type) {
	case *syntax.IdentExpr:
		obj = r.ev.info.Uses[n]
	case *syntax.SelectorExpr:
		obj = r.ev.info.NameUses[n.Name]
	}
	if obj == nil || obj.Kind() != check.ObjTypeName {
		return nil
	}
	return obj
}

// applied is the type `F(a…)` computes in value position, a step and its arguments' (EVALUATION.md §12.1).
func (r *run) applied(call *syntax.CallExpr) types.Type {
	obj := r.typeName(call.Fun)
	var fn *types.TypeFunc
	if obj != nil {
		if du, ok := obj.Type().Base().(*types.DepUnionType); ok {
			fn = du.Fn
		}
	}
	if fn == nil || len(call.Args) != len(fn.Params) {
		r.bug(call)
		return nil
	}
	if !r.step(call) {
		return nil
	}
	args := make(map[*types.Param]value.Value, len(fn.Params))
	for i, a := range call.Args {
		v := r.eval(a.Value)
		if v == nil {
			return nil
		}
		args[fn.Params[i]] = v
	}
	return r.typeArm(fn, args, call)
}

// typeArm is fn's body, or the arm its scrutinee selects for 1 + the path's length (TYPES.md §11.6).
func (r *run) typeArm(fn *types.TypeFunc, args map[*types.Param]value.Value, at syntax.Node) types.Type {
	s := fn.Scrutinee
	if s == nil {
		return fn.Body
	}
	if !r.spend(1+len(s.Path), func() source.Span { return r.span(at) }) {
		return nil
	}
	v := r.follow(args[s.Param], s.Path, at)
	if v == nil {
		return nil
	}
	i, ok := value.ArmIndex(v)
	if a := fn.Arm(i); ok && a != nil {
		return a.Result
	}
	r.bug(at)
	return nil
}

// enumMembers is e's members, retired ones included, a step each, typed as at (STDLIB.md §3).
func (r *run) enumMembers(e *types.EnumType, at syntax.Expr) value.Value {
	if !r.spend(len(e.Members), func() source.Span { return r.span(at) }) {
		return nil
	}
	p := r.prov(at, value.ProvLiteral)
	out := &value.List{T: r.typeOf(at), Elems: make([]value.Value, len(e.Members)), P: p}
	for i := range e.Members {
		out.Elems[i] = &value.Member{Enum: e, Index: i, P: p}
	}
	return out
}
