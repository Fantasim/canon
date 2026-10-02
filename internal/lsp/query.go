package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/edit"
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

// spot is a position in a .canon source of a project's analysis: the file, the byte offset, and
// the nodes holding it, from the file down to the innermost.
type spot struct {
	a     *build.Analysis
	snap  *edit.Snapshot
	info  *check.Info
	file  *syntax.File
	off   int
	chain []syntax.Node
	conv  *converter
}

// spotAt is the spot params names; nil when its document is in no project, the project does
// not analyse, or the file is no source of the analysis.
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
	f := sourceOf(a.Program(), abs)
	set, isSet := a.Files().(fileSet)
	switch {
	case f == nil:
		return nil, nil
	case !isSet:
		return nil, errFileSet
	}
	off := newLines(f.Src.Content).offset(p.Position)
	sp := &spot{a: a, snap: edit.NewSnapshot(a), info: a.Program().Info, file: f, off: off, conv: newConverter(set, root)}
	sp.chain = chainAt(f, off)
	return sp, nil
}

// analysis is the analysis of every package of the project abs's buffer belongs to, which the
// notifications before this request synced; nil for a document in no project or a project that
// does not open.
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
	ws, _ := p.current()
	if ws == nil {
		return nil, "", nil
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

// keyAt is the innermost key of a ref the spot is in, as the path of the entry it names (nil
// for none it can name), and the key's node; a nil node when the spot is in no key, or in a name nearer.
func (sp *spot) keyAt() (*edit.Path, syntax.Node) {
	// TYPES.md §4.1
	for _, n := range slices.Backward(sp.chain) {
		if expr, ok := n.(syntax.Expr); ok && sp.info.Keys[expr] != nil {
			return keyPath(sp.info.Keys[expr], expr), n
		}
		if sp.info.ObjectOf(n) != nil {
			return nil, nil
		}
	}
	return nil, nil
}

// keyPath is the value path of the entry a key written in source names: the collection's let,
// its field path, then the key; nil for a collection of a record's field or a key not written whole.
func keyPath(coll *types.Collection, e syntax.Expr) *edit.Path {
	k, ok := writtenKey(e)
	if !ok || coll.Kind != types.CollLet {
		return nil
	}
	p := edit.Path{Package: coll.Pkg, Root: coll.Name}
	for _, f := range coll.FieldPath {
		p.Segs = append(p.Segs, edit.Seg{Kind: edit.SegField, Name: f})
	}
	p.Segs = append(p.Segs, keySeg(k))
	return &p
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

// writtenKey is the key a name, a selector, an integer or a string without interpolation
// names, as evaluation reads it (EVALUATION.md, eval's literalKey).
func writtenKey(e syntax.Expr) (value.Key, bool) {
	switch x := e.(type) {
	case *syntax.IdentExpr:
		return value.Key{S: x.Name}, true
	case *syntax.SelectorExpr:
		return value.Key{S: x.Name.Name}, true
	case *syntax.IntLit:
		return value.Key{I: x.Value.Int64(), IsInt: true}, x.Value.IsInt64()
	case *syntax.RawStringLit:
		return value.Key{S: x.Value}, true
	case *syntax.StringLit:
		if len(x.Parts) == 0 {
			return value.Key{}, true
		}
		return value.Key{S: x.Parts[0].Text}, len(x.Parts) == 1 && x.Parts[0].Interp == nil
	}
	return value.Key{}, false
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
