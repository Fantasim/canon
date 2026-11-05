package edit

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// writeScope is where an edit writes a ref's word key: the package and file, and the node its text
// replaces, whose enclosing functions, lambdas and records add their names; no file for a new one.
type writeScope struct {
	s    *Snapshot
	pkg  string
	file *syntax.File
	at   syntax.Node
}

// scopeOf is the scope at node n of f, a file of s.
func (s *Snapshot) scopeOf(f *syntax.File, n syntax.Node) writeScope {
	return writeScope{s: s, pkg: s.packageOf(f.Src.Path), file: f, at: n}
}

// keyText is bare, the text of ref key k of ref type t, as written at ws: a word key that a name in
// scope there would read instead of the key is a string, which names a ref's key (TYPES.md 3.3, 4.1;
// DECISIONS 318, 366).
func (ws writeScope) keyText(k value.Key, t types.Type, bare string) string {
	if ws.s == nil || k.IsInt || bare != k.S || !ws.readsName(t, k.S) {
		return bare
	}
	return canonQuote(k.S)
}

// scope is where p prints: the file and package its names give, none for a Source's text.
func (p *canonPrinter) scope() writeScope {
	if p.names == nil || p.a == nil {
		return writeScope{}
	}
	return writeScope{s: p.a.snap, pkg: p.names.pkg, file: p.names.file}
}

// readsName reports word, written bare at ws against ref type t, read as a name in scope instead of
// t's key, whatever step 1 would find: a declaration of the package, a name an import binds, or a
// parameter, local, field or method of an enclosing body (TYPES.md 3.3, 4.1; DECISIONS 318, 366).
func (ws writeScope) readsName(t types.Type, word string) bool {
	_, isRef := baseOf(present(t)).(*types.RefType)
	return isRef && ws.names(word)
}

// names reports word a name in scope at ws: a declaration of the package, a name an import binds, or
// a parameter, local, field or method of an enclosing body (steps 2 to 5).
func (ws writeScope) names(word string) bool {
	return ws.s != nil && (ws.s.declares(ws.pkg, word) || ws.imports(word) || ws.encloses(word))
}

// declares reports package pkg of s declaring a name word, of any kind but an entry (step 4).
func (s *Snapshot) declares(pkg, word string) bool {
	p := s.byPath[pkg]
	return p != nil && slices.ContainsFunc(p.Decls, func(o check.Object) bool { return o.Name() == word && o.Kind() != check.ObjEntry })
}

// imports reports ws's file binding word by an import (step 5): one of its names, else its alias or,
// with neither, its path's last segment.
func (ws writeScope) imports(word string) bool {
	if ws.file == nil {
		return false
	}
	return slices.ContainsFunc(ws.file.Imports, func(imp *syntax.Import) bool {
		switch {
		case len(imp.Names) > 0:
			return slices.ContainsFunc(imp.Names, func(n *syntax.Ident) bool { return n.Name == word })
		case imp.Alias != nil:
			return imp.Alias.Name == word
		}
		return lastSegment(strings.Join(partNames(imp.Path.Parts), dotSeg)) == word
	})
}

// encloses reports a function, lambda or record holding ws's node that declares a parameter, local,
// field or method named word (steps 2 and 3); every name of its body counts, so a word one may read is quoted.
func (ws writeScope) encloses(word string) bool {
	if ws.file == nil || ws.at == nil {
		return false
	}
	for _, n := range chainTo(ws.file, ws.file, ws.file.Span(ws.at)) {
		switch n.(type) {
		case *syntax.FnDecl, *syntax.LambdaExpr, *syntax.RecordDecl:
			if ws.s.definesIn(n, word) {
				return true
			}
		}
	}
	return false
}

// definesIn reports a value or a method named word declared inside n.
func (s *Snapshot) definesIn(n syntax.Node, word string) bool {
	found := false
	syntax.Inspect(n, func(c syntax.Node) bool {
		if id, ok := c.(*syntax.Ident); ok && id.Name == word && s.info.Defs[id] != nil {
			k := s.info.Defs[id].Kind()
			found = scopeKinds[k] || k == check.ObjMethod
		}
		return !found
	})
	return found
}

// mapItemOf is the item of a map literal whose word key id is, and the map's key type; nil for any
// other name.
func (s *Snapshot) mapItemOf(f *syntax.File, id *syntax.Ident) (*syntax.FieldItem, types.Type) {
	chain := chainTo(f, f, f.Span(id))
	for i := len(chain) - 1; i > 0; i-- {
		if fi, isItem := chain[i].(*syntax.FieldItem); isItem && fi.Name == id {
			brace, isExpr := chain[i-1].(syntax.Expr)
			if !isExpr {
				return nil, nil
			}
			if kt, _, ok := mapTypes(s.info.Types[brace]); ok {
				return fi, kt
			}
			return nil, nil
		}
	}
	return nil, nil
}
