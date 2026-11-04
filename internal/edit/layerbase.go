package edit

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// layerActive reports layer x applied by s's analysis (EVALUATION.md §9.1).
func (s *Snapshot) layerActive(x string) bool {
	return slices.Contains(s.a.Layers(), x)
}

// withoutLayer is the value at res's path without layer x's lines, other layers included, and
// false when it has none: an entry only x adds. A session that does not apply x shows it as is.
func (s *Snapshot) withoutLayer(res resolution, x string) (value.Value, bool) {
	if !s.layerActive(x) {
		return res.Target, res.Target != nil
	}
	path := []value.Value{res.val}
	for _, st := range res.Steps {
		path = append(path, st.Value)
		if _, ok := s.historyWithout(path, x); !ok {
			return nil, false // an item only x adds, or one inside it
		}
	}
	return s.historyWithout(path, x)
}

// historyWithout is the newest value path's last value had that no amendment or file of x states.
func (s *Snapshot) historyWithout(path []value.Value, x string) (value.Value, bool) {
	for _, v := range s.a.History(path...) {
		if !s.statedBy(v, x) {
			return v, true
		}
	}
	return nil, false
}

// keptWithout reports p's value present once layer x's lines are taken out: false when p
// resolves to nothing, or to an entry only x adds.
func (s *Snapshot) keptWithout(p Path, x string) bool {
	res, err := s.open(p)
	if err != nil {
		return false
	}
	_, ok := s.withoutLayer(res, x)
	return ok
}

// statedBy reports v set by an amendment of layer x, or stated in one of x's files.
func (s *Snapshot) statedBy(v value.Value, x string) bool {
	p := v.Prov()
	if p == nil {
		return false
	}
	if amendedBy(p, x) {
		return true
	}
	f := s.files[p.Span.File]
	return f != nil && f.FileKind == syntax.FileLayer && f.Layer != nil && f.Layer.Name == x
}

// amendedBy reports p the right-hand side of an amendment of layer x: a value computed under one
// carries its layer too (API.md F11), but no text of x states it.
func amendedBy(p *value.Prov, x string) bool {
	return p != nil && p.Kind == value.ProvLayer && p.Layer() == x
}

// lineSource reports the line of f amending root r a Source can state (sourceLine).
func (s *Snapshot) lineSource(ctx context.Context, f *syntax.File, r Path, it *syntax.Amendment) bool {
	sp := f.Span(it.Value)
	return s.sourceLine(ctx, s.linePathOf(f, r, it.Path), string(f.Src.Content[sp.Start:sp.End]))
}

// sourceLine reports text a Source of the declared type at path (API.md 8.2): the one test of a
// layer line an operation can write, Editable's and the Undo's (DECISIONS 273).
func (s *Snapshot) sourceLine(ctx context.Context, path, text string) bool {
	t, pkg, ok := s.lineType(path)
	if !ok {
		return false
	}
	tc := &typing{ctx: ctx, ty: Typer{Pkg: pkg}}
	_, err := tc.value(Source(lineKey(text)), t)
	return err == nil
}

// lineType is the declared type of the value at path in s, or of an item of the collection
// holding it when s has no such item, and its package.
func (s *Snapshot) lineType(path string) (types.Type, string, bool) {
	p, err := Parse(path)
	if err != nil {
		return nil, "", false
	}
	if res, err := s.open(p); err == nil {
		t, err := s.Type(res.Resolved)
		return t, res.root.pkg.Path, err == nil && t != nil
	}
	parent, ok := enclosing(path)
	if !ok {
		return nil, "", false
	}
	pp, _ := Parse(parent)
	res, err := s.open(pp)
	if err != nil {
		return nil, "", false
	}
	t, err := s.Type(res.Resolved)
	if err != nil || t == nil {
		return nil, "", false
	}
	if _, vt, isMap := mapTypes(present(t)); isMap {
		return vt, res.root.pkg.Path, true
	}
	et := elemType(present(t))
	return et, res.root.pkg.Path, et != nil
}
