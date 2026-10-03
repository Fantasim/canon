package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/workspace"
)

type documentID struct {
	URI string `json:"uri"`
}

// positionParams is LSP 3.17's TextDocumentPositionParams.
type positionParams struct {
	TextDocument documentID `json:"textDocument"`
	Position     position   `json:"position"`
}

// spot is a position in a file of a project's analysis, the byte offset: in a .canon source, the
// file and the nodes holding it, from the file down to the innermost; in a loaded file, its id.
type spot struct {
	a      *build.Analysis
	snap   *edit.Snapshot
	info   *check.Info
	file   *syntax.File
	loaded source.FileID
	off    int
	chain  []syntax.Node
	conv   *converter
}

// spotAt is the spot params names; nil when its document is in no project, the project does
// not analyse, or the file is neither a source of the analysis nor a file it loaded.
func (s *server) spotAt(ctx context.Context, params json.RawMessage) (*spot, error) {
	var p positionParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	abs, ok := pathOf(p.TextDocument.URI)
	if !ok {
		return nil, nil
	}
	a, root, err := s.analysis(ctx, abs)
	if a == nil || err != nil {
		return nil, err
	}
	set, isSet := a.Files().(fileSet)
	if !isSet {
		return nil, errFileSet
	}
	sp := &spot{a: a, snap: edit.NewSnapshot(a), info: a.Program().Info, conv: newConverter(set, root)}
	f := sourceOf(a.Program(), abs)
	if f == nil {
		return sp.inLoaded(ctx, set, abs, p.Position)
	}
	sp.file, sp.off = f, newLines(f.Src.Content).offset(p.Position)
	sp.chain = chainAt(f, sp.off)
	return sp, nil
}

// inLoaded is the spot at pos in a file the analysis's values were read from, the version it
// read; nil for a file it did not read.
func (sp *spot) inLoaded(ctx context.Context, set fileSet, abs string, pos position) (*spot, error) {
	id, err := sp.snap.FileOf(ctx, abs)
	f := set.File(id)
	if err != nil || f == nil {
		return nil, err
	}
	sp.loaded, sp.off = id, newLines(f.Content).offset(pos)
	return sp, nil
}

// analysis is the analysis of every package of the project abs's buffer belongs to, synced first
// to the buffers; nil for a document in no project or a project the pass has not opened.
func (s *server) analysis(ctx context.Context, abs string) (*build.Analysis, string, error) {
	s.mu.Lock()
	var root string
	if doc := s.docs[abs]; doc != nil {
		root = doc.root
	}
	p := s.projects[root]
	s.mu.Unlock()
	if p == nil {
		return nil, "", nil
	}
	ws, err := p.opened(ctx)
	if ws == nil || err != nil {
		return nil, "", err
	}
	snap, err := ws.Read(ctx)
	if err != nil {
		return nil, "", err
	}
	a, err := workspace.Analyze(ctx, snap, nil)
	var oe *build.OpenError
	switch {
	case errors.As(err, &oe):
		return nil, "", nil // project.canon in error: its findings are published, nothing is read
	case err != nil:
		return nil, "", err
	}
	return a, root, nil
}

// sourceOf is the parsed source of prog named abs, nil for none.
func sourceOf(prog *check.Program, abs string) *syntax.File {
	for _, pkg := range prog.Packages {
		for _, f := range pkg.Files {
			if f.Src.Abs == abs {
				return f
			}
		}
	}
	return nil
}

// chainAt is the nodes of f whose span holds the byte offset off, outermost first.
func chainAt(f *syntax.File, off int) []syntax.Node {
	var chain []syntax.Node
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n == nil {
			return false
		}
		sp := f.Span(n)
		if off < int(sp.Start) || off >= int(sp.End) {
			return false
		}
		chain = append(chain, n)
		return true
	})
	return chain
}

// object is the object the innermost name at the spot declares or names, and that name.
func (sp *spot) object() (check.Object, syntax.Node) {
	for _, n := range slices.Backward(sp.chain) {
		if obj := sp.info.ObjectOf(n); obj != nil {
			return obj, n
		}
	}
	return nil, nil
}

// keyAt are the entries the innermost key of a ref at the spot names, none when it names none
// it can find; inKey is false when the spot is in no key, or in a name nearer. In a loaded file,
// the key is a ref a let's value states there.
func (sp *spot) keyAt(ctx context.Context) (rs []edit.Resolved, inKey bool, err error) {
	// TYPES.md §4.1, IMPLEMENTATION-PLAN §8.4 Features
	if sp.file == nil {
		rs, err := sp.statedEntries(ctx, nil)
		return rs, len(rs) > 0, err
	}
	for _, n := range slices.Backward(sp.chain) {
		if expr, ok := n.(syntax.Expr); ok && sp.info.Keys[expr] != nil {
			rs, err := sp.keyEntries(ctx, sp.info.Keys[expr], expr)
			return rs, true, err
		}
		if sp.info.ObjectOf(n) != nil {
			return nil, false, nil
		}
	}
	return nil, false, nil
}

// keyEntries are the entries a key written in source names: in a let's collection, by its path;
// in a record field's, the one of the instance its selector reads statically, else of each
// instance a let's value evaluates the key in, as a ref.
func (sp *spot) keyEntries(ctx context.Context, coll *types.Collection, e syntax.Expr) ([]edit.Resolved, error) {
	// TYPES.md §10.2, DECISIONS 285
	if coll.Kind == types.CollLet {
		return sp.resolved(keyPath(coll, e)), nil
	}
	if x, ok := e.(*syntax.SelectorExpr); ok {
		return sp.resolved(sp.selectedKey(x)), nil
	}
	at := sp.file.Span(e)
	return sp.statedEntries(ctx, &at)
}

// keyPath is the value path of the entry a key written in source names in a let's collection:
// the let, its field path, then the key; nil for a key not written whole.
func keyPath(coll *types.Collection, e syntax.Expr) *edit.Path {
	k, ok := writtenKey(e)
	if !ok {
		return nil
	}
	p := edit.Path{Package: coll.Pkg, Root: coll.Name}
	for _, f := range coll.FieldPath {
		p.Segs = append(p.Segs, edit.Seg{Kind: edit.SegField, Name: f})
	}
	p.Segs = append(p.Segs, keySeg(k))
	return &p
}

// selectedKey is the value path of the entry `x.key` names, where x is read statically from a
// let; nil for any other receiver.
func (sp *spot) selectedKey(x *syntax.SelectorExpr) *edit.Path {
	k, ok := writtenKey(x)
	p, static := sp.snap.StaticPath(x.X)
	if !ok || !static {
		return nil
	}
	p.Segs = append(p.Segs, keySeg(k))
	return &p
}

// statedEntries are the entries named by the refs the lets' values state at the spot, one per
// instance a ref stated once is evaluated in; with exact, only refs stated by that very span.
func (sp *spot) statedEntries(ctx context.Context, exact *source.Span) ([]edit.Resolved, error) {
	file := sp.loaded
	if sp.file != nil {
		file = sp.file.Src.ID
	}
	vs, err := sp.snap.ValuesAt(ctx, file, sp.off)
	if err != nil {
		return nil, err
	}
	var refs []*value.Ref
	for _, v := range vs {
		if ref, ok := v.(*value.Ref); ok && (exact == nil || ref.P.Span == *exact) {
			refs = append(refs, ref)
		}
	}
	if len(refs) == 0 {
		return nil, nil
	}
	return sp.snap.EntriesOf(ctx, refs)
}

// keySeg is the `[key]` segment of a key (API.md P9): an integer, a word, else a string.
func keySeg(k value.Key) edit.Seg {
	lit := edit.KeyLit{Kind: edit.KeyString, Text: k.S}
	switch {
	case k.IsInt:
		lit = edit.KeyLit{Kind: edit.KeyInt, Int: k.I}
	case value.IsWord(k.S):
		lit.Kind = edit.KeyWord
	}
	return edit.Seg{Kind: edit.SegKey, Key: lit}
}

// writtenKey is the key a name or a selector's name, else a literal, names, as evaluation reads it.
func writtenKey(e syntax.Expr) (value.Key, bool) {
	switch x := e.(type) {
	case *syntax.IdentExpr:
		return value.Key{S: x.Name}, true
	case *syntax.SelectorExpr:
		return value.Key{S: x.Name.Name}, true
	}
	return eval.LiteralKey(e)
}

// resolved is resolve's value as a list of one, empty for none.
func (sp *spot) resolved(p *edit.Path) []edit.Resolved {
	r, ok := sp.resolve(p)
	if !ok {
		return nil
	}
	return []edit.Resolved{r}
}

// resolve is the value at a path of the spot's analysis, false when it has none.
func (sp *spot) resolve(p *edit.Path) (edit.Resolved, bool) {
	if p == nil {
		return edit.Resolved{}, false
	}
	r, err := edit.Resolve(sp.snap, *p)
	return r, err == nil && r.Target != nil
}

// location is the LSP location of a span of the analysis, false for a span in no file.
func (sp *spot) location(at source.Span) (location, bool) {
	abs, rng, ok := sp.conv.spanOf(at)
	if !ok {
		return location{}, false
	}
	return location{URI: uriOf(abs), Range: rng}, true
}
