package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// StableAmendment is an amendment LOCK.md §6.1 forbids, left to lock as E6004 (DECISIONS 187).
type StableAmendment struct {
	Pkg, Layer string
	Table      string // the stable table an entry would be added to, "" for a field
	Field      string // the @stable field that would change
	Span       source.Span
}

// StableAmendments are the forbidden amendments met so far, in the order met.
func (e *Evaluator) StableAmendments() []StableAmendment {
	return e.stable
}

// amending is one amendment being applied.
type amending struct {
	layer string
	a     *syntax.Amendment
	rhs   value.Value
	path  string
	root  check.Object
}

// applyLayers applies a let's amendments, layer order then source order (EVALUATION.md §9.3).
func (r *run) applyLayers(obj check.Object, v value.Value) value.Value {
	pkg := r.ev.pkgs[obj.Pkg()]
	if pkg == nil || v == nil {
		return v
	}
	for _, layer := range r.ev.opt.Layers {
		for _, blk := range pkg.Layers[layer] {
			if v = r.amendBlock(obj, blk, layer, v); v == nil {
				return nil
			}
		}
	}
	return v
}

// amendBlock applies the amendments of one block when it amends obj; one of a broken layer file poisons obj silently (TYPES.md §1, DECISIONS 185).
func (r *run) amendBlock(obj check.Object, blk *syntax.AmendBlock, layer string, v value.Value) value.Value {
	if r.ev.info.NameUses[blk.Target] != obj {
		return v
	}
	file := r.ev.index.file[blk]
	if file == nil {
		r.bug(blk)
		return nil
	}
	if r.ev.broken(r.ev.info.Defs[file.Layer]) {
		r.stop()
		return nil
	}
	for _, a := range blk.Items {
		if v = r.amend(obj, blk, a, layer, v); v == nil {
			return nil
		}
	}
	return v
}

// amend evaluates one amendment's value in the layer's file and replaces the sub-value at its
// path; reading the amended let itself is E4301, as for any cycle.
func (r *run) amend(obj check.Object, blk *syntax.AmendBlock, a *syntax.Amendment, layer string, v value.Value) value.Value {
	saved := r.fr
	r.fr = r.ev.rootFrame(r.ev.index.file[blk])
	defer func() { r.fr = saved }()
	rhs := r.eval(a.Value)
	if rhs == nil || len(a.Path) == 0 {
		return nil
	}
	rhs = r.ev.reprov(rhs, &value.Prov{Kind: value.ProvLayer, Span: r.span(a.Value), Layer: layer}, true)
	m := &amending{layer: layer, a: a, rhs: rhs, root: obj, path: r.pathText(a)}
	return r.setPath(v, obj.Type(), a.Path, m)
}

// pathText is an amendment's path as written, for E1905.
func (r *run) pathText(a *syntax.Amendment) string {
	sp := r.span(a.Path[0]).Cover(r.span(a.Path[len(a.Path)-1]))
	return string(r.fr.file.Src.Content[sp.Start:sp.End])
}

// setPath is cur with the value at segs replaced, copy on write; E1905 is hard (EVALUATION.md §9.3).
func (r *run) setPath(cur value.Value, t types.Type, segs []*syntax.AmendSegment, m *amending) value.Value {
	seg := segs[0]
	switch x := cur.(type) {
	case *value.Record:
		return r.setField(x, segs, m)
	case *value.Map:
		return r.setMapKey(x, t, segs, m)
	case *value.None:
		r.fail(diag.E1905.AtNone(r.span(seg), m.path))
		return nil
	}
	if keyedColl(cur) || isPlainList(cur) {
		return r.setElement(cur, t, segs, m)
	}
	r.fail(diag.E1905.AtField(r.span(seg), m.path))
	return nil
}

func isPlainList(v value.Value) bool {
	l, ok := v.(*value.List)
	return ok && !std.Keyed(l)
}

// setField replaces a field; later defaulted fields follow (EVALUATION.md §9.3 step 3).
func (r *run) setField(rec *value.Record, segs []*syntax.AmendSegment, m *amending) value.Value {
	seg := segs[0]
	i := -1
	if seg.Name != nil {
		i = fieldIndex(rec.T, seg.Name.Name)
	}
	if i < 0 {
		r.fail(r.noField(rec, seg, m))
		return nil
	}
	f := fieldsOf(rec.T)[i]
	nv := r.next(rec.Fields[i], f.Type, segs, m, r.ev.fieldSite(f, rec.T))
	if nv == nil {
		return nil
	}
	if f.Stable && !value.Equal(rec.Fields[i], nv) {
		return r.forbid(m, "", f.Name)
	}
	cp := *rec
	cp.Fields, cp.Set = append([]value.Value(nil), rec.Fields...), append([]bool(nil), rec.Set...)
	cp.Fields[i], cp.Set[i] = nv, true
	if len(segs) == 1 && !r.derive(&cp, i) {
		return nil
	}
	return &cp
}

// noField is E1905 for a field the value lacks: a case without it, or none at all.
func (r *run) noField(rec *value.Record, seg *syntax.AmendSegment, m *amending) *diag.Builder {
	if ct, ok := rec.T.Base().(*types.CaseType); ok && seg.Name != nil {
		return diag.E1905.AtCaseField(r.span(seg), m.path, ct.Name, seg.Name.Name)
	}
	return diag.E1905.AtField(r.span(seg), m.path)
}

// next is the new value at a segment: the amendment's value converted at its storage point
// when it is the last, else the rest of the path applied to the current value.
func (r *run) next(cur value.Value, t types.Type, segs []*syntax.AmendSegment, m *amending, s site) value.Value {
	if len(segs) == 1 {
		nv := r.store(m.rhs, t, s, nil)
		if field, table, found := stableChange(cur, nv, t); found {
			return r.forbid(m, tableName(table, m), field)
		}
		return nv
	}
	if cur == nil || isNone(cur) {
		r.fail(diag.E1905.AtNone(r.span(segs[0]), m.path))
		return nil
	}
	return r.setPath(cur, unwrapOptional(t), segs[1:], m)
}

func unwrapOptional(t types.Type) types.Type {
	if o, ok := t.Base().(*types.OptionalType); ok {
		return o.Elem
	}
	return t
}

// derive re-evaluates, in declaration order, the fields after field i that came from their
// defaults (not written, not amended).
func (r *run) derive(rec *value.Record, i int) bool {
	for j, f := range fieldsOf(rec.T) {
		if j <= i || rec.Set[j] || f.Default == nil || f.Input != nil {
			continue
		}
		v := r.defaultValue(rec, f, nil)
		if v == nil {
			return false
		}
		rec.Fields[j] = r.ev.inField(v, rec, f)
	}
	return true
}

// tableName is the let an amendment adds a stable entry under, "" for a field change.
func tableName(table bool, m *amending) string {
	if table {
		return m.root.Name()
	}
	return ""
}

// forbid records a stable amendment for lock (E6004) and poisons the value.
func (r *run) forbid(m *amending, table, field string) value.Value {
	r.ev.stable = append(r.ev.stable, StableAmendment{Pkg: m.root.Pkg(), Layer: m.layer, Table: table, Field: field, Span: r.span(m.a)})
	r.stop()
	return nil
}
