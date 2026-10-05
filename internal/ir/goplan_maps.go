package ir

import "github.com/fantasim/canonlang/internal/types"

// HasMaps reports a decoded class holding a map in a field or stored result: a data-mode decoder then reads it in file order through jsonMap (WIRE.md §5.8, CODEGEN.md §5.9, DECISIONS 312).
func (pl *GoNamePlan) HasMaps() bool {
	for _, class := range pl.classes() {
		if !pl.Decoded(class) {
			continue
		}
		fields, fns := classBody(class)
		found := false
		visit := func(t TypeRef) { found = found || t.Kind == types.Map }
		for _, f := range fields {
			walkTypeRef(f.Type, visit)
		}
		for _, fn := range fns {
			if fn.Kind != FnTranslated {
				walkTypeRef(fn.Result, visit)
			}
		}
		if found {
			return true
		}
	}
	return false
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
