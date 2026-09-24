package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// evalList is `[a, b, …]`; against a keyed list its elements take identities (TYPES.md §9.1).
func evalList(r *run, e syntax.Expr, at *vpath) value.Value {
	x := e.(*syntax.ListLit)
	t := r.typeOf(e)
	elems := make([]value.Value, len(x.Elems))
	for i, el := range x.Elems {
		if elems[i] = r.evalAt(el, at.index(i)); elems[i] == nil {
			return nil
		}
	}
	p := r.prov(e, value.ProvLiteral)
	if lt, ok := t.Base().(*types.ListType); ok && lt.KeyedBy != nil {
		return r.keyedList(elems, t, r.hint(at), p, at)
	}
	return &value.List{T: t, Elems: elems, P: p}
}

// hint is the collection of the let whose literal is at at, nil for any other literal.
func (r *run) hint(at *vpath) *types.Collection {
	if r.coll != nil && r.coll.at == at && at != nil {
		return r.coll.c
	}
	return nil
}

// evalListComp is `[elem clauses]`: the element for each run of the innermost clause.
func evalListComp(r *run, e syntax.Expr, _ *vpath) value.Value {
	x := e.(*syntax.ListComp)
	var out []value.Value
	ok := r.clauses(x.Clauses, func() bool {
		v := r.eval(x.Elem)
		out = append(out, v)
		return v != nil
	})
	if !ok {
		return nil
	}
	return &value.List{T: r.typeOf(e), Elems: out, P: r.prov(e, value.ProvComputed)}
}

// clauses runs comprehension clauses as nested loops (EVALUATION.md §2.2); false: aborted.
func (r *run) clauses(cs []*syntax.CompClause, yield func() bool) bool {
	if len(cs) == 0 {
		return yield()
	}
	c, rest := cs[0], cs[1:]
	switch c.Keyword {
	case syntax.KwFor:
		coll := r.eval(c.X)
		if coll == nil {
			return false
		}
		return r.each(c, coll, c.Vars, func() bool { return r.clauses(rest, yield) }) != flowAbort
	case syntax.KwIf:
		yes, ok := r.truth(c.X)
		if !ok {
			return false
		}
		return !yes || r.clauses(rest, yield)
	case syntax.KwLet:
		v := r.eval(c.X)
		if v == nil || len(c.Vars) == 0 || !r.bind(c.Vars[0], v) {
			return false
		}
		return r.clauses(rest, yield)
	default:
	}
	return false
}

// bind binds a declared name in the current frame.
func (r *run) bind(id *syntax.Ident, v value.Value) bool {
	obj := r.ev.info.Defs[id]
	if obj == nil {
		r.bug(id)
		return false
	}
	r.fr.vars[obj] = v
	return true
}

// tableLit is a table literal, each entry an instance with its identity (TYPES.md §6.3).
func (r *run) tableLit(lit *syntax.BraceLit, at *vpath) value.Value {
	t := r.typeOf(lit)
	tt, ok := t.Base().(*types.TableType)
	if !ok {
		r.bug(lit)
		return nil
	}
	coll := r.hint(at)
	if coll == nil {
		coll = &types.Collection{Kind: types.CollLet, Elem: tt.Elem}
	}
	entries := make([]*value.Record, 0, len(lit.Items))
	for _, it := range lit.Items {
		ent, isEntry := it.(*syntax.EntryItem)
		if !isEntry || !r.step(ent.Value) {
			r.bug(it)
			return nil
		}
		k := value.Key{S: ent.Key.Name}
		ident := &value.Identity{Coll: coll, Key: k, Retired: ent.Mods != nil && ent.Mods.Retired.Valid()}
		rec, isRec := r.recordLit(ent.Value, ent, tt.Elem, ident, at.entry(k)).(*value.Record)
		if !isRec {
			return nil
		}
		entries = append(entries, rec)
	}
	return &value.Table{T: t, Entries: entries, P: r.prov(lit, value.ProvLiteral)}
}

// mapLit is a map literal, keys and values in the order written.
func (r *run) mapLit(lit *syntax.BraceLit, at *vpath) value.Value {
	m := &value.Map{T: r.typeOf(lit), P: r.prov(lit, value.ProvLiteral)}
	for _, it := range lit.Items {
		k, kn, x := r.mapItem(it, mapKeyType(m.T))
		if k == nil {
			return nil
		}
		v := r.evalAt(x, at.key(mapKey(k)))
		if v == nil {
			return nil
		}
		r.put(m, k, v, kn, at)
	}
	return m
}

// mapItem is a map literal item's key, the node it is written at and its value (TYPES.md §5.2).
func (r *run) mapItem(it syntax.BraceItem, kt types.Type) (value.Value, syntax.Node, syntax.Expr) {
	switch x := it.(type) {
	case *syntax.MapItem:
		return r.eval(x.Key), x.Key, x.Value
	case *syntax.FieldItem:
		if !r.step(x.Name) {
			return nil, nil, nil
		}
		return r.identKey(x.Name, kt), x.Name, x.Value
	}
	r.bug(it)
	return nil, nil, nil
}

// identKey is the key `name:` denotes in a map literal (TYPES.md §4.1, §5.2).
func (r *run) identKey(n *syntax.Ident, kt types.Type) value.Value {
	p := r.prov(n, value.ProvLiteral)
	obj := r.ev.info.NameUses[n]
	if obj == nil {
		if d, isDep := unwrapOptional(kt).Base().(*types.DepUnionType); isDep {
			return &value.Symbol{Name: n.Name, T: d, P: p}
		}
		return &value.Ref{T: unwrapOptional(kt), Key: value.Key{S: n.Name}, P: p}
	}
	switch obj.Kind() {
	case check.ObjEntry:
		return &value.Ref{T: unwrapOptional(kt), Key: value.Key{S: obj.Name()}, P: p}
	case check.ObjCase:
		if k, isKind := kt.Base().(*types.VariantKindType); isKind {
			return &value.CaseKind{T: k, Index: obj.Type().(*types.CaseType).Index, P: p}
		}
	default:
	}
	return r.objectValue(obj, &syntax.IdentExpr{Bounds: n.Bounds, Name: n.Name})
}

// mapKeyType is the key type of a map or dependent map type.
func mapKeyType(t types.Type) types.Type {
	switch m := t.Base().(type) {
	case *types.MapType:
		return m.Key
	case *types.DepMapType:
		return &types.RefType{Target: m.Coll}
	}
	return types.AnyType
}

// mapComp is `{k: v clauses}`.
func (r *run) mapComp(lit *syntax.BraceLit) value.Value {
	m := &value.Map{T: r.typeOf(lit), P: r.prov(lit, value.ProvComputed)}
	mi, ok := lit.Items[0].(*syntax.MapItem)
	if !ok {
		r.bug(lit)
		return nil
	}
	done := r.clauses(lit.Clauses, func() bool {
		k := r.eval(mi.Key)
		v := r.eval(mi.Value)
		if k != nil && v != nil {
			r.put(m, k, v, mi.Key, nil)
		}
		return !r.failed
	})
	if !done {
		return nil
	}
	return m
}

// put adds a map entry; a key given twice is E3322 (TYPES.md §5.2, DECISIONS 185).
func (r *run) put(m *value.Map, k, v value.Value, keyNode syntax.Node, at *vpath) {
	if _, dup := m.Get(k); dup {
		r.emit(diag.E3322.At(r.span(keyNode), k).Path(at.String()))
		r.ev.MarkInvalid(m)
		return
	}
	m.Keys, m.Vals = append(m.Keys, k), append(m.Vals, v)
}

// each runs body once per element of a snapshot of coll (EVALUATION.md §2.2).
func (r *run) each(n syntax.Node, coll value.Value, names []*syntax.Ident, body func() bool) flow {
	next := iterator(coll)
	for {
		vals, more := next()
		if !more {
			return flowNext
		}
		if !r.step(n) || !r.bindAll(names, vals) {
			return flowAbort
		}
		if !body() {
			if r.failed {
				return flowAbort
			}
			return flowBreak
		}
	}
}

// bindAll binds the names of a loop: one value, or the two halves of a pair.
func (r *run) bindAll(names []*syntax.Ident, vals []value.Value) bool {
	if len(names) == pairNames && len(vals) == 1 {
		p, ok := vals[0].(*value.Pair)
		if !ok {
			r.bug(nil)
			return false
		}
		vals = []value.Value{p.A, p.B}
	}
	for i, id := range names {
		if i >= len(vals) || !r.bind(id, vals[i]) {
			r.bug(nil)
			return false
		}
	}
	return true
}

// iterator steps through a snapshot of a collection, values being immutable.
func iterator(coll value.Value) func() ([]value.Value, bool) {
	i := 0
	switch c := coll.(type) {
	case *value.Map:
		return func() ([]value.Value, bool) {
			if i >= len(c.Keys) {
				return nil, false
			}
			i++
			return []value.Value{c.Keys[i-1], c.Vals[i-1]}, true
		}
	case *value.Range:
		n := c.Start
		return func() ([]value.Value, bool) {
			if c.HasEnd && n >= c.End {
				return nil, false
			}
			n++
			return []value.Value{&value.Int{V: n - 1, T: types.IntType, P: c.P}}, true
		}
	}
	elems := std.Elems(coll)
	return func() ([]value.Value, bool) {
		if i >= len(elems) {
			return nil, false
		}
		i++
		return []value.Value{elems[i-1]}, true
	}
}
