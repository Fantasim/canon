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
	snap := pl.scope(pl.SnapshotName())
	pl.snapshotHidden(snap)
	pl.declare(snap, CppLoad, pl.p.Name, nil)
	for _, v := range pl.values {
		if v.Reload {
			pl.declare(snap, pl.SnapshotGetter(v), pl.valueOrigin(v), v)
			pl.declare(snap, pl.Member(v.Name), pl.valueOrigin(v), v)
		}
	}
	store := pl.scope(pl.StoreName())
	for _, n := range cppStoreMembers {
		pl.declare(store, n, pl.p.Name, nil)
	}
}

// declareVariant declares GetKind and each case with fields' As<Case> (CODEGEN.md §5.5).
func (pl *CppNamePlan) declareVariant(v *Variant) {
	sc := pl.classScope(pl.TypeName(v), v.QName(), v)
	pl.declare(sc, GoGet+GoKind, v.QName(), v)
	pl.declare(sc, CppVariantMember, v.QName(), v)
	for _, c := range v.Cases {
		if len(c.Fields) > 0 {
			pl.declare(sc, pl.AsName(c), v.QName()+qnameSep+c.Name, c)
		}
	}
}

// declareClass declares a record's own members (declareRecordOwn), then each field's and export fn's getters and members.
func (pl *CppNamePlan) declareClass(c any) {
	origin := pl.classOrigin(c)
	sc := pl.classScope(pl.className(c), origin, c)
	if rec, ok := c.(*Record); ok {
		pl.declareRecordOwn(sc, rec, origin)
	}
	fields, fns := classBody(c)
	for _, f := range fields {
		getter, resolved := pl.FieldGetter(f)
		switch {
		case f.Optional && f.Type.Kind == types.Never:
		case f.Input != nil: // CODEGEN.md §7.7: its getter reads detail::<P>Inputs, the class stores nothing
			pl.declare(sc, getter, origin+qnameSep+f.Name, f)
		default:
			pl.slotNames(sc, c, cppSlot{getter: getter, resolved: resolved, canon: f.Name, t: f.Type, item: f, origin: origin + qnameSep + f.Name})
		}
	}
	for _, fn := range fns {
		getter, resolved := pl.StoredGetter(fn)
		if fn.Kind == FnTranslated {
			pl.declare(sc, resolved, origin+qnameSep+fn.Name, fn)
			continue
		}
		t, _ := goUnwrap(fn.Result)
		pl.slotNames(sc, c, cppSlot{getter: getter, resolved: resolved, canon: fn.Name, t: t, item: fn, origin: origin + qnameSep + fn.Name})
	}
}

// declareRecordOwn declares a record's static Load, the first non-@reload record value's, and a table entry's id and retired getters and members (CODEGEN.md §5.3, §5.9).
func (pl *CppNamePlan) declareRecordOwn(sc *nameScope, rec *Record, origin string) {
	if i := slices.IndexFunc(pl.values, func(v *Value) bool { return !v.Reload && v.Type.Kind == types.Record && v.Type.Named == rec }); i >= 0 {
		pl.declare(sc, CppLoad, pl.valueOrigin(pl.values[i]), pl.values[i])
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
		pl.declare(sc, pl.RefMember(s.canon), s.origin, s.item)
	}
	pl.declare(sc, s.getter, s.origin, s.item)
	pl.declare(sc, pl.Member(s.canon), s.origin, s.item)
}

// declareContainer declares a container's members, its static Load unless @reload, and each non-optional @stable field's FindBy<F> and its two sorted indexes (CODEGEN.md §5.9).
func (pl *CppNamePlan) declareContainer(v *Value) {
	name, origin := pl.ContainerName(v), pl.valueOrigin(v)
	sc := pl.containerScope(v, name)
	for _, n := range cppContainerMembers {
		pl.declare(sc, n, origin, v)
	}
	if !v.Reload {
		pl.declare(sc, CppLoad, origin, v)
	}
	rec := containerRecord(v)
	if rec == nil {
		return
	}
	for _, f := range rec.Fields {
		if !findsBy(f) {
			continue
		}
		by := pl.FindBy(f)
		for _, n := range []string{by.Func, by.Keys, by.Index} {
			pl.declare(sc, n, rec.QName()+qnameSep+f.Name, f)
		}
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

// containerRecord is the record a table's or keyed list's rows hold, nil for another element.
func containerRecord(v *Value) *Record {
	if v.Type.Elem == nil {
		return nil
	}
	rec, _ := v.Type.Elem.Named.(*Record)
	return rec
}

// findsBy reports a @stable field its container finds by, a non-optional one (CODEGEN.md §5.9).
func findsBy(f *Field) bool { return f.Stable && !f.Optional }
