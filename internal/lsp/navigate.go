package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// definition goes from a name to its declaration, and from the key of a ref to the entry it
// names, in a .canon source or a loaded file, open or not; null where neither is.
func (s *server) definition(ctx context.Context, params json.RawMessage) (any, error) {
	// IMPLEMENTATION-PLAN §8.4 Features, CLI.md §4
	sp, err := s.spotAt(ctx, params)
	if sp == nil {
		return nil, err
	}
	if path, at := sp.keyAt(); at != nil {
		r, ok := sp.resolve(path)
		if !ok || r.Target.Prov() == nil {
			return nil, nil
		}
		return sp.found(r.Target.Prov().Span)
	}
	obj, _ := sp.object()
	if obj == nil || obj.File() == nil || obj.Decl() == nil {
		return nil, nil
	}
	return sp.found(declared(sp.info, obj))
}

// found is a location's result: the location, or null for a span in no file.
func (sp *spot) found(at source.Span) (any, error) {
	if loc, ok := sp.location(at); ok {
		return loc, nil
	}
	return nil, nil
}

// declared is the span of the name declaring obj, its declaration's when no name does (an
// entry with an integer key).
func declared(info *check.Info, obj check.Object) source.Span {
	var name *syntax.Ident
	syntax.Inspect(obj.Decl(), func(n syntax.Node) bool {
		if id, ok := n.(*syntax.Ident); ok && name == nil && info.Defs[id] == obj {
			name = id
		}
		return name == nil
	})
	if name == nil {
		return obj.File().Span(obj.Decl())
	}
	return obj.File().Span(name)
}

type referenceParams struct {
	Context struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
}

// references is `canon refs` of the entry, keyed element or enum member at the position: the
// list Refs gives, in its order, after the target's own location with includeDeclaration; null
// at anything else. A reference that cannot be located fails it: a short list never looks whole.
func (s *server) references(ctx context.Context, params json.RawMessage) (any, error) {
	// API.md R7, R8; CLI.md §3.8, §4; IMPLEMENTATION-PLAN §8.4 Features
	var p referenceParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	sp, err := s.spotAt(ctx, params)
	if sp == nil {
		return nil, err
	}
	r, ok := sp.refsTarget()
	if !ok {
		return nil, nil
	}
	refs, err := sp.snap.Refs(ctx, r)
	if err != nil {
		return nil, refsError(err)
	}
	spans := make([]source.Span, 0, len(refs)+1)
	if at, ok := sp.declaredAt(r); ok && p.Context.IncludeDeclaration {
		spans = append(spans, at)
	}
	for _, ref := range refs {
		spans = append(spans, ref.Span)
	}
	return sp.locations(spans)
}

// locations are the spans' locations, in order; errNoFile names how many have no file.
func (sp *spot) locations(spans []source.Span) ([]location, error) {
	out := make([]location, 0, len(spans))
	for _, at := range spans {
		if loc, ok := sp.location(at); ok {
			out = append(out, loc)
		}
	}
	if missing := len(spans) - len(out); missing > 0 {
		return nil, fmt.Errorf(fmtNoFile, errRefs, missing, errNoFile)
	}
	return out, nil
}

// declaredAt is where a Refs target is declared: an entry's or element's value where it was
// written (in source or JSON), an enum member's name.
func (sp *spot) declaredAt(r edit.Resolved) (source.Span, bool) {
	m, isMember := r.Target.(*value.Member)
	switch {
	case isMember && m.Enum.Decl != nil && m.Index < len(m.Enum.Decl.Members):
		name := m.Enum.Decl.Members[m.Index].Name
		if obj := sp.info.Defs[name]; obj != nil && obj.File() != nil {
			return obj.File().Span(name), true
		}
	case r.Target.Prov() != nil:
		return r.Target.Prov().Span, true
	}
	return source.Span{}, false
}

// refsError is Refs failing (R7), as canon refs does: a value it would miss is named by its root.
func refsError(err error) error {
	var pe *edit.PathError
	if errors.As(err, &pe) && errors.Is(err, edit.ErrNoValue) {
		return fmt.Errorf(fmtRefsRoot, errRefs, edit.Path{Package: pe.Root.Pkg, Root: pe.Root.Name}, edit.ErrNoValue)
	}
	return fmt.Errorf(fmtWrap, errRefs, err)
}

// refsTarget is what Refs is asked about: the entry a ref's key names, else the entry or
// member the name at the position declares or names.
func (sp *spot) refsTarget() (edit.Resolved, bool) {
	if path, at := sp.keyAt(); at != nil {
		return sp.resolve(path)
	}
	obj, _ := sp.object()
	if obj == nil || !refsKinds[obj.Kind()] {
		return edit.Resolved{}, false
	}
	return sp.valueOf(obj)
}

// refsKinds are the names whose values Refs lists the references of (R7).
var refsKinds = map[check.ObjKind]bool{check.ObjEntry: true, check.ObjMember: true}

// valueOf is the value a name denotes, read as a value path reads it: a let's or a constant's
// root, an entry of a let's table, an enum member; false for any other name.
func (sp *spot) valueOf(obj check.Object) (edit.Resolved, bool) {
	read, ok := values[obj.Kind()]
	if !ok {
		return edit.Resolved{}, false
	}
	return read(sp, obj)
}

// values read the value of each kind of name that has one.
var values = map[check.ObjKind]func(*spot, check.Object) (edit.Resolved, bool){
	check.ObjLet:    (*spot).root,
	check.ObjConst:  (*spot).root,
	check.ObjEntry:  (*spot).entry,
	check.ObjMember: (*spot).member,
}

// root is a let's or a constant's value.
func (sp *spot) root(obj check.Object) (edit.Resolved, bool) {
	return sp.resolve(&edit.Path{Package: obj.Pkg(), Root: obj.Name()})
}

// member is an enum member, at its path `package:Enum.member`.
func (sp *spot) member(obj check.Object) (edit.Resolved, bool) {
	e, ok := obj.Type().Base().(*types.EnumType)
	if !ok {
		return edit.Resolved{}, false
	}
	return sp.resolve(&edit.Path{Package: e.Pkg, Root: e.Name, Segs: []edit.Seg{{Kind: edit.SegField, Name: obj.Name()}}})
}

// entry is the entry an entry object declares, in its let's table: the let an `entry`
// declaration names, or the let whose initializer holds it. An entry of a nested table is not
// one of the let's, and has no path.
func (sp *spot) entry(obj check.Object) (edit.Resolved, bool) {
	f := obj.File()
	if f == nil {
		return edit.Resolved{}, false
	}
	decl := f.Span(obj.Decl())
	var let check.Object
	key := value.Key{S: obj.Name()}
	switch d := obj.Decl().(type) {
	case *syntax.EntryDecl:
		let = sp.info.NameUses[d.Table]
		if n, isInt := d.Key.(*syntax.IntLit); isInt && n.Value.IsInt64() {
			key = value.Key{I: n.Value.Int64(), IsInt: true}
		}
	case *syntax.EntryItem:
		let = sp.letAround(f, decl)
	}
	if let == nil {
		return edit.Resolved{}, false
	}
	r, ok := sp.resolve(&edit.Path{Package: let.Pkg(), Root: let.Name(), Segs: []edit.Seg{keySeg(key)}})
	if !ok || r.Target.Prov() == nil || !inside(r.Target.Prov().Span, decl) {
		return edit.Resolved{}, false
	}
	return r, true
}

// letAround is the let of f whose declaration holds the span at, nil for none.
func (sp *spot) letAround(f *syntax.File, at source.Span) check.Object {
	for _, d := range f.Decls {
		if let, ok := d.(*syntax.LetDecl); ok && inside(at, f.Span(let)) {
			return sp.info.Defs[let.Name]
		}
	}
	return nil
}

// inside reports a within b, in the same file.
func inside(a, b source.Span) bool {
	return a.File == b.File && b.Start <= a.Start && a.End <= b.End
}
