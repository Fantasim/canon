package eval

import (
	"cmp"
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// site is the declaration a stored value relates, zero for none (ERRORS.md §1.5).
type site struct {
	decl source.Span
	has  bool
}

// store converts v at a storage point of declared type t (EVALUATION.md §4.3); nil: aborted.
func (r *run) store(v value.Value, t types.Type, s site, at *vpath) value.Value {
	if v == nil || t == nil || r.failed {
		return nil
	}
	if !r.ev.needsCheck(t) {
		return v
	}
	w := r.walk(v, t, s, at)
	if w == nil {
		return nil
	}
	return r.ev.carry(w, retype(w, t))
}

// retype gives a scalar the declared type it is stored as (value.Int, Float and Str keep it).
func retype(v value.Value, t types.Type) value.Value {
	switch x := v.(type) {
	case *value.Int:
		if x.T != t {
			return &value.Int{V: x.V, T: t, P: x.P}
		}
	case *value.Float:
		if x.T != t {
			return &value.Float{V: x.V, T: t, P: x.P}
		}
	}
	return v
}

func (r *run) walk(v value.Value, t types.Type, s site, at *vpath) value.Value {
	if !r.ev.needsCheck(t) || isNone(v) {
		return v
	}
	switch x := t.(type) {
	case *types.Alias:
		return r.walk(v, x.Def, s, at)
	case *types.Refined:
		if v = r.walk(v, x.Of, s, at); v == nil {
			return nil
		}
		return r.refine(v, x, s, at)
	case types.Basic:
		return r.scalar(v, x, s, at)
	case *types.OptionalType:
		return r.walk(v, x.Elem, s, at)
	case *types.LitUnionType:
		if str, ok := v.(*value.Str); ok && slices.Contains(x.Literals, str.V) {
			return v
		}
		return r.walk(v, x.Of, s, at)
	case *types.ListType:
		return r.walkList(v, x, s, at)
	case *types.MapType:
		return r.walkMap(v, x, s, at)
	case *types.PairType:
		return r.walkPair(v, x, s, at)
	}
	return v
}

// scalar checks a sized range (E3201) and rounds a Float32, E3202 on overflow (TYPES.md §7.2).
func (r *run) scalar(v value.Value, b types.Basic, s site, at *vpath) value.Value {
	lo, hi, limited := b.Limits()
	switch x := v.(type) {
	case *value.Int:
		if limited && (x.V < lo || x.V > hi) {
			r.soft(r.related(diag.E3201.At(located(v, s.decl), v, b), s), v, at)
		}
	case *value.Dur:
		if limited && (x.Ms < lo || x.Ms > hi) {
			r.soft(r.related(diag.E3201.At(located(v, s.decl), v, b), s), v, at)
		}
	case *value.Float:
		if b.Bits != types.Float32Type.Bits {
			return v
		}
		if math.Abs(x.V) >= float32Max {
			r.soft(r.related(diag.E3202.AtFloat32(located(v, s.decl), v), s), v, at)
			return v
		}
		return r.ev.carry(v, &value.Float{V: float64(float32(x.V)), T: b, P: x.P})
	}
	return v
}

// related relates the declaration the value was stored against.
func (r *run) related(b *diag.Builder, s site) *diag.Builder {
	if s.has {
		b.Related(s.decl, diag.NoteSource(s.decl))
	}
	return b
}

// refine checks one written refinement (TYPES.md §7.4, EVALUATION.md §12.1).
func (r *run) refine(v value.Value, x *types.Refined, s site, at *vpath) value.Value {
	w := r.ev.index.written.of(x)
	rel := s
	if w.has {
		rel = site{decl: w.decl, has: true}
	}
	loc := located(v, s.decl)
	if x.Range != nil && !within(v, x.Range) {
		r.soft(r.related(diag.E3204.At(loc, v, w.arg), rel), v, at)
	}
	if str, ok := v.(*value.Str); ok && x.Pattern != nil && !x.Pattern.MatchString(str.V) {
		r.soft(r.related(diag.E3205.At(loc, v, x.Pattern.String()), rel), v, at)
	}
	if x.Where == nil {
		return v
	}
	holds, ok := r.where(x.Where, v)
	if !ok {
		return nil
	}
	if !holds {
		r.soft(r.related(diag.E3206.At(loc, v, w.arg), rel), v, at)
	}
	return v
}

// within reports a number, a Duration, a string's byte length or a collection's size inside a
// bound: lo inclusive, hi inclusive only for `..=`.
func within(v value.Value, b *types.Bound) bool {
	m, ok := measure(v)
	if !ok {
		return true
	}
	if b.HasLo && m.cmp(b.Lo) < 0 {
		return false
	}
	if !b.HasHi {
		return true
	}
	c := m.cmp(b.Hi)
	return c < 0 || c == 0 && b.HiIncluded
}

// size is what a range refinement measures: an integer (a count, ms) or a Float.
type size struct {
	n       int64
	f       float64
	isFloat bool
}

func measure(v value.Value) (size, bool) {
	switch x := v.(type) {
	case *value.Int:
		return size{n: x.V}, true
	case *value.Dur:
		return size{n: x.Ms}, true
	case *value.Float:
		return size{f: x.V, isFloat: true}, true
	case *value.Str:
		return size{n: int64(len(x.V))}, true
	case *value.List:
		return size{n: int64(len(x.Elems))}, true
	case *value.Map:
		return size{n: int64(len(x.Keys))}, true
	}
	return size{}, false
}

// cmp is the sign of s minus the limit.
func (s size) cmp(l types.Limit) int {
	if s.isFloat {
		return cmp.Compare(s.f, l.F)
	}
	return cmp.Compare(s.n, l.I)
}
