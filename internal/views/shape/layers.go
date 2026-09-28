package shape

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/types"
)

// Layers is what the alias and refinement layers of a type state (TYP-04): the intersection of
// their ranges, the outermost pattern, every `where` outermost first, and the asset spec.
type Layers struct {
	Lo, Hi       types.Limit
	HasLo, HasHi bool
	HiIncluded   bool // a Float's upper bound is inclusive; an integer one is always stored inclusive
	Pattern      *regexp.Regexp
	Where        []*types.Predicate
	Asset        *types.AssetSpec
	Under        types.Type // the type under every layer
}

// LayersOf reads t's layers down to the first type that is neither an alias nor a refinement.
// Integer, Duration and length bounds are made inclusive (`..hi` is `..=hi-1`).
func LayersOf(t types.Type) Layers {
	var l Layers
	float := t.Base().Kind() == types.Float
	l.Under = refinements(t, func(r *types.Refined) {
		if l.Pattern == nil {
			l.Pattern = r.Pattern
		}
		if r.Where != nil {
			l.Where = append(l.Where, r.Where)
		}
		if r.Asset != nil && l.Asset == nil {
			l.Asset = r.Asset
		}
		if r.Range != nil {
			l.narrow(*r.Range, float)
		}
	})
	return l
}

// narrow intersects the bounds with b.
func (l *Layers) narrow(b types.Bound, float bool) {
	if !float && b.HasHi && !b.HiIncluded {
		b.Hi.I, b.HiIncluded = b.Hi.I-1, true
	}
	if b.HasLo && (!l.HasLo || above(b.Lo, l.Lo, float)) {
		l.Lo, l.HasLo = b.Lo, true
	}
	if !b.HasHi {
		return
	}
	tighter := !l.HasHi || above(l.Hi, b.Hi, float) || l.Hi == b.Hi && l.HiIncluded && !b.HiIncluded
	if tighter {
		l.Hi, l.HasHi, l.HiIncluded = b.Hi, true, b.HiIncluded || !float
	}
}

// above reports a > b, as floats or as integers.
func above(a, b types.Limit, float bool) bool {
	if float {
		return a.F > b.F
	}
	return a.I > b.I
}
