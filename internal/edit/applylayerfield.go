package edit

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// absentPart is the part of the edit layer's line an edit upto steps down the path rewrites when the
// path meets a field the line's literal leaves out beside a load or an expression: the new field,
// from its default (API.md W7, W8; DECISIONS 462); `computed` when there is none.
func (x *opCtx) absentPart(at staticSite, upto int) (linePart, error) {
	lit, d, ok := x.a.snap.absentField(at.line, at.n, x.res, max(upto, at.n))
	if !ok {
		return linePart{}, &NotEditableError{Reason: ReasonComputed}
	}
	st := x.res.Steps[d]
	dv, ok := x.literalDefault(d, st.Seg.Name)
	if !ok {
		return linePart{}, &NotEditableError{Reason: ReasonComputed}
	}
	w := walk{cur: dv, typ: x.typeAt(d + 1)}
	for _, s := range x.res.Steps[d+1 : upto] {
		if err := w.next(s.Seg); err != nil {
			return linePart{}, &NotEditableError{Reason: ReasonLayer}
		}
	}
	return linePart{node: lit, old: dv, steps: w.steps, depth: d, absent: true}, nil
}

// fillLeftOut gives cur, a record typed from the line's text, the default of the field seg names
// when its literal leaves it out, so a path goes on below it (W8); it stays unwritten (Set false).
func (x *opCtx) fillLeftOut(cur value.Value, seg Seg) bool {
	rec, ok := cur.(*value.Record)
	if !ok || seg.Kind != SegField {
		return false
	}
	i := fieldIndex(fieldsOf(rec.T), seg.Name)
	if i < 0 || rec.Fields[i] != nil {
		return false
	}
	rec.Fields[i], _ = x.a.defaultOf(rec, i)
	return rec.Fields[i] != nil
}

// partInverse is the inverse of an edit of part (E23): below a field its literal left out, a Set of
// that field to its default, which leaves it out again (E6), as layerAbsent; else undoStatic.
func (x *opCtx) partInverse(part linePart, cur value.Value, upto int) error {
	if part.fillAt == 0 || part.fillAt == len(part.steps) {
		return x.undoStatic(cur, upto)
	}
	k := part.depth + part.fillAt
	lit, err := x.a.sourceLit(part.steps[part.fillAt-1].Value, x.declaredAt(k), x.scopeAt(k, true), stated{})
	if err != nil {
		return err
	}
	x.inverse(Operation{Kind: OpSet, Path: x.pathAt(k), Value: lit})
	return nil
}

// literalDefault is the default of field name of the record at depth d of the path, one a literal
// states (Snapshot.literalDefault), computed without the record's other values.
func (x *opCtx) literalDefault(d int, name string) (value.Value, bool) {
	rt := present(x.typeAt(d))
	if !x.a.snap.literalDefault(rt, name) {
		return nil, false
	}
	fields := fieldsOf(rt)
	dv, ok := x.a.defaultOf(&value.Record{T: rt, Fields: make([]value.Value, len(fields)), Set: make([]bool, len(fields))}, fieldIndex(fields, name))
	return dv, ok && printable(dv)
}

// fieldInsert applies mutate to the value a field the line leaves out holds, from its default, and
// inserts the field where W7 puts it; nothing when it keeps its default (E6). Its inverse sets the
// field to that default, which leaves it out again (E23, E6).
func (x *opCtx) fieldInsert(at staticSite, part linePart, mutate func(value.Value) (value.Value, error)) error {
	cur := part.old
	if len(part.steps) > 0 {
		cur = part.steps[len(part.steps)-1].Value
	}
	nv, err := mutate(cur)
	if err != nil {
		return err
	}
	if rec, i, ok := fieldOf(part.old, part.steps); ok && nv != nil && x.a.isDefault(withField(rec, i, nv), i) {
		nv = nil
	}
	nf := withChange(part.old, part.steps, nv)
	if nf == nil || x.a.sameDefault(nf, part.old) {
		return nil
	}
	lit, err := x.a.sourceLit(part.old, x.declaredAt(part.depth+1), x.scopeAt(part.depth+1, true), stated{})
	if err != nil {
		return err
	}
	x.inverse(Operation{Kind: OpSet, Path: x.pathAt(part.depth + 1), Value: lit})
	rt := present(x.typeAt(part.depth))
	fields := fieldsOf(rt)
	rec := &value.Record{T: rt, Fields: make([]value.Value, len(fields)), Set: make([]bool, len(fields))}
	d := &canonDiff{a: x.a}
	if err := d.insertField(rec, fieldIndex(fields, x.res.Steps[part.depth].Seg.Name), nf, part.node.(*syntax.BraceLit)); err != nil {
		return err
	}
	x.w.addCanon(at.f.Src.Path, x.res.root.pkg.Path, d.out)
	return nil
}

// leftOut is the field item an edit of the part leaves out of the record literal holding it: a
// field the part's node gives whole, set to a default a literal states or left to it (E6), beside
// a load or an expression in that literal, which no spread supplies (W9).
func (x *opCtx) leftOut(at staticSite, part linePart, nv value.Value) (*syntax.FieldItem, bool) {
	if len(part.steps) > 0 || part.depth <= at.n {
		return nil, false
	}
	st := x.res.Steps[part.depth-1]
	parent, ok := x.a.snap.nodeAt(at.line, at.n, x.res, part.depth-1)
	if !ok || st.Seg.Kind != SegField || hasSpread(parent) {
		return nil, false
	}
	fi := fieldItem(parent, st.Seg.Name)
	if fi == nil || nv == nil {
		return fi, fi != nil
	}
	dv, ok := x.literalDefault(part.depth-1, st.Seg.Name)
	return fi, ok && x.a.sameDefault(dv, nv)
}
