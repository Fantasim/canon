package types

import "slices"

// Past is `past of`, statically of; false with of unchanged off pastKinds (TYPES.md §8.4).
func Past(of Type) (Type, bool) {
	if !pastBase(of.Base()) {
		return of, false
	}
	return &Refined{Of: of, Past: true}, true
}

// pastBase reports t of pastKinds, or a type function with no branch outside them (TYPES.md §8.4).
func pastBase(t Type) bool {
	fn := typeFuncOf(t)
	if fn == nil {
		return pastKinds[t.Kind()]
	}
	return !slices.ContainsFunc(Branches(fn), func(b Type) bool { return !pastKinds[b.Base().Kind()] })
}

// PastOnly reports r carries no refinement beyond its `past` flag: Range, Pattern, Where and Asset all unset (TYPES.md §8.4).
func PastOnly(r *Refined) bool {
	return r.Range == nil && r.Pattern == nil && r.Where == nil && r.Asset == nil
}
