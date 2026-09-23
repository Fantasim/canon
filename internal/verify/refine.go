package verify

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// refinement checks one refinement on a present value, `where` last (TYPES.md §7.4).
func (w *walker) refinement(v value.Value, r *types.Refined, at *Path) {
	src := w.src.types[r]
	if r.Range != nil {
		w.inRange(v, r, src, at)
	}
	if r.Pattern != nil {
		if s, ok := v.(*value.Str); ok && !r.Pattern.MatchString(s.V) {
			site := SiteOf(v)
			w.flag(site, w.src.related(diag.E3205.At(site.Span, v, r.Pattern.String()), r), v, at)
		}
	}
	if r.Asset != nil {
		w.asset(v, r, at)
	}
	if r.Where != nil {
		w.where(v, r, src, at)
	}
}

// inRange checks a range on a number or a Duration, or a length on a string (in bytes), a list
// or a map.
func (w *walker) inRange(v value.Value, r *types.Refined, src written, at *Path) {
	m, ok := measure(v)
	if !ok || m.within(r.Range) {
		return
	}
	site := SiteOf(v)
	w.flag(site, w.src.related(diag.E3204.At(site.Span, v, src.arg), r), v, at)
}

// size is what a range refinement compares: an integer count or value, or a Float.
type size struct {
	i       int64
	f       float64
	isFloat bool
}

func measure(v value.Value) (size, bool) {
	switch x := v.(type) {
	case *value.Int:
		return size{i: x.V}, true
	case *value.Dur:
		return size{i: x.Ms}, true
	case *value.Float:
		return size{f: x.V, isFloat: true}, true
	case *value.Str:
		return size{i: int64(len(x.V))}, true
	case *value.List:
		return size{i: int64(len(x.Elems))}, true
	case *value.Map:
		return size{i: int64(len(x.Keys))}, true
	}
	return size{}, false
}

// compare is -1, 0 or 1 as s is below, at or above the limit.
func (s size) compare(l types.Limit) int {
	if s.isFloat {
		return compareOrdered(s.f, l.F)
	}
	return compareOrdered(s.i, l.I)
}

func compareOrdered[T int64 | float64](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// within: lo is inclusive, hi inclusive only for `..=` (TYPES.md §7.4).
func (s size) within(b *types.Bound) bool {
	if b.HasLo && s.compare(b.Lo) < 0 {
		return false
	}
	if !b.HasHi {
		return true
	}
	c := s.compare(b.Hi)
	return c < 0 || c == 0 && b.HiIncluded
}

// where re-runs the predicate; a hard error aborts the root (EVALUATION.md §7.1).
func (w *walker) where(v value.Value, r *types.Refined, src written, at *Path) {
	holds, ok := w.ev.Where(w.ctx, r.Where, v)
	switch {
	case !ok:
		w.res.Poisoned, w.res.Valid = true, false
	case !holds:
		site := SiteOf(v)
		w.flag(site, w.src.related(diag.E3206.At(site.Span, v, src.arg), r), v, at)
	}
}

// asset checks, in order, a clean relative path, an allowed extension and an existing file;
// only the first failure is reported, the later checks needing the earlier ones.
func (w *walker) asset(v value.Value, r *types.Refined, at *Path) {
	s, ok := v.(*value.Str)
	if !ok {
		return
	}
	a, site := r.Asset, SiteOf(v)
	var b *diag.Builder
	switch {
	case !cleanPath(s.V):
		b = diag.E3703.At(site.Span, s.V)
	case !slices.Contains(a.Exts, extension(s.V)):
		b = diag.E3702.At(site.Span, s.V, a.Exts)
	case w.assets == nil || !w.assets.Exists(a.Root, s.V):
		b = diag.E3701.At(site.Span, s.V, a.Root)
	default:
		return
	}
	w.flag(site, w.src.related(b, r), v, at)
}

// cleanPath: `/` separators, no empty, `.` or `..` segment, no leading `/`, no `\`.
func cleanPath(p string) bool {
	if strings.Contains(p, backslash) {
		return false
	}
	return !slices.ContainsFunc(strings.Split(p, pathSep), func(seg string) bool {
		return seg == "" || seg == dot || seg == parentDir
	})
}

// extension is what follows the last `.` of the last segment; "" without one.
func extension(p string) string {
	last := p[strings.LastIndex(p, pathSep)+1:]
	i := strings.LastIndex(last, dot)
	if i < 0 {
		return ""
	}
	return last[i+1:]
}
