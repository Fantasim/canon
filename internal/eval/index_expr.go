package eval

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// index is `x[i]`: a position, a key, or a slice (STDLIB.md §4.1, §5, §6, §7).
func (r *run) index(x *syntax.IndexExpr) value.Value {
	v := r.recv(x.X)
	if !r.unwrap(v, false) {
		return nil
	}
	if v = r.deref(v, x); v == nil {
		return nil
	}
	if rng, ok := x.Index.(*syntax.RangeExpr); ok {
		if !r.step(rng) {
			return nil
		}
		lo, hi, ok := r.bounds(rng)
		if !ok {
			return nil
		}
		if rng.Op == syntax.TokRangeIncl {
			if hi == math.MaxInt64 {
				r.fail(diag.E4101.AtInteger(r.span(rng), r.span(rng)))
				return nil
			}
			hi++
		}
		return r.sliceOpen(v, lo, hi, rng.Hi == nil, x)
	}
	i := r.eval(x.Index)
	if i == nil {
		return nil
	}
	return r.lookup(v, i, x)
}

// lookup is v[i] for an evaluated index: a position, a key, or a slice by a Range value.
func (r *run) lookup(v, i value.Value, x *syntax.IndexExpr) value.Value {
	if l, ok := v.(*value.List); ok && !std.Keyed(l) {
		return r.position(l, i, x)
	}
	if s, isStr := v.(*value.Str); isStr {
		if rg, isRange := i.(*value.Range); isRange {
			return r.sliceOpen(s, rg.Start, rg.End, !rg.HasEnd, x)
		}
	}
	if rg, isRange := i.(*value.Range); isRange && keyedColl(v) {
		return r.sliceOpen(v, rg.Start, rg.End, !rg.HasEnd, x)
	}
	return r.lookupKey(v, i, x)
}

// position is element i of a plain list, negative from the end (E4002 out of range), or a
// slice by a Range value.
func (r *run) position(l *value.List, i value.Value, x syntax.Expr) value.Value {
	if rg, ok := i.(*value.Range); ok {
		return r.sliceOpen(l, rg.Start, rg.End, !rg.HasEnd, x)
	}
	n, ok := i.(*value.Int)
	if !ok {
		r.bug(x)
		return nil
	}
	j := n.V
	if j < 0 {
		j += int64(len(l.Elems))
	}
	if j < 0 || j >= int64(len(l.Elems)) {
		r.fail(diag.E4002.AtIndex(r.span(x), n.V, int64(len(l.Elems))))
		return nil
	}
	return r.read(l.Elems[j])
}

// sliceOpen slices a sequence or a string, to its end when open (STDLIB.md §4.1, §7).
func (r *run) sliceOpen(v value.Value, lo, hi int64, open bool, x syntax.Expr) value.Value {
	r.site = r.span(x)
	p := r.prov(x, value.ProvComputed)
	if s, ok := v.(*value.Str); ok {
		if open {
			hi = int64(len(s.V))
		}
		return r.std(std.SliceString(r.host(), s.V, lo, hi, p))
	}
	elems := std.Elems(v)
	if open {
		hi = int64(len(elems))
	}
	a, b, ok := std.SliceBounds(r.host(), int64(len(elems)), lo, hi)
	if !ok || !r.host().Charge(int(b-a)) {
		return r.std(nil, false)
	}
	return &value.List{T: r.typeOf(x), Elems: append([]value.Value(nil), elems[a:b]...), P: p}
}
