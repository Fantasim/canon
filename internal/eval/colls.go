package eval

import (
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// collKey identifies a collection the checker interned (TYPES.md §10.2).
type collKey struct {
	kind            types.CollKind
	pkg, name, path string
	owner           *types.RecordType
}

// collHint is the collection a let's own literal builds: its entries take its identity.
type collHint struct {
	c  *types.Collection
	at *vpath
}

// interned indexes the collections the checker made, found in the types it recorded: an
// entry and a ref are equal only when their collection pointers are (value.Equal).
func (e *Evaluator) interned() map[collKey]*types.Collection {
	if e.colls != nil {
		return e.colls
	}
	e.colls = map[collKey]*types.Collection{}
	add := func(c *types.Collection) {
		k := keyOf(c)
		if _, ok := e.colls[k]; !ok {
			e.colls[k] = c
		}
	}
	if e.info == nil {
		return e.colls
	}
	for _, t := range e.info.TypeExprs { //canon:unordered interned pointers, one per key
		refsIn(t, add)
	}
	for _, t := range e.info.Types { //canon:unordered interned pointers, one per key
		refsIn(t, add)
	}
	for _, c := range e.info.Keys { //canon:unordered interned pointers, one per key
		add(c)
	}
	return e.colls
}

func keyOf(c *types.Collection) collKey {
	return collKey{kind: c.Kind, pkg: c.Pkg, name: c.Name, path: strings.Join(c.FieldPath, dot), owner: c.Owner}
}

// refsIn calls add on the target of every ref in t's lists, maps, optionals, pairs and functions.
func refsIn(t types.Type, add func(*types.Collection)) {
	switch x := t.(type) {
	case *types.RefType:
		add(x.Target)
	case *types.Refined:
		refsIn(x.Of, add)
	case *types.ListType:
		refsIn(x.Elem, add)
	case *types.OptionalType:
		refsIn(x.Elem, add)
	case *types.MapType:
		refsIn(x.Key, add)
		refsIn(x.Value, add)
	case *types.DepMapType:
		add(x.Coll)
	case *types.PairType:
		refsIn(x.A, add)
		refsIn(x.B, add)
	case *types.FuncType:
		for _, p := range x.Params {
			refsIn(p, add)
		}
		refsIn(x.Result, add)
	}
}

// letColl is the collection a table or keyed-list let is: the checker's when it made one.
func (e *Evaluator) letColl(obj check.Object) *types.Collection {
	elem, keyed, ok := collectionOf(obj.Type())
	if !ok {
		return nil
	}
	for _, kind := range []types.CollKind{types.CollLet, types.CollDefines} {
		if c := e.interned()[collKey{kind: kind, pkg: obj.Pkg(), name: obj.Name()}]; c != nil {
			return c
		}
	}
	c := e.collection(&types.Collection{Kind: types.CollLet, Pkg: obj.Pkg(), Name: obj.Name(), Elem: elem, KeyedBy: keyed})
	e.colls[keyOf(c)] = c
	return c
}

// fieldColl is the collection field f of record type rt holds, for its level-1 refs.
func (e *Evaluator) fieldColl(rt *types.RecordType, f *types.Field) *types.Collection {
	if c, done := e.fieldColls[f]; done {
		return c
	}
	elem, keyed, ok := collectionOf(f.Type)
	if !ok {
		e.fieldColls[f] = nil
		return nil
	}
	var c *types.Collection
	for key, ic := range e.interned() { //canon:unordered at most one collection per owner and field
		if key.kind == types.CollField && key.owner == rt && key.path == f.Name {
			c = ic
		}
	}
	if c == nil {
		c = e.collection(&types.Collection{
			Kind: types.CollField, Pkg: rt.Pkg, Name: f.Name, Owner: rt,
			FieldPath: []string{f.Name}, Elem: elem, KeyedBy: keyed,
		})
	}
	e.fieldColls[f] = c
	return c
}

// owned are the collections whose level-1 refs an instance of t binds.
func (e *Evaluator) owned(t types.Type) map[*types.Collection]bool {
	rt := recordOf(t)
	if rt == nil {
		return nil
	}
	if out, done := e.ownedBy[rt]; done {
		return out
	}
	var out map[*types.Collection]bool
	for key, c := range e.interned() { //canon:unordered a set
		if key.kind == types.CollField && key.owner == rt {
			if out == nil {
				out = map[*types.Collection]bool{}
			}
			out[c] = true
		}
	}
	e.ownedBy[rt] = out
	return out
}

func recordOf(t types.Type) *types.RecordType {
	switch x := t.Base().(type) {
	case *types.RecordType:
		return x
	case *types.AppliedRecord:
		return x.Rec
	}
	return nil
}

// collectionOf is the element and key field of a table or keyed-list type.
func collectionOf(t types.Type) (types.Type, *types.Field, bool) {
	switch x := t.Base().(type) {
	case *types.TableType:
		return x.Elem, nil, true
	case *types.ListType:
		return x.Elem, x.KeyedBy, x.KeyedBy != nil
	}
	return nil, nil, false
}

// inField gives a collection field's entries the identity of rec's field (TYPES.md §10.2).
func (e *Evaluator) inField(v value.Value, rec *value.Record, f *types.Field) value.Value {
	rt := recordOf(rec.T)
	if rt == nil {
		return v
	}
	if c := e.fieldColl(rt, f); c != nil {
		return e.adopt(v, c, rec)
	}
	return v
}

// adopt gives the entries of a table or keyed list the identity of collection c (owned by
// owner for a field), copying the entries that have another.
func (e *Evaluator) adopt(v value.Value, c *types.Collection, owner *value.Record) value.Value {
	switch x := v.(type) {
	case *value.Table:
		entries := make([]*value.Record, len(x.Entries))
		changed := false
		for i, en := range x.Entries {
			entries[i] = e.adoptEntry(en, c, owner)
			changed = changed || entries[i] != en
		}
		if changed {
			return e.carry(v, &value.Table{T: x.T, Entries: entries, P: x.P})
		}
	case *value.List:
		elems := make([]value.Value, len(x.Elems))
		changed := false
		for i, el := range x.Elems {
			rec, ok := el.(*value.Record)
			elems[i] = el
			if ok && rec.Ident != nil {
				elems[i] = e.adoptEntry(rec, c, owner)
			}
			changed = changed || elems[i] != el
		}
		if changed {
			return e.carry(v, &value.List{T: x.T, Elems: elems, P: x.P})
		}
	}
	return v
}

func (e *Evaluator) adoptEntry(en *value.Record, c *types.Collection, owner *value.Record) *value.Record {
	if en.Ident == nil || en.Ident.Coll == c && en.Ident.Owner == owner {
		return en
	}
	return e.withIdentity(en, &value.Identity{Coll: c, Owner: owner, Key: en.Ident.Key, Retired: en.Ident.Retired})
}

// withIdentity is rec as an element of a collection: the instance itself when it has no
// identity yet, so it stays one instance (DECISIONS 79, 195), else a copy carrying its marks.
func (e *Evaluator) withIdentity(rec *value.Record, id *value.Identity) *value.Record {
	if rec.Ident == nil {
		rec.Ident = id
		e.gens.retagged++
		e.retags.note(rec)
		return rec
	}
	cp := *rec
	cp.Ident = id
	e.carry(rec, &cp)
	e.copiedFrom(rec, &cp)
	return &cp
}

// fieldSite is the declaration `label: String(1..)` of field f of t (ERRORS.md §1.5).
func (e *Evaluator) fieldSite(f *types.Field, t types.Type) site {
	if s, ok := e.sites[f]; ok {
		return s
	}
	var s site
	file := e.declFile(t)
	for _, it := range fieldDecls(t) {
		if d, ok := it.(*syntax.FieldDecl); ok && d.Name.Name == f.Name && file != nil && d.Type != nil {
			s = site{decl: file.Span(d.Name).Cover(file.Span(d.Type)), has: true}
		}
	}
	e.sites[f] = s
	return s
}

// fieldDecls are the body items of a record or of a case's declaration.
func fieldDecls(t types.Type) []syntax.RecordItem {
	if rt := recordOf(t); rt != nil && rt.Decl != nil && rt.Decl.Body != nil {
		return rt.Decl.Body.Items
	}
	ct, ok := t.Base().(*types.CaseType)
	if !ok || ct.Variant.Decl == nil {
		return nil
	}
	for _, it := range ct.Variant.Decl.Items {
		if c, isCase := it.(*syntax.VariantCase); isCase && c.Name.Name == ct.Name && c.Body != nil {
			return c.Body.Items
		}
	}
	return nil
}
