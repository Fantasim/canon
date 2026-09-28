package shape

import (
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// StripOptional is t without its outer optional, aliases and refinements inside it kept.
func StripOptional(t types.Type) types.Type {
	if o, ok := t.Base().(*types.OptionalType); ok {
		return o.Elem
	}
	return t
}

// KindIn reports t's base kind is one of ks.
func KindIn(t types.Type, ks ...types.Kind) bool {
	return slices.Contains(ks, t.Base().Kind())
}

// ElemOf is the element type of a list, nil for any other type.
func ElemOf(t types.Type) types.Type {
	if l, ok := t.Base().(*types.ListType); ok {
		return l.Elem
	}
	return nil
}

// ValueOf is the value type of a map or dependent map, nil for any other type.
func ValueOf(t types.Type) types.Type {
	switch m := t.Base().(type) {
	case *types.MapType:
		return m.Value
	case *types.DepMapType:
		return m.Value
	}
	return nil
}

// SameType is identity with aliases expanded and refinements kept (VIEWMODEL.md G8, G21).
func SameType(a, b types.Type) bool {
	return types.Identical(a, b) && sameRefinements(a, b)
}

// sameRefinements compares the refinements of two identical types, layer by layer.
func sameRefinements(a, b types.Type) bool {
	a, b = Unalias(a), Unalias(b)
	ra, aRefined := a.(*types.Refined)
	rb, bRefined := b.(*types.Refined)
	if aRefined || bRefined {
		return aRefined && bRefined && sameRefinement(ra, rb) && sameRefinements(ra.Of, rb.Of)
	}
	ea, ka, va := parts(a)
	eb, kb, vb := parts(b)
	return sameParts(ea, eb) && sameParts(ka, kb) && sameParts(va, vb)
}

// parts are the element of an optional or a list, and the key and value of a map.
func parts(t types.Type) (elem, key, val types.Type) {
	switch x := t.(type) {
	case *types.OptionalType:
		return x.Elem, nil, nil
	case *types.ListType:
		return x.Elem, nil, nil
	case *types.MapType:
		return nil, x.Key, x.Value
	}
	return nil, nil, nil
}

func sameParts(a, b types.Type) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return sameRefinements(a, b)
}

// Unalias is t with every alias removed.
func Unalias(t types.Type) types.Type {
	for {
		a, ok := t.(*types.Alias)
		if !ok {
			return t
		}
		t = a.Def
	}
}

func sameRefinement(a, b *types.Refined) bool {
	switch {
	case (a.Range == nil) != (b.Range == nil), (a.Pattern == nil) != (b.Pattern == nil),
		(a.Where == nil) != (b.Where == nil), (a.Asset == nil) != (b.Asset == nil):
		return false
	case a.Range != nil && *a.Range != *b.Range,
		a.Pattern != nil && a.Pattern.String() != b.Pattern.String(),
		a.Where != nil && a.Where.Text != b.Where.Text:
		return false
	}
	return a.Asset == nil || a.Asset.Root == b.Asset.Root && slices.Equal(a.Asset.Exts, b.Asset.Exts)
}

// refinements calls fn on each refinement of t's alias and refinement layers, outermost first,
// and returns the type under them.
func refinements(t types.Type, fn func(*types.Refined)) types.Type {
	for {
		switch x := t.(type) {
		case *types.Alias:
			t = x.Def
		case *types.Refined:
			fn(x)
			t = x.Of
		default:
			return t
		}
	}
}

// IsAsset reports an asset type (TYPES.md §13.4).
func IsAsset(t types.Type) bool {
	asset := false
	refinements(t, func(r *types.Refined) { asset = asset || r.Asset != nil })
	return asset
}

// HasPattern reports a String refined by the regex source src.
func HasPattern(t types.Type, src string) bool {
	found := false
	refinements(t, func(r *types.Refined) { found = found || r.Pattern != nil && r.Pattern.String() == src })
	return found
}

// Bounds reports whether a number type has a lower and an upper bound, as §12.3 encodes them (C12).
func Bounds(t types.Type) (lo, hi bool) {
	under := refinements(t, func(r *types.Refined) {
		if r.Range != nil {
			lo, hi = lo || r.Range.HasLo, hi || r.Range.HasHi
		}
	})
	if b, ok := under.(types.Basic); ok && b.K == types.Int {
		least, most, _ := b.Limits()
		lo, hi = lo || least != math.MinInt64, hi || most != math.MaxInt64
	}
	return lo, hi
}

// MenuType is the record T a value of type t is, or a list, keyed list or table of (VIEWMODEL.md
// N1); nil for none.
func MenuType(t types.Type) types.Type {
	b := t.Base()
	if e := ElemOf(b); e != nil {
		b = e.Base()
	} else if tt, ok := b.(*types.TableType); ok {
		b = tt.Elem.Base()
	}
	if a, ok := b.(*types.AppliedRecord); ok {
		return a.Rec
	}
	if _, ok := b.(*types.RecordType); ok {
		return b
	}
	return nil
}
