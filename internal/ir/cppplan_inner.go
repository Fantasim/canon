package ir

import (
	"slices"
)

// declareDetail declares detail's access struct with its loaders and resolvers, the decoders when a class is decoded, each dependent type's Decode<Alias>, each translated method's pure function <Class>_<fn> (CODEGEN.md §2.7 step 4, §3.3, §5.6, §7.6), and the input namespace (§7.7).
func (pl *CppNamePlan) declareDetail() {
	sc := pl.scope(cppDetail)
	access := pl.AccessName()
	pl.shareInner(sc, access, pl.p.Name, nil)
	pl.declareAccess(access)
	if len(pl.classes()) > 0 {
		pl.declareInner(sc, CppDecode, pl.p.Name, nil)
		pl.shareAs(pl.e.Namespace+cppScope+cppDetail, CppDecode, pl.p.Name, nil, meetsOverload)
	}
	for _, t := range pl.p.Types {
		if d, ok := t.(*Dependent); ok {
			pl.shareInner(sc, pl.Dependent(d).Decode, d.QName(), d)
		}
	}
	for _, m := range pl.methods() {
		pl.shareInner(sc, pl.PureName(pl.className(m.class), m.fn), m.origin, m.fn)
	}
	pl.declareInputSlots(sc)
}

// declareInner declares a name of detail or conformance, which inside that namespace hides the namespace's own name, which detail's decoders and conformance's vectors name: a namespace name equal to it collides (log-2026-09-24 "ir plans review", "Owed (C++ plan)").
func (pl *CppNamePlan) declareInner(sc *nameScope, name, origin string, item any) {
	if first, ok := pl.ns.names[name]; ok && !pl.reported[originPair{first, origin}] {
		pl.reported[originPair{first, origin}] = true
		pl.problems = append(pl.problems, GoNameProblem{Kind: GoCollision, Scope: sc.what, Name: name, First: first, Origin: origin, Item: pl.nsItems[name]})
	}
	pl.declare(sc, name, origin, item)
}

// shareInner is declareInner for a name the header declares, which the other packages of the namespace share (CODEGEN.md §3.5).
func (pl *CppNamePlan) shareInner(sc *nameScope, name, origin string, item any) {
	pl.localInner(sc, name, origin, item, meetsNever)
}

// localInner is shareInner for a name of meets kind: a conformance file's anonymous namespace holds names local to it, which hide the namespace's names of every package it sees (log-2026-09-24 "Owed (C++ plan)").
func (pl *CppNamePlan) localInner(sc *nameScope, name, origin string, item any, meets cppMeet) {
	pl.declareInner(sc, name, origin, item)
	pl.shareAs(pl.e.Namespace+cppScope+sc.what, name, origin, item, meets)
}

// cppMethod is a translated method with its class and its name in messages.
type cppMethod struct {
	fn     *ExportFn
	class  any
	origin string
}

// methods are the translated methods in class order.
func (pl *CppNamePlan) methods() []cppMethod {
	var out []cppMethod
	for _, c := range pl.classes() {
		_, fns := classBody(c)
		for _, fn := range fns {
			if fn.Kind == FnTranslated {
				out = append(out, cppMethod{fn: fn, class: c, origin: pl.classOrigin(c) + qnameSep + fn.Name})
			}
		}
	}
	return out
}

// declareConformance declares the conformance namespace: its capture and show helpers, Run<P>Conformance, each translated fn's vector struct and table, the methods' then the package fns', with the struct's fields (CONFORMANCE.md §7).
func (pl *CppNamePlan) declareConformance() {
	methods := pl.methods()
	var fns []*ExportFn
	for _, fn := range pl.p.Fns {
		if fn.Kind == FnTranslated {
			fns = append(fns, fn)
		}
	}
	if len(methods)+len(fns) == 0 {
		return
	}
	sc := pl.scope(cppConformance)
	for _, n := range cppConformanceOwn {
		pl.localInner(sc, n, cppConformance, nil, meetsLocal)
	}
	if pl.showsOptional(methods, fns) {
		pl.localInner(sc, cppShow, cppConformance, nil, meetsLocal)
	}
	pl.shareInner(sc, pl.RunConformanceName(), pl.p.Name, nil)
	for _, m := range methods {
		pl.declareVectors(sc, pl.className(m.class), m.origin, m.fn)
	}
	for _, fn := range fns {
		pl.declareVectors(sc, "", pl.fnOrigin(fn), fn)
	}
}

// declareVectors declares a fn's vector struct, its fields and table (CONFORMANCE.md §7.2).
func (pl *CppNamePlan) declareVectors(sc *nameScope, owner, origin string, fn *ExportFn) {
	vec := pl.Vector(owner, fn)
	pl.localInner(sc, vec.Struct, origin, fn, meetsLocal)
	pl.localInner(sc, vec.Table, origin, fn, meetsLocal)
	fields := pl.scope(vec.Struct)
	fields.hidden = pl.signatureTypes(fn)
	var inputs []string
	for _, r := range fn.Reads {
		inputs = append(inputs, r.Name)
	}
	for _, p := range fn.Params {
		inputs = append(inputs, p.Name)
	}
	for _, in := range inputs {
		field := cppVerbatim(in)
		if slices.Contains(goVectorOwn, in) {
			field = in + underscore
		}
		pl.declare(fields, field, origin+qnameSep+in, fn)
	}
	for _, n := range goVectorOwn {
		pl.declare(fields, n, origin, fn)
	}
}

// showsOptional reports a translated fn reading an optional path of self: the conformance file then writes Show (CONFORMANCE.md §7.2).
func (pl *CppNamePlan) showsOptional(methods []cppMethod, fns []*ExportFn) bool {
	optional := func(fn *ExportFn) bool {
		return slices.ContainsFunc(fn.Reads, func(r *Read) bool { return r.Optional })
	}
	return slices.ContainsFunc(methods, func(m cppMethod) bool { return optional(m.fn) }) || slices.ContainsFunc(fns, optional)
}

// declareAccess declares the access struct's members (CODEGEN.md §7.6): Load<V> per value, LoadSnapshot with @reload values, Resolve when a class resolves a ref at load.
func (pl *CppNamePlan) declareAccess(access string) {
	sc := pl.accessScope(access)
	for _, v := range pl.values {
		pl.declare(sc, pl.AccessLoader(v), pl.valueOrigin(v), v)
	}
	if pl.reloads() {
		pl.declare(sc, pl.AccessSnapshotLoader(), pl.p.Name, nil)
	}
	if slices.ContainsFunc(pl.classes(), func(c any) bool { return pl.walks(c, map[any]bool{}) }) {
		pl.declare(sc, CppResolve, pl.p.Name, nil)
	}
}
