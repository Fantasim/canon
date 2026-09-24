package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// call is `f(args)` on the callee the checker resolved (TYPES.md §12.2, EVALUATION.md §2.2).
func (r *run) call(x *syntax.CallExpr) value.Value {
	callee := r.ev.info.Calls[x]
	if callee == nil {
		r.bug(x)
		return nil
	}
	switch callee.Kind {
	case check.CalleeFn:
		if !r.step(x.Fun) {
			return nil
		}
		return r.callUser(x, callee.Obj, nil)
	case check.CalleeMethod:
		return r.callMethod(x, callee.Obj)
	case check.CalleeBuiltin, check.CalleeConvert:
		return r.callBuiltin(x, callee)
	case check.CalleeLambda:
		fn := r.eval(x.Fun)
		args := r.positional(x)
		if fn == nil || args == nil {
			return nil
		}
		return r.invoke(fn, args, r.span(x))
	}
	r.bug(x)
	return nil
}

// callMethod is `x.m(args)`, `x?.m(args)`, or `m(args)` on self in a record body.
func (r *run) callMethod(x *syntax.CallExpr, obj check.Object) value.Value {
	if !r.step(x.Fun) {
		return nil
	}
	self := r.fr.self
	if s, ok := x.Fun.(*syntax.SelectorExpr); ok {
		if self = r.recv(s.X); !r.unwrap(self, s.Optional) {
			return nil
		}
		if sel := r.ev.info.Selections[s]; sel != nil && sel.Deref {
			if self = r.deref(self, s); self == nil {
				return nil
			}
		}
	}
	return r.callUser(x, obj, self)
}

// callUser evaluates the arguments of a user function or method, then invokes it.
func (r *run) callUser(x *syntax.CallExpr, obj check.Object, self value.Value) value.Value {
	d, ok := obj.Decl().(*syntax.FnDecl)
	if !ok {
		r.bug(x)
		return nil
	}
	args := make([]value.Value, len(d.Params))
	for i, a := range x.Args {
		j := i
		if a.Name != nil {
			j = paramIndex(d.Params, a.Name.Name)
		}
		v := r.eval(a.Value)
		if v == nil || j < 0 || j >= len(args) {
			r.bug(x)
			return nil
		}
		args[j] = v
	}
	return r.invokeFn(fnCall{obj: obj, self: self, args: args, site: r.span(x)})
}

func paramIndex(ps []*syntax.Param, name string) int {
	for i, p := range ps {
		if p.Name.Name == name {
			return i
		}
	}
	return -1
}

// positional evaluates the arguments of a call of a function value, all positional.
func (r *run) positional(x *syntax.CallExpr) []value.Value {
	args := make([]value.Value, len(x.Args))
	for i, a := range x.Args {
		if args[i] = r.eval(a.Value); args[i] == nil {
			return nil
		}
	}
	return args
}

// fnCall is one invocation of a user function or method; name overrides its frame's name.
type fnCall struct {
	obj  check.Object
	self value.Value
	args []value.Value
	site source.Span
	name string
}

// invokeFn runs a user function or method (EVALUATION.md §3.3, §12.1, TYPES.md §6.2).
func (r *run) invokeFn(c fnCall) value.Value {
	d, isFn := c.obj.Decl().(*syntax.FnDecl)
	ft, isFt := c.obj.Type().(*types.FuncType)
	file := c.obj.File()
	if !isFn || !isFt || len(ft.Params) != len(d.Params) || len(c.args) != len(d.Params) {
		r.bug(nil)
		return nil
	}
	if !r.enter(c.site) {
		return nil
	}
	for i, p := range d.Params {
		if c.args[i] != nil {
			c.args[i] = r.store(c.args[i], ft.Params[i], declSite(file, p.Name, p.Type), nil)
		}
	}
	name := c.name
	if name == "" {
		name = fnName(c.obj, c.self)
	}
	fr := &frame{vars: map[check.Object]value.Value{}, self: c.self, file: file, pkg: c.obj.Pkg(), fn: name, call: c.site, caller: r.fr}
	saved := r.fr
	r.fr = fr
	r.depth++
	if r.params(d, ft, c.args) {
		r.block(d.Body)
	}
	r.fr = saved
	r.depth--
	if fr.ret == nil {
		r.bug(c.obj.Decl())
		return nil
	}
	return r.store(fr.ret, ft.Result, declSite(file, nil, d.Result), nil)
}

// enter checks the call depth (E4402) and spends the invocation's step.
func (r *run) enter(site source.Span) bool {
	if r.depth >= maxDepth {
		r.fail(diag.E4402.At(site))
		return false
	}
	return r.spend(1, func() source.Span { return site })
}

// params binds the parameters in the callee's frame, a missing one to its default.
func (r *run) params(d *syntax.FnDecl, ft *types.FuncType, args []value.Value) bool {
	for i, p := range d.Params {
		v := args[i]
		if v == nil && p.Default != nil {
			v = r.store(r.eval(p.Default), ft.Params[i], declSite(r.fr.file, p.Name, p.Type), nil)
		}
		obj := r.ev.info.Defs[p.Name]
		if v == nil || obj == nil {
			r.bug(p)
			return false
		}
		r.fr.vars[obj] = v
	}
	return true
}

// declSite is the site of a declaration `name: Type`, or of a written type alone.
func declSite(f *syntax.File, name *syntax.Ident, t syntax.Type) site {
	if f == nil || t == nil {
		return site{}
	}
	sp := f.Span(t)
	if name != nil {
		sp = f.Span(name).Cover(sp)
	}
	return site{decl: sp, has: true}
}

// fnName names a frame: the function, or `Record.method` for a method.
func fnName(obj check.Object, self value.Value) string {
	if rec, ok := self.(*value.Record); ok && obj.Kind() == check.ObjMethod {
		if rt := recordOf(rec.T); rt != nil {
			return rt.Name + dot + obj.Name()
		}
		if ct, isCase := rec.T.Base().(*types.CaseType); isCase {
			return ct.Variant.Name + dot + obj.Name()
		}
	}
	return obj.Name()
}

// callBuiltin calls a built-in function, conversion or method through the standard library.
func (r *run) callBuiltin(x *syntax.CallExpr, callee *check.Callee) value.Value {
	if !r.step(x.Fun) {
		return nil
	}
	s, isMethod := x.Fun.(*syntax.SelectorExpr)
	if !isMethod {
		if callee.Builtin == failStmt || callee.Builtin == warnStmt {
			return r.failWarn(x, callee.Builtin == warnStmt)
		}
		return r.runStd(std.Free, x, callee, nil)
	}
	recv := r.recv(s.X)
	if !r.unwrap(recv, s.Optional) {
		return nil
	}
	if sel := r.ev.info.Selections[s]; sel != nil && sel.Deref {
		if recv = r.deref(recv, s); recv == nil {
			return nil
		}
	}
	return r.runStd(std.Method, x, callee, recv)
}

// runStd evaluates a built-in's arguments, in the order written, into parameter order, and
// runs it at the call's site.
func (r *run) runStd(fn func(std.Host, *std.Call) (value.Value, bool), x *syntax.CallExpr, callee *check.Callee, recv value.Value) value.Value {
	var args []value.Value
	for i, a := range x.Args {
		j := i
		if a.Name != nil {
			j = std.ParamIndex(recv, callee.Builtin, a.Name.Name)
		}
		v := r.eval(a.Value)
		if v == nil || j < 0 {
			r.bug(x)
			return nil
		}
		for len(args) <= j {
			args = append(args, nil)
		}
		args[j] = v
	}
	r.site = r.span(x)
	c := &std.Call{
		Name: callee.Builtin, Overload: callee.Overload, Recv: recv, Args: args,
		Result: r.typeOf(x), Prov: r.prov(x, value.ProvComputed),
	}
	return r.std(fn(r.host(), c))
}
