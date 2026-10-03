package check_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// gaps lists the nodes of f's imports, declarations, amends and translations Info misses (IMPLEMENTATION-PLAN §4.7).
func gaps(f *syntax.File, info *check.Info, broken func(syntax.Decl) bool) []string {
	var out []string
	noObject := map[*syntax.Ident]bool{}
	noRecord := map[syntax.Node]bool{}
	for _, root := range roots(f, broken) {
		syntax.Inspect(root, func(n syntax.Node) bool {
			if n == nil || exemptNode(n) || noObject[identOf(n)] {
				return false
			}
			for _, id := range namesNoObject(info, n) {
				noObject[id] = true
			}
			if props := viewProps(n); props != nil {
				noRecord[props] = true
			}
			if noRecord[n] {
				return true
			}
			if msg := missing(info, n); msg != "" {
				line, col := f.Src.Position(f.Span(n).Start)
				out = append(out, fmt.Sprintf("%s:%d:%d %s %s", f.Src.Path, line, col, n.Kind(), msg))
			}
			return true
		})
	}
	return append(out, valueNameGaps(f, info, broken)...)
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
	for _, e := range f.Entries {
		out = append(out, e)
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
	default:
		out = presentationNoObject(n)
	}
	return out
}

// presentationNoObject are the identifiers of a view or translation node naming no object:
// group and show ids, property names, a key's reserved segments and ids (I18N.md K5).
func presentationNoObject(n syntax.Node) []*syntax.Ident {
	switch n := n.(type) {
	case *syntax.ViewGroup:
		return []*syntax.Ident{n.ID}
	case *syntax.ViewShow:
		return []*syntax.Ident{n.ID}
	case *syntax.ViewField:
		return propNames(n.Props)
	case *syntax.TranslationEntry:
		return keyNoObject(n.Key.Parts)
	}
	return nil
}

// propNames are the property names of a view field's list (VIEWMODEL.md §3.5).
func propNames(props *syntax.BraceLit) []*syntax.Ident {
	if props == nil {
		return nil
	}
	var out []*syntax.Ident
	for _, it := range props.Items {
		if fi, ok := it.(*syntax.FieldItem); ok {
			out = append(out, fi.Name)
		}
	}
	return out
}

// reservedSegments are I18N.md §3.2's; the kind words among them are followed by a name.
var (
	reservedSegments = strings.Fields("help title subtitle singular plural group show check intro text deprecated placeholder none step field method case member")
	kindWords        = []string{"field", "method", "case", "member"}
)

// keyNoObject reads a key as K5 does and returns the segments that are not names.
func keyNoObject(parts []*syntax.Ident) []*syntax.Ident {
	var out []*syntax.Ident
	prev := ""
	for _, p := range parts {
		switch {
		case slices.Contains(kindWords, prev):
		case prev == "group" || prev == "show" || slices.Contains(reservedSegments, p.Name):
			out = append(out, p)
		}
		prev = p.Name
	}
	return out
}

// viewProps is a view field's property list, which is no value: only its values are recorded.
func viewProps(n syntax.Node) syntax.Node {
	if f, ok := n.(*syntax.ViewField); ok && f.Props != nil {
		return f.Props
	}
	return nil
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

// exemptDecl is an emit, whose options name no object; its `values:` names are valueNameGaps'.
func exemptDecl(d syntax.Decl) bool {
	_, ok := d.(*syntax.EmitDecl)
	return ok
}

// exemptNode is a subtree naming no object, but for `@files` templates (valueNameGaps): annotations,
// doc comments and modifiers.
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

// valueNameGaps lists the `emit … values:` names and `@files` template names Info misses: each in
// Uses, each `.g` in NameUses, but `{id}` (the key) and a `g` several cases declare.
func valueNameGaps(f *syntax.File, info *check.Info, broken func(syntax.Decl) bool) []string {
	var out []string
	for _, d := range f.Decls {
		if broken(d) {
			continue
		}
		var names []syntax.Node
		switch d := d.(type) {
		case *syntax.EmitDecl:
			names = emitValueNames(info, d)
		case *syntax.LetDecl:
			names = templateNames(info, d)
		}
		for _, n := range names {
			line, col := f.Src.Position(f.Span(n).Start)
			out = append(out, fmt.Sprintf("%s:%d:%d %s not recorded", f.Src.Path, line, col, n.Kind()))
		}
	}
	return out
}

// emitValueNames are the names of an emit's `values:` list that Uses lacks.
func emitValueNames(info *check.Info, d *syntax.EmitDecl) []syntax.Node {
	var out []syntax.Node
	for _, it := range d.Options.Items {
		fi, ok := it.(*syntax.FieldItem)
		if !ok || fi.Name.Name != check.OptValues {
			continue
		}
		list, _ := fi.Value.(*syntax.ListLit)
		for i := 0; list != nil && i < len(list.Elems); i++ {
			if x, isName := list.Elems[i].(*syntax.IdentExpr); !isName || info.Uses[x] == nil {
				out = append(out, list.Elems[i])
			}
		}
	}
	return out
}

// templateNames are the names of a let's `@files` template Info lacks: a name naming one field
// across the types its path may have must be recorded; one naming several, or `{id}`, need not.
func templateNames(info *check.Info, d *syntax.LetDecl) []syntax.Node {
	var out []syntax.Node
	for _, a := range d.Annotations {
		if a.Name.Name != syntax.AnnFiles || len(a.Args) == 0 {
			continue
		}
		s, ok := a.Args[0].Value.(*syntax.StringLit)
		for i := 0; ok && i < len(s.Parts); i++ {
			if p := s.Parts[i]; p.Interp != nil {
				out = append(out, pathGaps(info, elemTypes(info, d), tplNames(p.Interp.X))...)
			}
		}
	}
	return out
}

// elemTypes is the element type of the let d's table or keyed list.
func elemTypes(info *check.Info, d *syntax.LetDecl) []types.Type {
	switch x := info.Defs[d.Name].Type().Base().(type) {
	case *types.TableType:
		return []types.Type{x.Elem}
	case *types.ListType:
		return []types.Type{x.Elem}
	}
	return nil
}

// tplNames is a template name path: `f`, then each `.g`; nil for `{id}`.
func tplNames(x syntax.Expr) []syntax.Node {
	if id, ok := x.(*syntax.IdentExpr); ok && id.Name == "id" {
		return nil
	}
	return namePathOf(x)
}

func namePathOf(x syntax.Expr) []syntax.Node {
	if s, ok := x.(*syntax.SelectorExpr); ok {
		return append(namePathOf(s.X), s.Name)
	}
	return []syntax.Node{x}
}

// pathGaps are the names of path naming one field across the types ts the path may have, unrecorded.
func pathGaps(info *check.Info, ts []types.Type, path []syntax.Node) []syntax.Node {
	var out []syntax.Node
	for _, n := range path {
		fields := fieldsNamed(ts, nameOf(n))
		if len(fields) < 2 && info.ObjectOf(n) == nil {
			out = append(out, n)
		}
		ts = ts[:0:0]
		for _, f := range fields {
			ts = append(ts, f.Type)
		}
	}
	return out
}

// fieldsNamed are the distinct fields name names across ts.
func fieldsNamed(ts []types.Type, name string) []*types.Field {
	var out []*types.Field
	for _, t := range ts {
		for _, f := range fieldsOf(t) {
			if f.Name == name && !slices.Contains(out, f) {
				out = append(out, f)
			}
		}
	}
	return out
}

func nameOf(n syntax.Node) string {
	switch n := n.(type) {
	case *syntax.IdentExpr:
		return n.Name
	case *syntax.Ident:
		return n.Name
	}
	return ""
}

// fieldsOf are the fields a value of type t may have: a record's, a case's, every case's of a
// variant, every branch's of a dependent type (DECISIONS 280).
func fieldsOf(t types.Type) []*types.Field {
	return fieldsThrough(t, map[*types.TypeFunc]bool{})
}

func fieldsThrough(t types.Type, seen map[*types.TypeFunc]bool) []*types.Field {
	if opt, ok := t.Base().(*types.OptionalType); ok {
		t = opt.Elem
	}
	switch x := t.Base().(type) {
	case *types.RecordType:
		return x.Fields
	case *types.CaseType:
		return x.Fields
	case *types.VariantType:
		var out []*types.Field
		for _, c := range x.Cases {
			out = append(out, c.Fields...)
		}
		return out
	case *types.TypeAppType:
		return branchFields(x.Fn, seen)
	case *types.DepUnionType:
		return branchFields(x.Fn, seen)
	}
	return nil
}

// branchFields are the fields of every branch of fn, read once.
func branchFields(fn *types.TypeFunc, seen map[*types.TypeFunc]bool) []*types.Field {
	if seen[fn] {
		return nil
	}
	seen[fn] = true
	if fn.Body != nil {
		return fieldsThrough(fn.Body, seen)
	}
	var out []*types.Field
	for _, a := range fn.Arms {
		if a.Result != nil {
			out = append(out, fieldsThrough(a.Result, seen)...)
		}
	}
	return out
}
