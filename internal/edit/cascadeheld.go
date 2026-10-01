package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// givenValue is the value an operation wrote at a path of a JSON source and the one it replaced,
// the held symbols the write kept as the file wrote them (DECISIONS 175), and whether it changed
// the value there.
type givenValue struct {
	path    string
	v, was  value.Value
	held    map[*value.Symbol]bool
	changed bool
}

// restores reports c's field given, by the last operation that wrote it (not one that carried it
// as it was, writing a field beside it), held data, and no later operation changing a field its
// type is computed from: E15 leaves it, as E22 restores data (log-2026-09-29 M4 B7-r2, B7-r3).
func (a *applier) restores(c fieldCheck) bool {
	i, v, ok := a.lastWrite(c)
	return ok && a.given[i].holds(v) && !a.retyped(c, a.given[i+1:])
}

// lastWrite is the last of a.given that wrote c's field, not carrying it as it was while writing
// a field beside it, and the value it wrote there; false for none.
func (a *applier) lastWrite(c fieldCheck) (int, value.Value, bool) {
	p, err := Parse(c.path())
	if err != nil {
		return 0, nil, false
	}
	for i, g := range slices.Backward(a.given) {
		v, covered := g.at(p, g.v)
		if !covered {
			continue
		}
		if was, _ := g.at(p, g.was); v == nil || v != was {
			return i, v, true
		}
	}
	return 0, nil, false
}

// holds reports v a held symbol, or a map with one among its keys or values (log-2026-09-29 M4
// B7-r4: a keyed map is judged by its keys).
func (g givenValue) holds(v value.Value) bool {
	switch x := v.(type) {
	case *value.Symbol:
		return g.held[x]
	case *value.Map:
		return slices.ContainsFunc(x.Keys, g.holds) || slices.ContainsFunc(x.Vals, g.holds)
	}
	return false
}

// keepHeld gives the Undo c's field back as the file wrote it when, kept and written by no
// operation, it held data before the edit (DECISIONS 175): retyped back, the Undo's own E15 would
// drop it (E22; log-2026-09-29 M4 B10).
func (a *applier) keepHeld(c fieldCheck) error {
	if _, _, written := a.lastWrite(c); written || c.t.file == "" {
		return nil // an operation that wrote it gives it back itself
	}
	v := a.baseValue(c)
	if !decodedIn(v) {
		return nil
	}
	lit, err := a.baseLit(v, fieldRules(c.f))
	if err != nil {
		return err
	}
	a.cascadeUndo = append(a.cascadeUndo, Operation{Kind: OpSet, Path: c.path(), Value: lit})
	return nil
}

// baseLit is v, a value of the base, as E23 carries an old value in scope's rules: read, its
// tokens included, in the base snapshot through the base's host.
func (a *applier) baseLit(v value.Value, scope *types.Field) (Lit, error) {
	base := &applier{ctx: a.ctx, env: a.env, snap: a.base, base: a.base, host: a.baseHost, baseHost: a.baseHost, marks: a.marks}
	return base.sourceLit(v, scope)
}

// dropBack is the inverse's value of c's field that E15 drops unread, raw its member: the value
// before the edit as E23 carries it, whose Durations read back whatever their unit, when no
// operation wrote the field; else raw (log-2026-09-29 M4 B10).
func (a *applier) dropBack(c fieldCheck, raw []byte) (Lit, error) {
	if _, _, written := a.lastWrite(c); written {
		return FromJSON(raw), nil
	}
	v := a.baseValue(c)
	if v == nil {
		return FromJSON(raw), nil
	}
	return a.baseLit(v, fieldRules(c.f))
}

// baseValue is c's field's value before the edit, nil when the base does not reach it.
func (a *applier) baseValue(c fieldCheck) value.Value {
	p, err := Parse(c.path())
	if err != nil {
		return nil
	}
	res, err := a.base.open(p)
	if err != nil {
		return nil
	}
	return res.Target
}

// decodedIn reports v a symbol a JSON source wrote, or a map with one among its keys or values.
func decodedIn(v value.Value) bool {
	switch x := v.(type) {
	case *value.Symbol:
		return x.P != nil
	case *value.Map:
		return slices.ContainsFunc(x.Keys, decodedIn) || slices.ContainsFunc(x.Vals, decodedIn)
	}
	return false
}

// retyped reports one of later changing a field c's type is computed from, or a part of one:
// written at it or inside it, or through the record holding it, a Set to its default
// included, which leaves it out (API.md E6, E15; M4.1).
func (a *applier) retyped(c fieldCheck, later []givenValue) bool {
	for _, d := range c.f.DependsOn {
		if d >= len(c.t.decl) {
			continue
		}
		dep := childPath(c.t.path, Seg{Kind: SegField, Name: c.t.decl[d].Name})
		if slices.ContainsFunc(later, func(g givenValue) bool { return g.changed && g.changes(dep) }) {
			return true
		}
	}
	return false
}

// changes reports g writing the value at path dep: at it or inside it, or as a part of what it
// wrote around it whose value differs from the one it replaced.
func (g givenValue) changes(dep string) bool {
	if within(g.path, dep, pathMarks, true) {
		return true
	}
	p, err := Parse(dep)
	if err != nil || !within(dep, g.path, pathMarks, true) {
		return false
	}
	v, _ := g.at(p, g.v)
	was, _ := g.at(p, g.was)
	return !sameValue(v, was)
}

// at is the value at p in v, stated at g's path, when p is g's path followed by field segments;
// covered is false when g did not write p; the value is nil when g wrote it by another route.
func (g givenValue) at(p Path, v value.Value) (value.Value, bool) {
	gp, err := Parse(g.path)
	if err != nil || len(gp.Segs) > len(p.Segs) {
		return nil, false
	}
	if (Path{Package: p.Package, Root: p.Root, Segs: p.Segs[:len(gp.Segs)]}).String() != g.path {
		return nil, false
	}
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
