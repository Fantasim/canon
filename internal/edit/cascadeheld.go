package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/value"
)

// givenValue is the value an operation wrote at a path of a JSON source, the held symbols the
// write kept as the file wrote them (DECISIONS 175), and whether it changed the value there.
type givenValue struct {
	path    string
	v       value.Value
	held    map[*value.Symbol]bool
	changed bool
}

// restores reports c's field given, by the last operation that wrote it, a held symbol, and no
// later operation changing a field its type is computed from: E15 leaves it, as E22 restores
// data (log-2026-09-29 M4 B7-r2, B7-r3).
func (a *applier) restores(c fieldCheck) bool {
	p, err := Parse(c.path())
	if err != nil {
		return false
	}
	for i, g := range slices.Backward(a.given) {
		if v, covered := g.at(p); covered {
			s, isSymbol := v.(*value.Symbol)
			return isSymbol && g.held[s] && !a.retyped(c, a.given[i+1:])
		}
	}
	return false
}

// retyped reports one of later changing a field c's type is computed from, or a part of one.
func (a *applier) retyped(c fieldCheck, later []givenValue) bool {
	for _, d := range c.f.DependsOn {
		if d >= len(c.t.decl) {
			continue
		}
		dep := childPath(c.t.path, Seg{Kind: SegField, Name: c.t.decl[d].Name})
		if slices.ContainsFunc(later, func(g givenValue) bool { return g.changed && within(g.path, dep, pathMarks, true) }) {
			return true
		}
	}
	return false
}

// at is the value g gave at p, when p is g's path followed by field segments; covered is false
// when g did not write p; the value is nil when g wrote it by another route.
func (g givenValue) at(p Path) (value.Value, bool) {
	gp, err := Parse(g.path)
	if err != nil || len(gp.Segs) > len(p.Segs) {
		return nil, false
	}
	if (Path{Package: p.Package, Root: p.Root, Segs: p.Segs[:len(gp.Segs)]}).String() != g.path {
		return nil, false
	}
	v := g.v
	for _, seg := range p.Segs[len(gp.Segs):] {
		rec, isRecord := v.(*value.Record)
		if seg.Kind != SegField || !isRecord {
			return nil, true
		}
		i := fieldIndex(fieldsOf(rec.T), seg.Name)
		if i < 0 {
			return nil, true
		}
		v = rec.Fields[i]
	}
	return v, true
}

// pathAt is the canonical path of the value at cursor k.
func (x *opCtx) pathAt(k int) string {
	segs := make([]Seg, 0, k)
	for _, st := range x.res.Steps[:min(k, len(x.res.Steps))] {
		segs = append(segs, st.Seg)
	}
	return Path{Package: x.res.root.pkg.Path, Root: x.res.root.obj.Name(), Segs: segs}.String()
}
