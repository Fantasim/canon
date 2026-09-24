package eval

import (
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// letValue evaluates a let with its entries, at its declared type (TYPES.md §9.3).
func (r *run) letValue(st *rootState, d *syntax.LetDecl) value.Value {
	obj := st.obj
	at := rootPath(obj.Name())
	coll := r.ev.letColl(obj)
	if coll != nil {
		r.coll = &collHint{c: coll, at: at}
	}
	v := r.evalAt(d.Value, at)
	r.coll = nil
	if v = r.addEntries(st, v, coll, at); v == nil {
		return nil
	}
	v = r.store(v, obj.Type(), declSite(obj.File(), d.Name, d.Type), at)
	if v != nil && coll != nil {
		v = r.ev.adopt(v, coll, nil)
	}
	return r.applyLayers(obj, v)
}

// addEntries appends the entries `entry t.k { … }` declares (TYPES.md §9.3, DECISIONS 185).
func (r *run) addEntries(st *rootState, v value.Value, coll *types.Collection, at *vpath) value.Value {
	decls := r.ev.index.entries[st.obj]
	if v == nil || len(decls) == 0 {
		return v
	}
	var added []*value.Record
	for _, eo := range decls {
		d, ok := eo.Decl().(*syntax.EntryDecl)
		if !ok || r.ev.broken(eo) {
			r.stop()
			return nil
		}
		rec := r.entryDecl(d, eo.File(), st.obj.Type(), coll, at)
		if rec == nil {
			return nil
		}
		added = append(added, rec)
	}
	return appendEntries(v, added)
}

// entryDecl builds one declared entry, in its own file; a keyed list's key field is its key.
func (r *run) entryDecl(d *syntax.EntryDecl, file *syntax.File, t types.Type, coll *types.Collection, at *vpath) *value.Record {
	elem, keyed, _ := collectionOf(t)
	saved := r.fr
	r.fr = r.ev.rootFrame(file)
	r.fr.caller = saved.caller
	defer func() { r.fr = saved }()
	if !r.step(d.Value) {
		return nil
	}
	k := entryKey(d.Key)
	ident := &value.Identity{Coll: coll, Key: k, Retired: d.Mods != nil && d.Mods.Retired.Valid()}
	rec, ok := r.recordLit(d.Value, d, elem, ident, at.entry(k)).(*value.Record)
	if !ok {
		return nil
	}
	if keyed != nil && keyed.Index < len(rec.Fields) {
		kv := r.keyFieldValue(keyed, d.Key)
		if kv == nil {
			return nil
		}
		rec.Fields[keyed.Index], rec.Set[keyed.Index] = kv, true
		rec.Ident.Key, _ = std.KeyOf(kv)
	}
	return rec
}

// entryKey is the key an entry declaration writes: a word or an integer.
func entryKey(k syntax.EntryKey) value.Key {
	switch x := k.(type) {
	case *syntax.Ident:
		return value.Key{S: x.Name}
	case *syntax.IntLit:
		return value.Key{I: x.Value.Int64(), IsInt: true}
	}
	return value.Key{}
}

// keyFieldValue is a keyed-list element's key field its entry key gives (TYPES.md §9.3).
func (r *run) keyFieldValue(f *types.Field, key syntax.EntryKey) value.Value {
	k := entryKey(key)
	p := r.prov(key, value.ProvLiteral)
	switch b := f.Type.Base().(type) {
	case *types.EnumType:
		for i, m := range b.Members {
			if m.Name == k.S {
				return &value.Member{Enum: b, Index: i, P: p}
			}
		}
	case *types.RefType:
		return &value.Ref{T: f.Type, Key: k, P: p}
	case types.Basic:
		if k.IsInt {
			return &value.Int{V: k.I, T: f.Type, P: p}
		}
		return &value.Str{V: k.S, T: f.Type, P: p}
	}
	r.bug(key)
	return nil
}

func appendEntries(v value.Value, added []*value.Record) value.Value {
	switch x := v.(type) {
	case *value.Table:
		return &value.Table{T: x.T, Entries: append(append([]*value.Record(nil), x.Entries...), added...), P: x.P}
	case *value.List:
		elems := append([]value.Value(nil), x.Elems...)
		for _, a := range added {
			elems = append(elems, a)
		}
		return &value.List{T: x.T, Elems: elems, P: x.P}
	}
	return v
}
