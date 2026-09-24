package check_test

import (
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// gaps lists the nodes of f's imports, declarations and amend blocks Info misses (IMPLEMENTATION-PLAN §4.7).
func gaps(f *syntax.File, info *check.Info, broken func(syntax.Decl) bool) []string {
	var out []string
	noObject := map[*syntax.Ident]bool{}
	for _, root := range roots(f, broken) {
		syntax.Inspect(root, func(n syntax.Node) bool {
			if n == nil || exemptNode(n) || noObject[identOf(n)] {
				return false
			}
			for _, id := range namesNoObject(info, n) {
				noObject[id] = true
			}
			if msg := missing(info, n); msg != "" {
				line, col := f.Src.Position(f.Span(n).Start)
				out = append(out, fmt.Sprintf("%s:%d:%d %s %s", f.Src.Path, line, col, n.Kind(), msg))
			}
			return true
		})
	}
	return out
}

// roots are the subtrees the oracle walks: imports, checked declarations, a layer's name and
// its amend blocks.
func roots(f *syntax.File, broken func(syntax.Decl) bool) []syntax.Node {
	var out []syntax.Node
	for _, imp := range f.Imports {
		out = append(out, imp)
	}
	for _, d := range f.Decls {
		if !exemptDecl(d) && !broken(d) {
			out = append(out, d)
		}
	}
	if f.Layer != nil {
		out = append(out, f.Layer)
	}
	for _, b := range f.Amends {
		out = append(out, b)
	}
	return out
}

// identOf is n as an identifier, or nil.
func identOf(n syntax.Node) *syntax.Ident {
	id, _ := n.(*syntax.Ident)
	return id
}

// namesNoObject are the identifiers under n that name no object (IMPLEMENTATION-PLAN §4.7).
func namesNoObject(info *check.Info, n syntax.Node) []*syntax.Ident {
	var out []*syntax.Ident
	switch n := n.(type) {
	case *syntax.CallExpr:
		if c := info.Calls[n]; c != nil && (c.Kind == check.CalleeBuiltin || c.Kind == check.CalleeConvert) {
			out = argNames(n.Args)
		}
	case *syntax.LoadExpr:
		out = argNames(n.Args)
		if n.Method != nil {
			out = append(out, n.Method)
		}
	case *syntax.ExpectStmt:
		out = []*syntax.Ident{n.Outcome}
		if id, ok := n.Message.(*syntax.Ident); ok {
			out = append(out, id)
		}
	case *syntax.SelectorExpr:
		if info.Keys[n] != nil {
			out = []*syntax.Ident{n.Name}
		}
	case *syntax.Import:
		out = n.Path.Parts[:len(n.Path.Parts)-1]
	case *syntax.AssetType:
		for _, e := range n.Exts {
			if id, ok := e.(*syntax.Ident); ok {
				out = append(out, id)
			}
		}
	}
	return out
}

func argNames(args []*syntax.Arg) []*syntax.Ident {
	var out []*syntax.Ident
	for _, a := range args {
		if a.Name != nil {
			out = append(out, a.Name)
		}
	}
	return out
}

// exemptDecl is a declaration whose content names no object: an emit's options.
func exemptDecl(d syntax.Decl) bool {
	_, ok := d.(*syntax.EmitDecl)
	return ok
}

// exemptNode is a subtree naming no object: annotations and doc comments.
func exemptNode(n syntax.Node) bool {
	switch n.(type) {
	case *syntax.Annotation, *syntax.DocComment, *syntax.Modifiers:
		return true
	}
	return false
}

// missing is what Info lacks for n, "" when it has it.
func missing(info *check.Info, n syntax.Node) string {
	if msg := missingRecord(info, n); msg != "" {
		return msg
	}
	switch n := n.(type) {
	case *syntax.IdentExpr:
		if info.Uses[n] == nil && info.Keys[n] == nil && !info.Symbols[n] {
			return "not in Uses, Keys or Symbols"
		}
	case *syntax.Ident:
		if info.ObjectOf(n) == nil {
			return "not in Defs or NameUses"
		}
	}
	if e, ok := n.(syntax.Expr); ok && info.Types[e] == nil && !isPackage(info, e) {
		return "not in Types"
	}
	if t, ok := n.(syntax.Type); ok && info.TypeExprs[t] == nil {
		return "not in TypeExprs"
	}
	return ""
}

// missingRecord is the per-kind table of §4.7 a node lacks: Calls, Selections, Literals, Matches.
func missingRecord(info *check.Info, n syntax.Node) string {
	switch n := n.(type) {
	case *syntax.CallExpr:
		if info.Calls[n] == nil {
			return "not in Calls"
		}
	case *syntax.SelectorExpr:
		if info.Selections[n] == nil && !qualifiedForm(info, n) {
			return "not in Selections"
		}
	case *syntax.BraceLit:
		if _, ok := info.Literals[n]; !ok {
			return "not in Literals"
		}
	case *syntax.MatchExpr, *syntax.MatchStmt:
		if info.Matches[n] == nil {
			return "not in Matches"
		}
	}
	return ""
}

// qualifiedForm reports `pkg.x`, `T.member` or `pkg.T.member`, which have no Selection.
func qualifiedForm(info *check.Info, s *syntax.SelectorExpr) bool {
	switch x := s.X.(type) {
	case *syntax.IdentExpr:
		o := info.Uses[x]
		return o != nil && (o.Kind() == check.ObjPackage || o.Kind() == check.ObjTypeName || o.Kind() == check.ObjBuiltin)
	case *syntax.SelectorExpr:
		o := info.NameUses[x.Name]
		return o != nil && o.Kind() == check.ObjTypeName
	}
	return false
}

// isPackage reports a package qualifier, which has no type (DECISIONS 152).
func isPackage(info *check.Info, e syntax.Expr) bool {
	id, ok := e.(*syntax.IdentExpr)
	return ok && info.Uses[id] != nil && info.Uses[id].Kind() == check.ObjPackage
}

// IMPLEMENTATION-PLAN §4.7.
func TestInfoIsComplete(t *testing.T) {
	l := loadExamples(t, "teamboard", "sovcommon/ui", "sovcommon/roles")
	prog, _ := l.run(t)
	broken := func(syntax.Decl) bool { return false }
	for _, f := range l.files {
		for _, g := range gaps(f, prog.Info, broken) {
			t.Error(g)
		}
	}
}
