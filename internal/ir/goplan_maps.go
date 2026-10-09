package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// HasMaps reports a decoded class holding a map in a field or stored result, this package's or another's this emit's readers read, or a decoded @text fn's result holding one (DECISIONS 340): a data-mode decoder then reads it in file order through jsonMap (WIRE.md §5.8, CODEGEN.md §2.8, §5.9; DECISIONS 312; log-2026-10-06 "U2 (gen/go) done" 5).
func (pl *GoNamePlan) HasMaps() bool {
	isMap := func(t TypeRef) bool { return t.Kind == types.Map }
	return pl.textHolds(isMap) || slices.ContainsFunc(pl.readClasses(), func(class any) bool { return classHolds(class, isMap) })
}

// readClasses are the classes data mode's decoders read: this package's decoded ones, then the other packages' its readers read.
func (pl *GoNamePlan) readClasses() []any {
	if pl.data == nil {
		return nil
	}
	out := slices.DeleteFunc(pl.classes(), func(class any) bool { return !pl.Decoded(class) })
	return append(out, pl.foreign.Read...)
}

// classHolds reports a record or case whose fields or stored results hold a type is reports, at any depth.
func classHolds(class any, is func(TypeRef) bool) bool {
	fields, fns := classBody(class)
	found := false
	visit := func(t TypeRef) { found = found || is(t) }
	for _, f := range fields {
		walkTypeRef(f.Type, visit)
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			walkTypeRef(fn.Result, visit)
		}
	}
	return found
}

// MapKeyTarget is the value the ref keys of map t resolve into at load, in class (WIRE.md §5.8, DECISIONS 312): nil when t's key is no ref, or its target is none class's holders all resolve a ref into (CODEGEN.md §5.8), where the key is read unchecked.
func (pl *GoNamePlan) MapKeyTarget(t TypeRef, class any) *Value {
	if pl.data == nil {
		return nil
	}
	return pl.data.mapKeyTarget(t, class)
}

func (d *goData) mapKeyTarget(t TypeRef, class any) *Value {
	if t.Kind != types.Map || t.Key == nil || t.Key.Kind != types.Ref || t.Key.Ref == nil {
		return nil
	}
	return d.resolvesTo(t.Key.Ref, class)
}

// checksKeys reports a record or case whose own field or stored result holds a map with ref keys its loader checks.
func (d *goData) checksKeys(key any) bool {
	fields, fns := classBody(key)
	holds := func(t TypeRef) bool { return d.keyChecked(t, key) }
	for _, f := range fields {
		if (!f.Optional || f.Type.Kind != types.Never) && holds(f.Type) {
			return true
		}
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated && holds(fn.Result) {
			return true
		}
	}
	return false
}

// keyChecked reports a map, t itself or one its list, optional or map elements are, with ref keys resolved at load in class.
func (d *goData) keyChecked(t TypeRef, class any) bool {
	for {
		if d.mapKeyTarget(t, class) != nil {
			return true
		}
		if t.Elem == nil || t.Kind != types.List && t.Kind != types.Optional && t.Kind != types.Map {
			return false
		}
		t = *t.Elem
	}
}

// ReadsDurations reports what a types-mode decoder reads holding a Duration, which it reads through jsonDuration (WIRE.md §5.1, CODEGEN.md §5.13): a class its decoders or readers read, a pair record they read slot by slot, a dependent type's branch, a decoded @text fn's result (DECISIONS 340).
func (pl *GoNamePlan) ReadsDurations() bool {
	read := pl.readClasses()
	for _, t := range pl.p.Types {
		if d, ok := t.(*Dependent); ok && pl.Decoded(d) {
			read = append(read, d)
		}
	}
	return pl.textHolds(func(t TypeRef) bool { return t.Kind == types.Duration }) || slices.ContainsFunc(read, func(class any) bool {
		return classHoldsDuration(class) || slices.ContainsFunc(pairRecords(class), classHoldsDuration)
	})
}

// classHoldsDuration reports a record, case or dependent type holding a Duration in a field, stored result or branch.
func classHoldsDuration(class any) bool {
	isDuration := func(t TypeRef) bool { return t.Kind == types.Duration }
	d, ok := class.(*Dependent)
	if !ok {
		return classHolds(class, isDuration)
	}
	found := false
	for _, b := range d.Branches {
		walkTypeRef(b.Type, func(t TypeRef) { found = found || isDuration(t) })
	}
	return found
}

// pairRecords are the element records of class's pairs fields, read slot by slot (WIRE.md §5.14).
func pairRecords(class any) []any {
	fields, _ := classBody(class)
	var out []any
	for _, f := range fields {
		if f.Pairs == nil || f.Type.Elem == nil {
			continue
		}
		if rec, ok := f.Type.Elem.Named.(*Record); ok {
			out = append(out, rec)
		}
	}
	return out
}
