package edit

import (
	"cmp"
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// RefKind says how a reference names its target (API.md R7).
type RefKind uint8

// Ref is one place that references an entry or an enum member (R7). Path, for a value or a
// key, is the canonical path of the referring value or map entry without its package (P10).
type Ref struct {
	Kind    RefKind
	Package string
	Path    string
	Span    source.Span
}

// Refs lists the references to r's target in every package of s, in R8 order (API.md R7):
// values and map keys stated in a source, names in code, views, checks and amendments. r names
// a table entry, a keyed-list element or an enum member, else ErrBadOp; ErrForeign if not of s.
func (s *Snapshot) Refs(ctx context.Context, r Resolved) ([]Ref, error) {
	refs, _, err := s.refsOf(ctx, r)
	return refs, err
}

// refsOf is Refs, and the paths where a default's or a constant's token, which every instance
// taking it shares, names the target while another instance's names another entry: no rewrite
// of that token renames one instance's entry alone (API.md R7, E12; DECISIONS 483).
func (s *Snapshot) refsOf(ctx context.Context, r Resolved) ([]Ref, []string, error) {
	res, err := s.reopen(r)
	if err != nil {
		return nil, nil, err
	}
	tg, ok := s.target(res)
	if !ok {
		return nil, nil, &PathError{Seg: len(r.Steps) - 1, Err: ErrBadOp}
	}
	sc, err := s.valueRefs(ctx, tg)
	if err != nil {
		return nil, nil, err
	}
	refs, covers := sc.listed()
	names, shared := s.nameRefs(tg, covers, sc.uses)
	refs = append(refs, names...)
	slices.SortFunc(refs, func(a, b Ref) int { return s.refOrder(a, b) })
	return refs, shared, nil
}

// refOrder is R8: the F2 order of the spans (file, then position), then kind, package, path.
func (s *Snapshot) refOrder(a, b Ref) int {
	return cmp.Or(
		cmp.Compare(s.display(a.Span.File), s.display(b.Span.File)),
		cmp.Compare(a.Span.Start, b.Span.Start), cmp.Compare(a.Span.End, b.Span.End),
		cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Package, b.Package), cmp.Compare(a.Path, b.Path),
	)
}

// target is what Refs looks for: an entry's identity, or an enum member, and the objects
// that declare it, which names in code resolve to.
type target struct {
	ident   *value.Identity
	member  *value.Member
	objects map[check.Object]bool
	s       *Snapshot
	paths   map[*types.Collection]bool // the let-path collections met, naming the target's instance or not
	held    *heldHome                  // the optional record field holding the target, when its identity names no field collection
	res     resolution                 // the target's path, which an amendment's line is matched against
}

// heldHome is the optional record field holding a target, which takes no field identity (DECISIONS 411, 470).
type heldHome struct {
	owner *types.RecordType
	field string
}

// target is the entry, keyed-list element or member res names (API.md R7, P7a).
func (s *Snapshot) target(res resolution) (*target, bool) {
	if m, ok := res.Target.(*value.Member); ok && res.root.enum != nil {
		tg := &target{member: m, objects: map[check.Object]bool{}, s: s, paths: map[*types.Collection]bool{}}
		if d, isEnum := res.root.obj.Decl().(*syntax.EnumDecl); isEnum && m.Index < len(d.Members) {
			tg.objects[s.info.Defs[d.Members[m.Index].Name]] = true
		}
		return tg, true
	}
	n := len(res.Steps)
	rec, ok := res.Target.(*value.Record)
	if n == 0 || !ok || rec.Ident == nil || !keyedContainer(res.Steps[n-1].Container) {
		return nil, false
	}
	tg := &target{ident: rec.Ident, objects: s.entryObjects(rec.Ident), s: s, paths: map[*types.Collection]bool{}, res: res}
	tg.held = heldAt(res, rec.Ident)
	return tg, true
}

// heldAt is the optional record field holding res's target, nil unless the target is an element
// of a keyed list or table that field holds and its identity names no field collection.
func heldAt(res resolution, id *value.Identity) *heldHome {
	above := res.Steps[:len(res.Steps)-1] // the steps down to the collection holding the target
	if id.Coll != nil && id.Coll.Kind == types.CollField || len(above) == 0 || above[len(above)-1].Seg.Kind != SegField {
		return nil
	}
	field := above[len(above)-1]
	rt, isRec := typeKey(field.Container).(*types.RecordType)
	if !isRec {
		return nil
	}
	i := fieldIndex(rt.Fields, field.Seg.Name)
	if i < 0 {
		return nil
	}
	if _, opt := rt.Fields[i].Type.Underlying().(*types.OptionalType); !opt {
		return nil
	}
	return &heldHome{owner: rt, field: field.Seg.Name}
}

// is reports coll the record field's collection, as the checker records it for `.k` (DECISIONS 470).
func (h *heldHome) is(coll *types.Collection) bool {
	return h != nil && coll.Kind == types.CollField && coll.Owner == h.owner && slices.Equal(coll.FieldPath, []string{h.field})
}

// isEntry reports id the target entry's identity: its collection, owner and key, which a spread copy
// keeps and an optional field's entry holds with no owner (DECISIONS 410, 411, 473).
func (tg *target) isEntry(id *value.Identity) bool {
	return id != nil && tg.ident != nil && id.Coll == tg.ident.Coll && id.Owner == tg.ident.Owner && id.Key == tg.ident.Key
}

// keyedContainer reports a table or a keyed list: what holds entries and keyed elements.
func keyedContainer(t types.Type) bool {
	switch b := baseOf(t).(type) {
	case *types.TableType:
		return true
	case *types.ListType:
		return b.KeyedBy != nil
	}
	return false
}

// entryObjects are the declarations of an entry of a let's table, which static keys resolve
// to: the item of its literal and its `entry` declarations.
func (s *Snapshot) entryObjects(id *value.Identity) map[check.Object]bool {
	out := map[check.Object]bool{}
	if id.Coll == nil || id.Coll.Kind != types.CollLet || len(id.Coll.FieldPath) > 0 || id.Key.IsInt {
		return out
	}
	pkg := s.byPath[id.Coll.Pkg]
	if pkg == nil {
		return out
	}
	for _, obj := range pkg.Decls {
		if obj.Kind() == check.ObjLet && obj.Name() == id.Coll.Name {
			if e := entryItem(syntax.Unparen(initializer(obj.Decl())), id.Key.S); e != nil {
				out[s.info.Defs[e.Key]] = true
			}
		}
		if d, isEntry := obj.Decl().(*syntax.EntryDecl); isEntry && obj.Name() == id.Key.S && s.entryOf(d, id.Coll) {
			out[obj] = true
		}
	}
	return out
}

// entryOf reports an `entry` declaration of coll's let.
func (s *Snapshot) entryOf(d *syntax.EntryDecl, coll *types.Collection) bool {
	let := s.info.NameUses[d.Table]
	return let != nil && let.Pkg() == coll.Pkg && let.Name() == coll.Name
}

// isValue reports v is the target: a ref to the entry, or the member.
func (tg *target) isValue(v value.Value) bool {
	switch x := v.(type) {
	case *value.Ref:
		rt, ok := baseOf(x.T).(*types.RefType)
		if !ok || tg.ident == nil || x.Key != tg.ident.Key {
			return false
		}
		if rt.Target == tg.ident.Coll && x.Owner == tg.ident.Owner || rt.Target != tg.ident.Coll && tg.names(rt.Target) {
			return true
		}
		held := heldIdent(x)
		return held != nil && held.Coll == tg.ident.Coll && held.Owner == tg.ident.Owner
	case *value.Member:
		return tg.member != nil && x.Enum == tg.member.Enum && x.Index == tg.member.Index
	}
	return false
}

// heldIdent is the identity of the entry a bound level-1 ref names in the collection its
// instance's field holds, whatever identity that entry carries (API.md R7; DECISIONS 410); nil
// for another ref, or a key that names no entry there.
func heldIdent(x *value.Ref) *value.Identity {
	rt, ok := baseOf(x.T).(*types.RefType)
	if !ok || rt.Target == nil || rt.Target.Kind != types.CollField || x.Owner == nil || len(rt.Target.FieldPath) == 0 {
		return nil
	}
	i := fieldIndex(fieldsOf(x.Owner.T), rt.Target.FieldPath[0])
	if i < 0 || i >= len(x.Owner.Fields) {
		return nil
	}
	return keyedIdent(x.Owner.Fields[i], x.Key)
}

// keyedIdent is the identity of the element or entry keyed k in a keyed list or table, nil for none.
func keyedIdent(v value.Value, k value.Key) *value.Identity {
	switch c := v.(type) {
	case *value.List:
		for _, e := range c.Elems {
			if rec, isRec := e.(*value.Record); isRec && rec.Ident != nil && rec.Ident.Key == k {
				return rec.Ident
			}
		}
	case *value.Table:
		for _, rec := range c.Entries {
			if rec.Ident != nil && rec.Ident.Key == k {
				return rec.Ident
			}
		}
	}
	return nil
}

// names reports coll naming the target's collection: that collection, or a let path to the
// record field instance holding the target (DECISIONS 315, 316), resolved once.
func (tg *target) names(coll *types.Collection) bool {
	switch {
	case tg.ident == nil || coll == nil:
		return false
	case coll == tg.ident.Coll || tg.held.is(coll):
		return true
	case coll.Kind != types.CollLet || len(coll.FieldPath) == 0:
		return false
	}
	if known, ok := tg.paths[coll]; ok {
		return known
	}
	tg.paths[coll] = tg.instanceAt(coll)
	return tg.paths[coll]
}

// instanceAt reports the let path coll reaching the collection instance holding the target.
func (tg *target) instanceAt(coll *types.Collection) bool {
	return slices.ContainsFunc(letPathEntries(tg.s, coll), func(e value.Value) bool {
		rec, ok := e.(*value.Record)
		return ok && rec.Ident != nil && rec.Ident.Coll == tg.ident.Coll && rec.Ident.Owner == tg.ident.Owner
	})
}

// letPathEntries are the entries or elements of the collection a let path coll reaches
// (DECISIONS 315, 316), none when it reaches nothing.
func letPathEntries(s *Snapshot, coll *types.Collection) []value.Value {
	p := Path{Package: coll.Pkg, Root: coll.Name}
	for _, f := range coll.FieldPath {
		p.Segs = append(p.Segs, Seg{Kind: SegField, Name: f})
	}
	res, err := Resolve(s, p)
	if err != nil {
		return nil
	}
	switch c := res.Target.(type) {
	case *value.List:
		return c.Elems
	case *value.Table:
		entries := make([]value.Value, len(c.Entries))
		for i, e := range c.Entries {
			entries[i] = e
		}
		return entries
	}
	return nil
}
