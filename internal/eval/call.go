package eval

import (
	"slices"

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
	if callee == nil && r.nonConstant() { // a call the checker left unresolved in a constant (DECISIONS 150, 210)
		return nil
	}
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
	if r.nonConstant() {
		return nil
	}
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
	if r.nonConstant() {
		return nil
	}
	d, isFn := c.obj.Decl().(*syntax.FnDecl)
	ft, isFt := c.obj.Type().(*types.FuncType)
	if !isFn || !isFt || len(ft.Params) != len(d.Params) || len(c.args) != len(d.Params) {
		r.bug(nil)
		return nil
	}
	if !r.enter(c.site) {
		return nil
	}
	fr := r.calleeFrame(c)
	if fr.ts && !r.tsEntry(fr, d) || !r.storeArgs(fr, d, ft, c.args) {
		return nil
	}
	r.ev.record(c)
	saved, dep := r.fr, r.dep
	r.fr, r.dep = fr, nil // a body computes: its literals are not kept as written (TYPES.md §6.2)
	r.ev.depth++
	if r.params(d, ft, c.args) {
		r.block(d.Body)
	}
	r.fr, r.dep = saved, dep
	r.ev.depth--
	if fr.ret == nil {
		r.bug(c.obj.Decl())
		return nil
	}
	v := r.store(fr.ret, ft.Result, declSite(fr.file, nil, d.Result), nil)
	if r.fr.ts && !fr.ts {
		return r.tsRead(v) // CONFORMANCE.md §2.2: a precomputed or lookup result is read like a field
	}
	return v
}

// storeArgs takes each argument in turn: safe in TS mode, then its sized type and range (DECISIONS 311).
func (r *run) storeArgs(fr *frame, d *syntax.FnDecl, ft *types.FuncType, args []value.Value) bool {
	for i, p := range d.Params {
		if args[i] == nil {
			continue
		}
		if fr.ts && unsafeInt(args[i]) {
			r.tsFail()
			return false
		}
		args[i] = r.store(args[i], ft.Params[i], declSite(fr.file, p.Name, p.Type), nil)
	}
	return true
}

// calleeFrame is a call's frame, TS-mode code when the caller's is and fn is translated (CONFORMANCE.md §4).
func (r *run) calleeFrame(c fnCall) *frame {
	name := c.name
	if name == "" {
		name = fnName(c.obj, c.self)
	}
	fr := &frame{vars: map[check.Object]value.Value{}, self: c.self, file: c.obj.File(), pkg: c.obj.Pkg(), fn: name, call: c.site}
	fr.ts = r.fr.ts && runtimeInput(c.obj)
	r.noteCode(fr.file)
	return fr.under(r.fr)
}

// enter checks the call depth (within), then spends the invocation's step (EVALUATION.md §3.3, §12.1).
func (r *run) enter(site source.Span) bool {
	return r.within(site) && r.spend(1, func() source.Span { return site })
}

// within is E4402 at site once the live frames reach the limit; more frames omit implicit ones (DECISIONS 195, 197, 210).
func (r *run) within(site source.Span) bool {
	if r.ev.depth < maxDepth {
		r.noteDepth()
		return true
	}
	r.voidTrace()
	if r.ev.vec != nil {
		r.ev.cut(DepthLimit)
	}
	stack, _ := r.frames()
	r.abortAt(diag.E4402.At(site).Stack(stack).MoreFrames(r.ev.depth-r.ev.implicit-len(stack)), r.wholePath())
	return false
}

// nest opens an implicit frame, a field default or a where run: counted, never listed, no step (DECISIONS 210).
func (r *run) nest(site source.Span) bool {
	if !r.within(site) {
		return false
	}
	r.ev.depth++
	r.ev.implicit++
	return true
}

// unnest closes the implicit frame nest opened.
func (r *run) unnest() {
	r.ev.depth--
	r.ev.implicit--
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

// fnName names a frame (API.md F13): the function, `Record.m`, a case's `V.c.m`, a variant-level `V.m` (TYPES.md §12.1).
func fnName(obj check.Object, self value.Value) string {
	if rec, ok := self.(*value.Record); ok && obj.Kind() == check.ObjMethod {
		if rt := recordOf(rec.T); rt != nil {
			return rt.Name + dot + obj.Name()
		}
		if ct, isCase := rec.T.Base().(*types.CaseType); isCase {
			return caseMethodName(ct, obj)
		}
	}
	return obj.Name()
}

// caseMethodName is `V.m` for a variant-level method of ct's variant, else `V.c.m`.
func caseMethodName(ct *types.CaseType, obj check.Object) string {
	if d := ct.Variant.Decl; d != nil && slices.ContainsFunc(d.Items, func(it syntax.VariantItem) bool { return syntax.Node(it) == obj.Decl() }) {
		return ct.Variant.Name + dot + obj.Name()
	}
	return ct.Variant.Name + dot + ct.Name + dot + obj.Name()
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
	var argTypes []types.Type
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
			args, argTypes = append(args, nil), append(argTypes, nil)
		}
		args[j], argTypes[j] = v, r.typeOf(a.Value)
	}
	r.site = r.span(x)
	c := &std.Call{
		Name: callee.Builtin, Overload: callee.Overload, Recv: recv, Args: args, ArgTypes: argTypes,
		Result: r.typeOf(x), Prov: r.prov(x, value.ProvComputed),
	}
	return r.std(fn(r.host(), c))
}
