package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// origin is the first structural path, in value order, reaching v from a root that can state it (API.md §7.2).
func (s *Snapshot) origin(v value.Value) string {
	anchor, ok := anchorOf(v)
	if !ok {
		return ""
	}
	roots, anchored := s.anchorRoots(anchor)
	for _, r := range roots {
		rv, err := s.force(r)
		if err != nil {
			continue
		}
		f := finder{target: v, anchor: anchor, anchored: anchored}
		f.accept = func(segs []Seg) bool {
			f.hit, f.done = s.structural(Path{Package: r.pkg.Path, Root: r.obj.Name(), Segs: segs})
			return f.done
		}
		if !f.is(rv) || !f.accept(nil) {
			f.visit(rv, nil)
		}
		if f.done {
			return f.hit
		}
	}
	return ""
}

// structural resolves p and reports its canonical form when its walk stays in a source tree.
func (s *Snapshot) structural(p Path) (string, bool) {
	res, err := s.open(p)
	if err != nil {
		return "", false
	}
	switch s.judge(res, OpSet, "").reason() {
	case ReasonComputed, ReasonFormat:
		return "", false
	default:
		return res.Canonical, true
	}
}

// anchorOf is where v's own source is stated: its literal or JSON value, its spread, or the
// literal that omitted a defaulted field.
func anchorOf(v value.Value) (source.Span, bool) {
	p := provOf(v)
	switch {
	case p == nil:
		return source.Span{}, false
	case p.Kind == value.ProvLiteral, p.Kind == value.ProvJSON, p.Kind == value.ProvSpread:
		return p.Span, true
	case p.Kind == value.ProvDefault && p.Via != nil:
		return p.Via.Span, true
	}
	return source.Span{}, false
}

// anchorRoots are the roots whose tree can hold anchor: the declaration of a .canon file that
// contains it (an entry's collection for an `entry`), or every root that loads a file for a data
// file; false when the anchor is in no root (a default in a type), so every root is searched.
func (s *Snapshot) anchorRoots(anchor source.Span) ([]rootRef, bool) {
	f := s.files[anchor.File]
	if f != nil {
		if r := s.declaring(f, anchor); r != nil {
			return r, true
		}
	}
	var out []rootRef
	for _, pkg := range s.pkgs {
		for _, obj := range pkg.Decls {
			if e := initializer(obj.Decl()); e != nil && (f != nil || loads(e)) {
				out = append(out, rootRef{pkg: pkg, obj: obj})
			}
		}
	}
	return out, f == nil
}

// declaring is the root whose declaration in f contains anchor.
func (s *Snapshot) declaring(f *syntax.File, anchor source.Span) []rootRef {
	for _, pkg := range s.pkgs {
		for _, obj := range pkg.Decls {
			if obj.File() != f || !contains(f.Span(obj.Decl()), anchor) {
				continue
			}
			if d, ok := obj.Decl().(*syntax.EntryDecl); ok {
				obj = s.info.NameUses[d.Table]
			}
			if obj != nil && initializer(obj.Decl()) != nil {
				return []rootRef{{pkg: s.byPath[obj.Pkg()], obj: obj}}
			}
		}
	}
	return nil
}

// loads reports an expression holding a load.
func loads(e syntax.Expr) bool {
	found := false
	syntax.Inspect(e, func(n syntax.Node) bool {
		if _, ok := n.(*syntax.LoadExpr); ok {
			found = true
		}
		return !found
	})
	return found
}

func contains(outer, inner source.Span) bool {
	return outer.File == inner.File && outer.Start <= inner.Start && inner.End <= outer.End
}

// finder collects the paths reaching target; anchored, it descends only into values whose
// source can hold anchor.
type finder struct {
	target   value.Value
	anchor   source.Span
	anchored bool
	accept   func([]Seg) bool // reports a path to the target that ends the search
	hit      string
	done     bool
}

// is reports v is the target, or a value passed along from it (EVALUATION.md §13).
func (f *finder) is(v value.Value) bool {
	return v == f.target || provOf(v) != nil && provOf(v) == provOf(f.target)
}

// visit walks v's children in value order until accept ends the search.
func (f *finder) visit(v value.Value, segs []Seg) {
	for _, c := range children(v) {
		at := append(slices.Clip(segs), c.seg)
		switch {
		case f.is(c.v):
			f.accept(at)
		case f.holds(c.v):
			f.visit(c.v, at)
		}
		if f.done {
			return
		}
	}
}

// holds reports a value whose source may contain the anchor: a literal or JSON value around
// it, or a collection with no provenance of its own (load.dir); unanchored, any stated value.
func (f *finder) holds(v value.Value) bool {
	p := provOf(v)
	switch {
	case v == nil:
		return false
	case p == nil:
		return true
	case p.Kind == value.ProvLiteral, p.Kind == value.ProvJSON:
		return !f.anchored || contains(p.Span, f.anchor)
	case p.Kind == value.ProvDefault, p.Kind == value.ProvSpread:
		return !f.anchored
	}
	return false
}

// child is one child of a value and its canonical segment.
type child struct {
	seg Seg
	v   value.Value
}

// children are a value's children in value order (API.md §5.2 Children), keyed canonically.
func children(v value.Value) []child {
	var out []child
	switch x := v.(type) {
	case *value.Record:
		for i, f := range fieldsOf(x.T) {
			if i < len(x.Fields) && x.Fields[i] != nil {
				out = append(out, child{Seg{Kind: SegField, Name: f.Name}, x.Fields[i]})
			}
		}
	case *value.List:
		lt, _ := x.T.Base().(*types.ListType)
		for i, e := range x.Elems {
			out = append(out, child{elemSeg(lt, e, i), e})
		}
	case *value.Table:
		for _, e := range x.Entries {
			out = append(out, child{entrySeg(e.Ident.Key), e})
		}
	case *value.Map:
		kt, _, _ := mapTypes(x.T)
		for i, k := range x.Keys {
			out = append(out, child{keySeg(k, kt), x.Vals[i]})
		}
	}
	return out
}

// elemSeg is listSeg for a list whose type may not be a list type.
func elemSeg(lt *types.ListType, e value.Value, i int) Seg {
	if lt == nil {
		return intSeg(int64(i))
	}
	return listSeg(lt, e, i)
}
