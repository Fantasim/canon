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

// amending is one amendment being applied: its value, the instances on its path, outermost
// first, what a type argument names where the path lands, and the keys its path evaluated.
type amending struct {
	layer  string
	a      *syntax.Amendment
	rhs    value.Value
	path   string
	root   check.Object
	holder site // the innermost field or let on the path: a stored value names its declaration (API.md F12)
	at     []*value.Record
	dep    *depCtx
	keys   map[*syntax.AmendSegment]value.Value
	trail  []*vpath // the path after each segment locate followed (API.md F1)
	quiet  bool     // the path is being located: a missing segment is the replace's E1905
	visit  int      // the base walk's visit of the value the path has reached (copies.go)
	inColl bool     // the record the path reaches next is an element of the table or keyed list holding it
	place  bool     // the value the path reaches next is a let's or a collection field's: a collection its entries belong to
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
	if !r.keysKept(v) {
		return nil
	}
	r.settle(v)
	return v
}

// amendBlock applies the amendments of one block when it amends obj; one of a broken layer file poisons obj silently (TYPES.md §1, DECISIONS 185).
func (r *run) amendBlock(obj check.Object, blk *syntax.AmendBlock, layer string, v value.Value) value.Value {
	if r.ev.info.NameUses[blk.Target] != obj {
		return v
	}
	file := r.ev.fileOf(blk)
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

// amend evaluates an amendment's value where its path lands, converts it, replaces (EVALUATION.md §9.3).
func (r *run) amend(obj check.Object, blk *syntax.AmendBlock, a *syntax.Amendment, layer string, v value.Value) value.Value {
	if len(a.Path) == 0 {
		return nil
	}
	saved := r.fr
	r.fr = r.ev.rootFrame(r.ev.fileOf(blk))
	r.fr.layer = layer // what its right-hand side computes, called frames included, is the layer's (API.md §4, F11)
	defer func() { r.fr = saved }()
	if mv := r.moving(); mv.copies == nil { // v is still the base value: its copy facts, once (copies.go)
		mv.copies = r.ev.copiesOf(v, r.reads)
	}
	_, _, place := collectionOf(obj.Type()) // an optional let holds no collection, as an optional field (DECISIONS 411, 510)
	m := &amending{layer: layer, a: a, root: obj, holder: objSite(obj), path: r.pathText(a), keys: map[*syntax.AmendSegment]value.Value{}, visit: rootVisit, place: place}
	m.dep = &depCtx{field: obj.Type(), at: a.Value}
	if r.locate(v, obj.Type(), m); r.failed {
		return nil
	}
	if m.rhs = r.rhs(m); m.rhs == nil {
		return nil
	}
	return r.setPath(v, obj.Type(), a.Path, m)
}

// rhs is the amendment's value, evaluated where its path lands, in layer provenance (EVALUATION.md §9.3).
func (r *run) rhs(m *amending) value.Value {
	outer := r.dep
	r.dep = m.dep
	v := r.evalAt(m.a.Value, m.located())
	r.dep = outer
	if v == nil {
		return nil
	}
	return r.ev.reprov(v, &value.Prov{Kind: value.ProvLayer, Span: r.span(m.a.Value), Ext: &value.ProvExt{Layer: m.layer}})
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
		r.unreached(m, diag.E1905.AtNone(r.span(seg), m.path))
		return nil
	}
	if keyedColl(cur) || isPlainList(cur) {
		return r.setElement(cur, t, segs, m)
	}
	r.unreached(m, diag.E1905.AtField(r.span(seg), m.path))
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
		r.unreached(m, r.noField(rec, seg, m))
		return nil
	}
	f, self, inColl := fieldsOf(rec.T)[i], m.visit, m.inColl
	outer, rt := m.holder, recordOf(rec.T)
	m.inColl, m.place = false, rt != nil && r.ev.fieldColl(rt, f) != nil
	m.at, m.holder = append(m.at, rec), r.ev.fieldSite(f, rec.T)
	defer func() { m.at, m.holder = m.at[:len(m.at)-1], outer }()
	nv := r.descend(m, i, func() value.Value { return r.next(rec.Fields[i], f.Type, segs, m) })
	if nv == nil {
		return nil
	}
	if f.Stable && !value.Equal(rec.Fields[i], nv) {
		return r.forbid(m, "", f.Name)
	}
	cp := *rec
	cp.Fields, cp.Set = append([]value.Value(nil), rec.Fields...), append([]bool(nil), rec.Set...)
	if !inColl { // an amended copy of an entry held outside its collection is a record of its own (EVALUATION.md §4.2; DECISIONS 510)
		cp.Ident = nil
	}
	if len(segs) == 1 { // a field the amendment writes holds its own entries (DECISIONS 478)
		nv = r.ev.inField(nv, &cp, f)
	}
	cp.Fields[i], cp.Set[i] = nv, true
	r.moving().replaces(&cp, rec)
	r.moved(rec, &cp)
	r.ev.shareParams(rec, &cp) // its re-derived defaults read the same arguments (TYPES.md §11.1)
	r.ev.unbindFrom(&cp, m.at, r.mv)
	if !r.ev.isCopy(rec, self, r.mv) && !r.derive(&cp, i, m.pathAfter(len(m.a.Path)-len(segs))) { // a copy applied none of its defaults (DECISIONS 486)
		return nil
	}
	r.ev.bindTo(&cp, rec, r.mv, cp.Fields[i]) // EVALUATION.md §3.4: the copy is the instance its refs resolve against
	return r.copied(rec, &cp)
}

// descend is eval with m's path at child index i of the value it has reached (copies.go).
func (r *run) descend(m *amending, i int, eval func() value.Value) value.Value {
	outer := m.visit
	m.visit = r.moving().copies.child(outer, i)
	defer func() { m.visit = outer }()
	return eval()
}

// noField is E1905 for a field the value lacks: a case without it, or none at all.
func (r *run) noField(rec *value.Record, seg *syntax.AmendSegment, m *amending) *diag.Builder {
	if ct, ok := rec.T.Base().(*types.CaseType); ok && seg.Name != nil {
		return diag.E1905.AtCaseField(r.span(seg), m.path, ct.Name, seg.Name.Name)
	}
	return diag.E1905.AtField(r.span(seg), m.path)
}

// next is the new value at a segment: the amendment's value converted at its storage point,
// related to its holder, when it is the last, else the rest of the path applied to the current value.
func (r *run) next(cur value.Value, t types.Type, segs []*syntax.AmendSegment, m *amending) value.Value {
	if len(segs) == 1 {
		nv := r.store(m.rhs, t, m.holder, m.pathAfter(len(m.a.Path)))
		if field, table, found := stableChange(cur, nv, t); found {
			return r.forbid(m, tableName(table, m), field)
		}
		r.ev.remember(nv, cur)
		return nv
	}
	if cur == nil || isNone(cur) {
		r.unreached(m, diag.E1905.AtNone(r.span(segs[0]), m.path))
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

// derive re-evaluates, in declaration order, the later fields from rec's own defaults (not written,
// spread or amended) that read a field changed marks, i and each one re-derived; the others keep theirs.
func (r *run) derive(rec *value.Record, i int, at *vpath) bool {
	fields := fieldsOf(rec.T)
	changed := make([]bool, len(fields))
	changed[i] = true
	for j, f := range fields {
		if j <= i || rec.Set[j] || f.Default == nil || f.Input != nil || spreadCopied(rec.Fields[j]) {
			continue
		}
		if changed[j] = r.ev.readsChanged(f, fields, changed); !changed[j] {
			continue
		}
		v := r.defaultValue(rec, f, at.field(f.Name), omitter(rec, j))
		if v == nil {
			return false
		}
		rec.Fields[j] = r.ev.inField(v, rec, f)
	}
	return true
}

// spreadCopied reports a field value a spread copied: written for §9.3 step 3 (EVALUATION.md).
func spreadCopied(v value.Value) bool {
	if v == nil {
		return false
	}
	p := v.Prov()
	return p != nil && p.Kind == value.ProvSpread
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
