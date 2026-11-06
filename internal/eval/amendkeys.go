package eval

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// rekey is a line that changed a keyed-list element's key field: the line, where it writes that
// key, and the key the element's path named.
type rekey struct {
	m    *amending
	span source.Span
	want value.Value
}

// elementKey is a keyed-list element by its identity, which a copy of it keeps.
type elementKey struct {
	coll *types.Collection
	key  value.Key
}

// identityOf is rec's identity as an elementKey; false when it has none.
func identityOf(rec *value.Record) (elementKey, bool) {
	if rec == nil || rec.Ident == nil {
		return elementKey{}, false
	}
	return elementKey{coll: rec.Ident.Coll, key: rec.Ident.Key}, true
}

// noteKey records the line when it changed the key field of rec, rebuilt from was in keyed list cur; the first change's key is the one its path named (EVALUATION.md §9.3).
func (r *run) noteKey(cur, was value.Value, rec *value.Record, whole bool, m *amending) {
	key := keyOfList(cur)
	id, ok := identityOf(rec)
	if key == nil || !ok {
		return
	}
	old, now := fieldAt(was, key.Index), fieldAt(rec, key.Index)
	if old == nil || now == nil || sameKeyValue(old, now) {
		return
	}
	mv := r.moving()
	k := rekey{m: m, span: r.keySpan(whole, key.Name, m), want: old}
	if prev, held := mv.rekeyed[id]; held {
		k.want = prev.want
	}
	mv.rekeyed[id] = k
	mv.rekey = &k
}

// keySpan is where line m writes an element's key field: the field of a literal replacing it whole, parenthesized or not, else the line's value.
func (r *run) keySpan(whole bool, name string, m *amending) source.Span {
	if lit, ok := unparen(m.a.Value).(*syntax.BraceLit); ok && whole {
		for _, it := range lit.Items {
			if f, isField := it.(*syntax.FieldItem); isField && f.Name.Name == name {
				return r.span(f.Value)
			}
		}
	}
	return r.span(m.a.Value)
}

// unparen is e without the parentheses around it.
func unparen(e syntax.Expr) syntax.Expr {
	for {
		p, ok := e.(*syntax.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// keysKept is false, after a hard E1913 at the last line that changed it, when an element of a keyed list the let's lines rebuilt ends, every line applied, with another key than its path named (EVALUATION.md §9.3).
func (r *run) keysKept(v value.Value) bool {
	if r.mv == nil || r.mv.rekey == nil {
		return true
	}
	rec, now := r.rekeyedIn(v, map[value.Value]bool{})
	if rec == nil {
		return true
	}
	id, _ := identityOf(rec)
	k, ok := r.mv.rekeyed[id]
	if !ok {
		k = *r.mv.rekey
	}
	r.unreached(k.m, diag.E1913.At(k.span, k.m.path, now, k.want))
	return false
}

// rekeyedIn is the first element of a keyed list among v's fresh values whose key field is not its key, with that field.
func (r *run) rekeyedIn(v value.Value, done map[value.Value]bool) (*value.Record, value.Value) {
	if !r.mv.isFresh(v) || done[v] {
		return nil, nil
	}
	done[v] = true
	key := keyOfList(v)
	for _, c := range components(v) {
		if rec, now := otherKey(c, key); rec != nil {
			return rec, now
		}
		if rec, now := r.rekeyedIn(c, done); rec != nil {
			return rec, now
		}
	}
	return nil, nil
}

// otherKey is c, an element of a list keyed by key, and its key field when that field is not its key; nil otherwise.
func otherKey(c value.Value, key *types.Field) (*value.Record, value.Value) {
	rec, ok := c.(*value.Record)
	if key == nil || !ok || rec.Ident == nil {
		return nil, nil
	}
	now := fieldAt(rec, key.Index)
	if now == nil {
		return nil, nil
	}
	if k, _ := std.KeyOf(now); k == rec.Ident.Key {
		return nil, nil
	}
	return rec, now
}

// keyOfList is the key field of keyed list v's elements, nil for any other value.
func keyOfList(v value.Value) *types.Field {
	l, ok := v.(*value.List)
	if !ok || !std.Keyed(l) {
		return nil
	}
	lt, _ := l.T.Base().(*types.ListType)
	return lt.KeyedBy
}

// fieldAt is field i of record v, nil when v is no record or has none there.
func fieldAt(v value.Value, i int) value.Value {
	rec, ok := v.(*value.Record)
	if !ok || i >= len(rec.Fields) {
		return nil
	}
	return rec.Fields[i]
}

// sameKeyValue reports two key field values naming one key.
func sameKeyValue(a, b value.Value) bool {
	ka, _ := std.KeyOf(a)
	kb, _ := std.KeyOf(b)
	return ka == kb
}
