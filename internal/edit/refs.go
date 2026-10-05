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
	res, err := s.reopen(r)
	if err != nil {
		return nil, err
	}
	tg, ok := s.target(res)
	if !ok {
		return nil, &PathError{Seg: len(r.Steps) - 1, Err: ErrBadOp}
	}
	refs, covers, err := s.valueRefs(ctx, tg)
	if err != nil {
		return nil, err
	}
	refs = append(refs, s.nameRefs(tg, covers)...)
	slices.SortFunc(refs, func(a, b Ref) int { return s.refOrder(a, b) })
	return refs, nil
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
	return &target{ident: rec.Ident, objects: s.entryObjects(rec.Ident), s: s, paths: map[*types.Collection]bool{}}, true
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
		return rt.Target == tg.ident.Coll && x.Owner == tg.ident.Owner || rt.Target != tg.ident.Coll && tg.names(rt.Target)
	case *value.Member:
		return tg.member != nil && x.Enum == tg.member.Enum && x.Index == tg.member.Index
	}
	return false
}

// names reports coll naming the target's collection: that collection, or a let path to the
// record field instance holding the target (DECISIONS 315, 316), resolved once.
func (tg *target) names(coll *types.Collection) bool {
	switch {
	case tg.ident == nil || coll == nil:
		return false
	case coll == tg.ident.Coll:
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
	p := Path{Package: coll.Pkg, Root: coll.Name}
	for _, f := range coll.FieldPath {
		p.Segs = append(p.Segs, Seg{Kind: SegField, Name: f})
	}
	res, err := Resolve(tg.s, p)
	if err != nil {
		return false
	}
	var entries []value.Value
	switch c := res.Target.(type) {
	case *value.List:
		entries = c.Elems
	case *value.Table:
		for _, e := range c.Entries {
			entries = append(entries, e)
		}
	}
	return slices.ContainsFunc(entries, func(e value.Value) bool {
		rec, ok := e.(*value.Record)
		return ok && rec.Ident != nil && rec.Ident.Coll == tg.ident.Coll && rec.Ident.Owner == tg.ident.Owner
	})
}
