package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// goData is data mode's layout (CODEGEN.md §5.8, §5.9, §5.11): per class (a *Record, *Variant or *Case of the package), the emitted values holding it by value and whether a loader decodes it whole; a ref's slot resolves only inside its holder's own value or the snapshot.
type goData struct {
	pl      *GoNamePlan
	holders map[any][]*Value
	decoded map[any]bool
}

// indexClasses maps each own field and export method to its record or case, and each record, variant, case with fields and dependent type to its Go type.
func (pl *GoNamePlan) indexClasses() {
	pl.classOf, pl.goNameOf = map[any]any{}, map[any]string{}
	own := func(class any, goName string, fields []*Field, fns []*ExportFn) {
		pl.goNameOf[class] = goName
		for _, f := range fields {
			pl.classOf[f] = class
		}
		for _, fn := range fns {
			pl.classOf[fn] = class
		}
	}
	for _, t := range pl.p.Types {
		switch x := t.(type) {
		case *Record:
			own(x, pl.TypeName(x), x.Fields, x.Methods)
		case *Variant:
			pl.goNameOf[x] = pl.TypeName(x)
			for _, c := range x.Cases {
				own(c, pl.CaseName(x, c), c.Fields, c.Methods)
			}
		case *Dependent:
			pl.goNameOf[x] = pl.TypeName(x)
		}
	}
}

// newGoData finds, from each emitted value's rows or record, the classes it holds and the classes its loader decodes (a pairs record is read slot by slot, not decoded whole).
func newGoData(pl *GoNamePlan) *goData {
	d := &goData{pl: pl, holders: map[any][]*Value{}, decoded: map[any]bool{}}
	for _, v := range pl.emitted {
		if key := goRootClass(v); key != nil {
			d.hold(key, v, map[any]bool{})
			d.decode(key)
		}
	}
	return d
}

// goRootClass is the record of a value's rows (a table or keyed list) or of the value itself.
func goRootClass(v *Value) any {
	t := v.Type
	if t.Kind != types.Record && t.Elem != nil {
		t = *t.Elem
	}
	if t.Kind == types.Record && t.Named != nil {
		return t.Named
	}
	return nil
}

func (d *goData) hold(key any, v *Value, seen map[any]bool) {
	if seen[key] {
		return
	}
	seen[key] = true
	d.holders[key] = append(d.holders[key], v)
	for _, to := range goHeldBy(key, true) {
		d.hold(to, v, seen)
	}
}

func (d *goData) decode(key any) {
	if d.decoded[key] {
		return
	}
	d.decoded[key] = true
	for _, dep := range heldDependents(key) {
		d.decoded[dep] = true
	}
	for _, to := range goHeldBy(key, false) {
		d.decode(to)
	}
}

// goHeldBy are the classes a class holds by value: a variant its cases with fields, a record or case those of its fields (pairs fields when pairs is set) and of its stored fns' results.
func goHeldBy(key any, pairs bool) []any {
	if v, ok := key.(*Variant); ok {
		return casesWithFields(v)
	}
	fields, fns := classBody(key)
	var out []any
	for _, f := range fields {
		if pairs || f.Pairs == nil {
			out = goClassesOf(f.Type, out)
		}
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			out = goClassesOf(fn.Result, out)
		}
	}
	return out
}

// goClassesOf adds the record or variant t is, or its element's, through lists, optionals and map values.
func goClassesOf(t TypeRef, out []any) []any {
	if (t.Kind == types.Record || t.Kind == types.Variant) && t.Named != nil {
		return append(out, t.Named)
	}
	if t.Elem != nil {
		return goClassesOf(*t.Elem, out)
	}
	return out
}

// layout lays a ref slot out as data mode stores it: its key always, its entry when it resolves, ok when optional (CODEGEN.md §5.8); nothing in baked mode (d nil).
func (d *goData) layout(s *GoSlot, class any) {
	if d == nil || s.Ref == nil {
		return
	}
	s.Resolved = d.resolvesTo(s.Ref, class) != nil
	s.Main, s.Key, s.OK = s.Resolved, true, s.Optional
}

// resolvesTo is the value a ref of class resolves into: an emitted container every holder of the class resolves it to, the holder being that container or both being @reload; nil else, the ref then a key only (CODEGEN.md §5.8, §5.11; log-2026-09-24 "ir name plans + support plan").
func (d *goData) resolvesTo(r *RefTarget, class any) *Value {
	if r.Coll != types.CollLet || r.Local || r.Pkg != d.pl.p.Name || d.pl.byValue[r.Value] == nil {
		return nil
	}
	target := d.pl.byValue[r.Value]
	if !IsContainer(target) || !allResolve(d.holders[class], target) {
		return nil
	}
	return target
}

// allResolve reports holders, at least one, each resolving a ref into target: target itself, or @reload as target is (CODEGEN.md §5.8, §5.11).
func allResolve(holders []*Value, target *Value) bool {
	return len(holders) > 0 && !slices.ContainsFunc(holders, func(w *Value) bool { return w != target && (!w.Reload || !target.Reload) })
}

// needsWalk reports a class holding a resolved ref, itself or through a class it holds: a loader then resolves it after reading (CODEGEN.md §5.8).
func (d *goData) needsWalk(key any) bool { return d.reaches(key, map[any]bool{}) }

func (d *goData) reaches(key any, seen map[any]bool) bool {
	if seen[key] || d.holders[key] == nil {
		return false
	}
	seen[key] = true
	if d.resolvesOwn(key) {
		return true
	}
	for _, to := range goHeldBy(key, true) {
		if d.reaches(to, seen) {
			return true
		}
	}
	return false
}

// resolvesOwn reports a record or case whose own field or precomputed method holds a resolved ref, or a map with ref keys the loader checks.
func (d *goData) resolvesOwn(key any) bool {
	if d.checksKeys(key) {
		return true
	}
	fields, fns := classBody(key)
	for _, f := range fields {
		if (!f.Optional || f.Type.Kind != types.Never) && d.pl.Slot(f).Resolved {
			return true
		}
	}
	for _, fn := range fns {
		if fn.Kind == FnPrecomputed && d.pl.MethodSlot(fn).Resolved {
			return true
		}
	}
	return false
}

// Decoded reports a record, variant or case a data-mode loader decodes whole, or a dependent type one of them holds: it gets decode<T> (CODEGEN.md §5.6, §6.1).
func (pl *GoNamePlan) Decoded(class any) bool { return pl.data != nil && pl.data.decoded[class] }

// NeedsWalk reports a decoded class whose refs, or its held classes' refs, a data-mode loader resolves: it gets resolve<T> (CODEGEN.md §5.8).
func (pl *GoNamePlan) NeedsWalk(class any) bool { return pl.Decoded(class) && pl.Walks(class) }

// Walks is NeedsWalk for any class a data-mode load holds, decoded whole or not: a pairs-held class resolves its refs inline (log-2026-09-24 "Consumer units (A5)").
func (pl *GoNamePlan) Walks(class any) bool { return pl.data != nil && pl.data.needsWalk(class) }

// Holders are the emitted values holding a class by value, in declaration order (data mode).
func (pl *GoNamePlan) Holders(class any) []*Value {
	if pl.data == nil {
		return nil
	}
	return pl.data.holders[class]
}
