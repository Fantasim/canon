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

// MapKeyTarget is the value the ref keys of map t resolve into at load, in class (WIRE.md §5.8, DECISIONS 312, 387): nil when t's key is no ref, or its target is none class's holders all resolve a ref into (CODEGEN.md §5.8), where the key is read unchecked; a literal union over a ref is keyed by its ref arm's target.
func (pl *GoNamePlan) MapKeyTarget(t TypeRef, class any) *Value {
	if pl.data == nil {
		return nil
	}
	return pl.data.mapKeyTarget(t, class)
}

// UnionRef is the ref arm of a literal union over a string-keyed ref, nil for any other type: the union is its string in every mode (CODEGEN.md §4.1, §5.13; DECISIONS 387).
func UnionRef(t TypeRef) *TypeRef {
	if t.Kind == types.LitUnion && t.Elem != nil && t.Elem.Kind == types.Ref && t.Elem.Ref != nil && StringWire(t.Elem) {
		return t.Elem
	}
	return nil
}

// UnionTarget is the value a data loader checks the keys of union t against in class: where a plain ref of its ref arm resolves at load (CODEGEN.md §5.8; DECISIONS 387); nil for another type, or where the key is read unchecked.
func (pl *GoNamePlan) UnionTarget(t TypeRef, class any) *Value {
	if pl.data == nil {
		return nil
	}
	return pl.data.unionTarget(t, class)
}

func (d *goData) unionTarget(t TypeRef, class any) *Value {
	if r := UnionRef(t); r != nil {
		return d.resolvesTo(r.Ref, class)
	}
	return nil
}

// ElemRefTarget is the value ref r, a present element of a list of optional refs, resolves into at load in class: where a plain ref of it does, so a data loader checks it (CODEGEN.md §5.8; DECISIONS 385); nil elsewhere, the key read unchecked.
func (pl *GoNamePlan) ElemRefTarget(r TypeRef, class any) *Value {
	if pl.data == nil {
		return nil
	}
	return pl.data.elemRefTarget(r, class)
}

func (d *goData) elemRefTarget(r TypeRef, class any) *Value {
	if r.Kind != types.Ref || r.Ref == nil {
		return nil
	}
	return d.resolvesTo(r.Ref, class)
}

// LeafChecked reports a field's, stored result's or lookup cell's type t, a union or optional ref a data loader checks in class exactly where it checks a plain ref (CODEGEN.md §5.8, DECISIONS 385, 387): a union, or a list one level deep of unions, optional unions or optional refs; never under a map value or at a second list level, where a plain ref is a key only.
func (pl *GoNamePlan) LeafChecked(t TypeRef, class any) bool {
	return pl.data != nil && pl.data.leafChecked(t, class)
}

func (d *goData) leafChecked(t TypeRef, class any) bool {
	if t.Kind == types.Optional && t.Elem != nil { // a stored result's own optional
		t = *t.Elem
	}
	if d.unionTarget(t, class) != nil {
		return true
	}
	if t.Kind != types.List || t.Elem == nil {
		return false
	}
	e := *t.Elem
	if e.Kind == types.Optional && e.Elem != nil {
		return d.unionTarget(*e.Elem, class) != nil || d.elemRefTarget(*e.Elem, class) != nil
	}
	return d.unionTarget(e, class) != nil
}

func (d *goData) mapKeyTarget(t TypeRef, class any) *Value {
	if t.Kind != types.Map || t.Key == nil {
		return nil
	}
	if t.Key.Kind == types.LitUnion {
		return d.unionTarget(*t.Key, class)
	}
	if t.Key.Kind != types.Ref || t.Key.Ref == nil {
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

// keyChecked reports a map, t itself or one its list, optional or map elements are, with ref keys resolved at load in class, or a type whose unions and optional refs it checks (leafChecked).
func (d *goData) keyChecked(t TypeRef, class any) bool {
	if d.leafChecked(t, class) {
		return true
	}
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
