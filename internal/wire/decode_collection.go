package wire

import (
	"context"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// File is a load.dir file after its `at:` path; its stem keys a table entry (WIRE.md §6.5).
type File struct {
	Sel  Selection
	Stem string
	At   source.Span // where a bad or repeated stem is reported
}

// Dir reads load.dir's files as a list or keyed list in file order, or a table (WIRE.md §6.5).
func (d *Decoder) Dir(ctx context.Context, files []File, t types.Type) (value.Value, bool, error) {
	r := d.start(ctx, t)
	r.watch(d)
	var v value.Value
	switch x := t.Base().(type) {
	case *types.ListType:
		sels := make([]Selection, len(files))
		for i, f := range files {
			sels[i] = f.Sel
		}
		v = r.elements(sels, t, x, r.dirScope(), nil)
	case *types.TableType:
		v = r.dirTable(files, t, x)
	default:
		r.misuse(ErrNoWireType, t)
	}
	return r.result(v)
}

// dirScope is load.dir's list scope: Decoder.Field's unit and int, which each file's element takes (§4.1).
func (r *run) dirScope() wscope {
	sc := wscope{root: true, fr: r.rootFrame()}
	if f := r.d.Field; f != nil {
		sc.unit, sc.asInt = f.Unit, f.Enc == types.EncInt
	}
	return sc
}

// step is one level of where the decoder is: a record instance, one of its fields, or an
// element of a list, map or table.
type step struct {
	rec   *value.Record
	field string
	elem  bool
}

func (r *run) enter(rv *value.Record) { r.trail = append(r.trail, step{rec: rv}) }

func (r *run) leave() { r.trail = r.trail[:len(r.trail)-1] }

// element decodes an element, a map value or a table row, one level below its collection.
func (r *run) element(sel Selection, t types.Type, sc wscope) value.Value {
	r.trail = append(r.trail, step{elem: true})
	defer r.leave()
	return r.value(sel, t, sc)
}

// list is a JSON array, in order; a keyed list's elements carry their identity (WIRE.md §5.7).
func (r *run) list(sel Selection, t types.Type, sc wscope) value.Value {
	lt := t.Base().(*types.ListType)
	if sc.bits {
		if e, ok := lt.Elem.Base().(*types.EnumType); ok {
			return r.bits(sel, t, e)
		}
		r.misuse(ErrNoWireType, t)
		return nil
	}
	n := sel.Node
	if n.Kind != jsonsrc.Array {
		return r.mismatch(n, diag.KindArray, t)
	}
	sels := make([]Selection, len(n.Elems))
	for i := range sels {
		sels[i] = sel.child(i)
	}
	if v := r.elements(sels, t, lt, sc, prov(n)); v != nil {
		return v
	}
	return nil
}

// elements decodes each selection as an element of lt, every one even after a failure.
func (r *run) elements(sels []Selection, t types.Type, lt *types.ListType, sc wscope, p *value.Prov) *value.List {
	var coll *types.Collection
	var owner *value.Record
	if lt.KeyedBy != nil {
		coll, owner = r.collection(lt.Elem, lt.KeyedBy, sc.root)
	}
	l := &value.List{T: t, Elems: make([]value.Value, 0, len(sels)), P: p}
	ok := true
	for _, s := range sels {
		r.entryKey = lt.KeyedBy
		v := r.dirElement(s, lt.Elem, sc)
		r.entryKey = nil
		if v == nil {
			ok = false
			continue
		}
		if lt.KeyedBy != nil {
			identify(v, lt.KeyedBy, coll, owner)
		}
		l.Elems = append(l.Elems, v)
	}
	if lt.KeyedBy != nil && r.entryOf(lt.Elem, sc) {
		r.remember(l.Elems)
	}
	if !ok {
		return nil
	}
	return l
}

// identify gives a keyed-list element its identity: the collection and its key field's key.
func identify(v value.Value, key *types.Field, coll *types.Collection, owner *value.Record) {
	rv, ok := v.(*value.Record)
	if !ok || key.Index >= len(rv.Fields) {
		return
	}
	if k, ok := keyOfValue(rv.Fields[key.Index]); ok {
		rv.Ident = &value.Identity{Coll: coll, Owner: owner, Key: k}
	}
}

// table is a source-wire table: an object keyed by entry id, in file order (WIRE.md §5.7).
func (r *run) table(sel Selection, t types.Type, sc wscope) value.Value {
	tt, n := t.Base().(*types.TableType), sel.Node
	if n.Kind != jsonsrc.Object {
		return r.mismatch(n, diag.KindObject, t)
	}
	coll, owner := r.collection(tt.Elem, nil, sc.root)
	tv := &value.Table{T: t, P: prov(n)}
	ok := true
	root := n.Pointer() == ""
	for i := range n.Members {
		m := &n.Members[i]
		if root && m.Key == KeySchema {
			continue
		}
		if !value.IsWord(m.Key) {
			r.report(diag.E7114.At(m.KeySpan, m.Key), m.Value)
			ok = false
		}
		e, retired := r.row(m.Value, tt.Elem, sc.fr)
		if e == nil || !ok {
			ok = false
			continue
		}
		e.Ident = &value.Identity{Coll: coll, Owner: owner, Key: value.Key{S: m.Key}, Retired: retired}
		tv.Entries = append(tv.Entries, e)
	}
	if r.entryOf(tt.Elem, sc) {
		r.remember(recordValues(tv.Entries))
	}
	if !ok {
		return nil
	}
	return tv
}

// recordValues is a table's entries as values.
func recordValues(entries []*value.Record) []value.Value {
	out := make([]value.Value, len(entries))
	for i, e := range entries {
		out[i] = e
	}
	return out
}

// row decodes a table row: the entry's record, and whether `$retired` retires it.
func (r *run) row(n *jsonsrc.Node, t types.Type, fr *frame) (*value.Record, bool) {
	r.trail = append(r.trail, step{elem: true})
	defer r.leave()
	rw := &rowState{}
	return r.recordAt(n, t, fr, rw), rw.retired
}

// dirTable is a table of one entry per file, keyed by the file's stem (WIRE.md §6.5).
func (r *run) dirTable(files []File, t types.Type, tt *types.TableType) value.Value {
	coll, _ := r.collection(tt.Elem, nil, true)
	tv := &value.Table{T: t}
	first := map[string]source.Span{}
	ok := true
	for _, f := range files {
		if f.Sel.Star {
			r.misuse(ErrStar, t)
			return nil
		}
		stemOK := r.stem(f, first)
		e, retired := r.dirRow(f.Sel, tt.Elem)
		if e == nil || !stemOK {
			ok = false
			continue
		}
		e.Ident = &value.Identity{Coll: coll, Key: value.Key{S: f.Stem}, Retired: retired}
		tv.Entries = append(tv.Entries, e)
	}
	if r.entryOf(tt.Elem, wscope{root: true}) {
		r.remember(recordValues(tv.Entries))
	}
	if !ok {
		return nil
	}
	return tv
}

// stem checks a stem or `$id` key: an identifier (E7114) not used before (E3102, WIRE.md §6.5).
func (r *run) stem(f File, first map[string]source.Span) bool {
	at, dup := first[f.Stem]
	switch {
	case !value.IsWord(f.Stem):
		r.report(diag.E7114.At(f.At, f.Stem), nil)
	case dup:
		return r.soft(diag.E3102.AtKey(f.At, literal(f.Stem), at), nil) // kept: both entries stay
	default:
		first[f.Stem] = f.At
		return true
	}
	return false
}

// mapValue is a JSON object whose keys decode as the key type, in document order (WIRE.md §5.8).
func (r *run) mapValue(sel Selection, t types.Type, sc wscope) value.Value {
	mt := t.Base().(*types.MapType)
	return r.entries(sel, t, mt.Key, sc, func(value.Value) (types.Type, wscope) {
		return mt.Value, sc.inner()
	})
}

// depMap is keyed by refs into its collection; each value sees its key as the binder.
func (r *run) depMap(sel Selection, t types.Type, sc wscope) value.Value {
	dt := t.Base().(*types.DepMapType)
	return r.entries(sel, t, &types.RefType{Target: dt.Coll}, sc, func(k value.Value) (types.Type, wscope) {
		inner := sc.inner()
		inner.fr = sc.fr.bind(dt.Binder, k)
		return dt.Value, inner
	})
}

// valueOf is the type and scope of a map's value, which a dependent map's key selects.
type valueOf func(key value.Value) (types.Type, wscope)

// entries decodes an object as a map: each key, then its value; two keys with one text are E3317.
func (r *run) entries(sel Selection, t, kt types.Type, sc wscope, of valueOf) value.Value {
	n := sel.Node
	if n.Kind != jsonsrc.Object {
		return r.mismatch(n, diag.KindObject, t)
	}
	m := &value.Map{T: t, P: prov(n)}
	seen := map[string]string{}
	ok := true
	for i := range n.Members {
		mem := &n.Members[i]
		k := r.mapKey(mem, kt, sc.fr)
		keyOK := k != nil && r.distinct(seen, k, mem)
		vt, vsc := of(k)
		v := r.element(sel.child(i), vt, vsc)
		if v == nil || !keyOK {
			ok = false
			continue
		}
		m.Keys, m.Vals = append(m.Keys, k), append(m.Vals, v)
	}
	if !ok {
		return nil
	}
	return m
}

// distinct keeps k by its wire text: a text seen before is E3317 at the second (WIRE.md §5.8).
func (r *run) distinct(seen map[string]string, k value.Value, m *jsonsrc.Member) bool {
	text, err := keyOf(k)
	if err != nil {
		return true
	}
	if first, dup := seen[text]; dup {
		return r.soft(diag.E3317.At(m.KeySpan, literal(first), literal(m.Key), text), m.Value) // kept: both keys stay
	}
	seen[text] = m.Key
	return true
}

// collKey names a collection field: the record type holding it and its field path.
type collKey struct {
	owner *types.RecordType
	path  string
}

// collection is what the table or keyed list being decoded is, and the instance holding it:
// the Decoder's for the whole value, the field collection a ref of the type targets, else one
// of its own.
func (r *run) collection(elem types.Type, key *types.Field, root bool) (*types.Collection, *value.Record) {
	if root {
		if r.d.Coll != nil {
			return r.d.Coll, nil
		}
		return &types.Collection{Kind: types.CollLet, Elem: elem, KeyedBy: key}, nil
	}
	var names []string
	var inner *value.Record
	for j := len(r.trail) - 1; j >= 0 && !r.trail[j].elem; j-- {
		st := r.trail[j]
		if st.rec == nil {
			names = append([]string{st.field}, names...)
			continue
		}
		if c := r.colls[collKey{declOf(st.rec.T), strings.Join(names, pointSep)}]; c != nil {
			return c, st.rec
		}
		if inner == nil {
			inner = st.rec
		}
	}
	return &types.Collection{Kind: types.CollField, Elem: elem, KeyedBy: key}, inner
}

// owner is the nearest enclosing instance a ref into a field collection is bound to (RES-03).
func (r *run) owner(target *types.Collection) *value.Record {
	if target == nil || target.Kind != types.CollField {
		return nil
	}
	for _, st := range slices.Backward(r.trail) {
		if st.rec != nil && declOf(st.rec.T) == target.Owner {
			return st.rec
		}
	}
	return nil
}

// declOf is the record declaration of a record value's type; nil for a case.
func declOf(t types.Type) *types.RecordType {
	switch d := t.Base().(type) {
	case *types.RecordType:
		return d
	case *types.AppliedRecord:
		return d.Rec
	}
	return nil
}

// fieldCollections is each field collection a ref in t targets: its entries and refs share it.
func fieldCollections(t types.Type) map[collKey]*types.Collection {
	s := scan{out: map[collKey]*types.Collection{}, seen: map[types.Type]bool{}}
	s.visit(t)
	return s.out
}

type scan struct {
	out  map[collKey]*types.Collection
	seen map[types.Type]bool
}

func (s scan) visit(t types.Type) {
	if t == nil || s.seen[t] {
		return
	}
	s.seen[t] = true
	if rt, ok := t.(*types.RefType); ok && rt.Target != nil && rt.Target.Kind == types.CollField {
		s.out[collKey{rt.Target.Owner, strings.Join(rt.Target.FieldPath, pointSep)}] = rt.Target
	}
	for _, c := range components(t) {
		s.visit(c)
	}
}

// components are the types t is built from, down to fields and dependent branches.
func components(t types.Type) []types.Type {
	switch x := t.(type) {
	case *types.Alias:
		return []types.Type{x.Def}
	case *types.Refined:
		return []types.Type{x.Of}
	case *types.OptionalType:
		return []types.Type{x.Elem}
	case *types.ListType:
		return []types.Type{x.Elem}
	case *types.MapType:
		return []types.Type{x.Key, x.Value}
	case *types.DepMapType:
		return []types.Type{x.Value}
	case *types.TableType:
		return []types.Type{x.Elem}
	case *types.LitUnionType:
		return []types.Type{x.Of}
	case *types.AppliedRecord:
		return []types.Type{x.Rec}
	case *types.RecordType:
		return fieldTypes(x.Fields, nil)
	case *types.VariantType:
		var out []types.Type
		for _, c := range x.Cases {
			out = fieldTypes(c.Fields, out)
		}
		return out
	case *types.TypeAppType:
		return branches(x.Fn)
	}
	return nil
}

func fieldTypes(fields []*types.Field, out []types.Type) []types.Type {
	for _, f := range fields {
		out = append(out, f.Type)
	}
	return out
}

// branches are the types a type function may give.
func branches(fn *types.TypeFunc) []types.Type {
	out := []types.Type{fn.Body}
	for _, a := range fn.Arms {
		out = append(out, a.Result)
	}
	return out
}
