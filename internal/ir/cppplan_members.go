package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// declareMembers declares each class's members (CODEGEN.md §5.4, §5.5, §7.2), each container's (§5.9) and the snapshot's (§5.11).
func (pl *CppNamePlan) declareMembers() {
	for _, c := range pl.classes() {
		if v, ok := c.(*Variant); ok {
			pl.declareVariant(v)
			continue
		}
		pl.declareClass(c)
	}
	for _, v := range pl.values {
		if v.Type.Kind != types.Record {
			pl.declareContainer(v)
		}
	}
	if !pl.reloads() {
		return
	}
	snap := pl.scope(pl.Upper() + goSnapshotSuffix)
	pl.declare(snap, cppLoad, pl.p.Name, nil)
	for _, v := range pl.values {
		if v.Reload {
			pl.declare(snap, cppOverride(v.Cpp, GoGet+cppUpperCamel(v.Name)), v.Name, v)
			pl.declare(snap, pl.Member(v.Name), v.Name, v)
		}
	}
	store := pl.scope(pl.Upper() + goStoreSuffix)
	for _, n := range cppStoreMembers {
		pl.declare(store, n, pl.p.Name, nil)
	}
}

// declareVariant declares GetKind and each case with fields' As<Case> (CODEGEN.md §5.5).
func (pl *CppNamePlan) declareVariant(v *Variant) {
	sc := pl.scope(pl.TypeName(v))
	pl.declare(sc, GoGet+GoKind, v.QName(), v)
	pl.declare(sc, cppVariantMember, v.QName(), v)
	for _, c := range v.Cases {
		if len(c.Fields) > 0 {
			pl.declare(sc, pl.AsName(c), v.QName()+qnameSep+c.Name, c)
		}
	}
}

// declareClass declares a record's own members (declareRecordOwn), then each field's and export fn's getters and members.
func (pl *CppNamePlan) declareClass(c any) {
	origin := pl.classOrigin(c)
	sc := pl.scope(pl.className(c))
	if rec, ok := c.(*Record); ok {
		pl.declareRecordOwn(sc, rec, origin)
	}
	fields, fns := classBody(c)
	for _, f := range fields {
		if f.Optional && f.Type.Kind == types.Never {
			continue
		}
		getter, resolved := pl.FieldGetter(f)
		pl.slotNames(sc, c, cppSlot{getter: getter, resolved: resolved, canon: f.Name, t: f.Type, item: f, origin: origin + qnameSep + f.Name})
	}
	for _, fn := range fns {
		name := pl.FnName(fn)
		if fn.Kind == FnTranslated {
			pl.declare(sc, name, origin+qnameSep+fn.Name, fn)
			continue
		}
		t, _ := goUnwrap(fn.Result)
		pl.slotNames(sc, c, cppSlot{getter: name + cppKeySuffix(t), resolved: name, canon: fn.Name, t: t, item: fn, origin: origin + qnameSep + fn.Name})
	}
}

// declareRecordOwn declares a record's static Load, the first non-@reload record value's, and a table entry's id and retired getters and members (CODEGEN.md §5.3, §5.9).
func (pl *CppNamePlan) declareRecordOwn(sc *nameScope, rec *Record, origin string) {
	if i := slices.IndexFunc(pl.values, func(v *Value) bool { return !v.Reload && v.Type.Kind == types.Record && v.Type.Named == rec }); i >= 0 {
		pl.declare(sc, cppLoad, pl.values[i].Name, pl.values[i])
	}
	if slices.ContainsFunc(pl.values, func(v *Value) bool { return tableRecord(v) == rec }) {
		for _, n := range cppEntryMembers {
			pl.declare(sc, n, origin, rec)
		}
	}
}

// cppSlot is a field or stored result as a class stores it: its getter, its resolved getter's name, its Canon name, its type without the outer `?`.
type cppSlot struct {
	getter, resolved, canon, origin string
	t                               TypeRef
	item                            any
}

// slotNames declares a slot's resolved getter and entry member when its ref resolves, then its getter and member (CODEGEN.md §5.8, §7.2).
func (pl *CppNamePlan) slotNames(sc *nameScope, class any, s cppSlot) {
	if pl.Resolves(s.t, class) {
		pl.declare(sc, s.resolved, s.origin, s.item)
		pl.declare(sc, pl.Member(s.canon)+cppRefSuffix, s.origin, s.item)
	}
	pl.declare(sc, s.getter, s.origin, s.item)
	pl.declare(sc, pl.Member(s.canon), s.origin, s.item)
}

// declareContainer declares a container's members, its static Load unless @reload, and each non-optional @stable field's FindBy<F> and its two sorted indexes (CODEGEN.md §5.9).
func (pl *CppNamePlan) declareContainer(v *Value) {
	name, origin := pl.ContainerName(v), pl.p.Name+qnameSep+v.Name
	sc := pl.scope(name)
	for _, n := range cppContainerMembers {
		pl.declare(sc, n, origin, v)
	}
	if !v.Reload {
		pl.declare(sc, cppLoad, origin, v)
	}
	rec := tableRecord(v)
	if rec == nil && v.Type.Elem != nil {
		rec, _ = v.Type.Elem.Named.(*Record)
	}
	if rec == nil {
		return
	}
	for _, f := range rec.Fields {
		if !f.Stable || f.Optional {
			continue
		}
		by := cppByPrefix + cppUpperCamel(f.Name)
		for _, n := range []string{goFindByPrefix + cppUpperCamel(f.Name), by + cppKeys + underscore, by + underscore} {
			pl.declare(sc, n, rec.QName()+qnameSep+f.Name, f)
		}
	}
}

// declareDetail declares detail's access struct with its loaders and resolvers, the decoders when a class is decoded, and each translated method's pure function <Class>_<fn> (CODEGEN.md §2.7 step 4, §3.3, §7.6).
func (pl *CppNamePlan) declareDetail() {
	sc := pl.scope(cppDetail)
	access := pl.Upper() + cppAccessSuffix
	pl.declareDetailName(sc, access, pl.p.Name, nil)
	pl.share(pl.e.Namespace+cppScope+cppDetail, access, pl.p.Name, nil)
	pl.declareAccess(access)
	if len(pl.classes()) > 0 {
		pl.declareDetailName(sc, cppDecode, pl.p.Name, nil)
		pl.shared = append(pl.shared, cppShared{scope: pl.e.Namespace + cppScope + cppDetail, name: cppDecode, origin: pl.p.Name, overload: true})
	}
	for _, m := range pl.methods() {
		name := pl.PureName(pl.className(m.class), m.fn)
		pl.declareDetailName(sc, name, m.origin, m.fn)
		pl.share(pl.e.Namespace+cppScope+cppDetail, name, m.origin, m.fn)
	}
}

// declareDetailName declares a name of detail, which also hides the namespace's own inside detail, where the decoders name every class: a namespace name equal to it collides (log-2026-09-24 "ir plans review").
func (pl *CppNamePlan) declareDetailName(sc *nameScope, name, origin string, item any) {
	if first, ok := pl.ns.names[name]; ok && !pl.reported[originPair{first, origin}] {
		pl.reported[originPair{first, origin}] = true
		pl.problems = append(pl.problems, GoNameProblem{Kind: GoCollision, Scope: sc.what, Name: name, First: first, Origin: origin, Item: pl.nsItems[name]})
	}
	pl.declare(sc, name, origin, item)
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

// declareConformance declares the conformance namespace: its capture and show helpers, each translated fn's vector struct and table, the methods' then the package fns', with the struct's fields, and Run<P>Conformance (CONFORMANCE.md §7).
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
		pl.declare(sc, n, cppConformance, nil)
	}
	if pl.showsOptional(methods, fns) {
		pl.declare(sc, cppShow, cppConformance, nil)
	}
	run := cppRunPrefix + pl.Upper() + goConformanceSuffix
	pl.declare(sc, run, pl.p.Name, nil)
	pl.share(pl.e.Namespace+cppScope+cppConformance, run, pl.p.Name, nil)
	for _, m := range methods {
		pl.declareVectors(sc, pl.className(m.class), m.origin, m.fn)
	}
	for _, fn := range fns {
		pl.declareVectors(sc, "", fn.Name, fn)
	}
}

// declareVectors declares a fn's vector struct <Owner><Fn>Vector with its fields, each input verbatim and `_` added to want and code, and its table k<Owner><Fn> (CONFORMANCE.md §7.2).
func (pl *CppNamePlan) declareVectors(sc *nameScope, owner, origin string, fn *ExportFn) {
	base := owner + cppUpperCamel(fn.Name)
	pl.declare(sc, base+cppVectorSuffix, origin, fn)
	pl.declare(sc, cppConstPrefix+base, origin, fn)
	fields := pl.scope(base + cppVectorSuffix)
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
	sc := pl.scope(access)
	for _, v := range pl.values {
		pl.declare(sc, cppLoad+cppUpperCamel(v.Name), v.Name, v)
	}
	if pl.reloads() {
		pl.declare(sc, cppLoad+goSnapshotSuffix, pl.p.Name, nil)
	}
	if slices.ContainsFunc(pl.classes(), func(c any) bool { return pl.walks(c, map[any]bool{}) }) {
		pl.declare(sc, cppResolve, pl.p.Name, nil)
	}
}

// walks reports a class holding a resolved ref, itself or through the classes its fields hold (a variant through its cases): a loader resolves it after reading (CODEGEN.md §5.8).
func (pl *CppNamePlan) walks(class any, seen map[any]bool) bool {
	if seen[class] || !slices.Contains(pl.classes(), class) {
		return false
	}
	seen[class] = true
	if v, ok := class.(*Variant); ok {
		return slices.ContainsFunc(casesWithFields(v), func(c any) bool { return pl.walks(c, seen) })
	}
	fields, fns := classBody(class)
	for _, f := range fields {
		if pl.Resolves(f.Type, class) || slices.ContainsFunc(cppClassesOf(f.Type, nil), func(c any) bool { return pl.walks(c, seen) }) {
			return true
		}
	}
	return slices.ContainsFunc(fns, func(fn *ExportFn) bool {
		t, _ := goUnwrap(fn.Result)
		return fn.Kind != FnTranslated && pl.Resolves(t, class)
	})
}
