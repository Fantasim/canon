package progen_test

import (
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// Operators on layers and inputs, and on packages and names.
func namesOperators() []operator {
	// TYPES.md §3.1 names no import of a cycle to report it at.
	e2002 := op(diag.E2002.Def().Code, "TYPES.md §3.1 (import cycle)", importCycle)
	e2002.loose = true
	return []operator{
		op(diag.E1902.Def().Code, "EVALUATION.md §9.2 (amend a type)", amendType),
		op(diag.E1903.Def().Code, "EVALUATION.md §11.1 (input record reached twice)", inputRecordTwice),
		op(diag.E1904.Def().Code, "EVALUATION.md §11.3 (inline flag in an input pattern)", inputField(replaceType(`String(/(?i)^sk-/)?`))),
		op(diag.E1905.Def().Code, "EVALUATION.md §9.2 (no such field)", amendPath(renameLastSegment)),
		{code: diag.E1901.Def().Code, rule: "EVALUATION.md §9.1 (--layer names no layer)", loose: true, sites: unknownLayer},
		op(diag.E1906.Def().Code, "EVALUATION.md §9.1 (layer declared twice)", layerTwice),
		op(diag.E1907.Def().Code, "EVALUATION.md §11.1 (input with a default)", inputField(inputDefault)),
		op(diag.E1908.Def().Code, "EVALUATION.md §9.2 (path set twice)", amendPath(duplicateAmendment)),
		op(diag.E1909.Def().Code, "EVALUATION.md §9.2 (amend another package)", amendForeign),
		op(diag.E1910.Def().Code, "EVALUATION.md §11.1 (input list)", inputField(replaceType(`[String]`))),
		op(diag.E1911.Def().Code, "EVALUATION.md §11.1 (invalid env name)", inputField(badEnvName)),
		op(diag.E2001.Def().Code, "TYPES.md §3.1 (two packages in one directory)", twoPackagesOneDir),
		e2002,
		op(diag.E2003.Def().Code, "TYPES.md §3.1 (unknown package)", afterPackage("import ", "nowhere", "")),
		op(diag.E2004.Def().Code, "TYPES.md §3.1 (no such public name)", importName("Nope")),
		op(diag.E2005.Def().Code, "TYPES.md §3.1 (name bound twice by imports)", importTwice),
		op(diag.E2006.Def().Code, "TYPES.md §3.1 (descendant package)", descendantPackage),
		op(diag.E2101.Def().Code, "TYPES.md §4.2 (contextual name shadowed)", shadowMember),
		op(diag.E2102.Def().Code, "TYPES.md §3.3 (unknown name)", unknownName),
		op(diag.E2103.Def().Code, "TYPES.md §10.2 (ref without collection)", refWithoutCollection),
		op(diag.E2104.Def().Code, "TYPES.md §3.6 (field and method)", fieldAndMethod),
		op(diag.E2105.Def().Code, "TYPES.md §3.6 (id on a table element)", idOnTableElement),
		op(diag.E2106.Def().Code, "TYPES.md §3.2 (declared twice)", constTwice),
		op(diag.E2107.Def().Code, "TYPES.md §3.4 (local declared twice)", localTwice),
		op(diag.E2108.Def().Code, "TYPES.md §3.4 (self outside a record)", topFnStatement("let zzSelf = ", "self")),
		op(diag.E2109.Def().Code, "TYPES.md §3.4 (it outside a predicate)", topFnStatement("let zzIt = ", "it")),
		op(diag.E2110.Def().Code, "TYPES.md §3.2 (type as a value)", topFnStatement("let zzType = ", "Int")),
	}
}

// peers are the targets of tg's package.
func peers(tg target) []target {
	var out []target
	for _, p := range *tg.all {
		if p.pkg == tg.pkg && p.path != projectFile {
			out = append(out, p)
		}
	}
	return out
}

// declared lists the names of tg's package's declarations of type T.
func declared[T syntax.Node](tg target, name func(T) *syntax.Ident) []string {
	var out []string
	for _, p := range peers(tg) {
		for _, n := range nodes[T](p) {
			if id := name(n); id != nil {
				out = append(out, id.Name)
			}
		}
	}
	return out
}

func amendType(tg target) []progen.Site {
	if !isLayer(tg) {
		return nil
	}
	var out []progen.Site
	for _, name := range declared(tg, func(d *syntax.TypeDecl) *syntax.Ident { return d.Name }) {
		out = append(out, appendDecl(tg, "amend ", name, " {\n}"))
	}
	return out
}

// inputFields are the input fields of a source file.
func inputFields(tg target) []*syntax.FieldDecl {
	var out []*syntax.FieldDecl
	for _, f := range nodes[*syntax.FieldDecl](tg) {
		if isSource(tg) && f.Input.Valid() {
			out = append(out, f)
		}
	}
	return out
}

func inputField(f func(target, *syntax.FieldDecl) progen.Site) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		var out []progen.Site
		for _, fd := range inputFields(tg) {
			out = append(out, f(tg, fd))
		}
		return out
	}
}

func replaceType(t string) func(target, *syntax.FieldDecl) progen.Site {
	return func(tg target, f *syntax.FieldDecl) progen.Site {
		s, e := span(tg, f.Type)
		return site(replace(s, e, t))
	}
}

func inputDefault(tg target, f *syntax.FieldDecl) progen.Site {
	_, e := span(tg, f.Env)
	return seq(1, insert(e, " = "), insert(e, "none"))
}

func badEnvName(tg target, f *syntax.FieldDecl) progen.Site {
	s, e := span(tg, f.Env)
	return site(replace(s, e, `"1KEY"`))
}

func inputRecordTwice(tg target) []progen.Site {
	var out []progen.Site
	for _, r := range nodes[*syntax.RecordDecl](tg) {
		if r.Body == nil || !hasInput(r.Body) {
			continue
		}
		s, e := span(tg, r.Name)
		end := declEnd(tg)
		second := site(mark(tg, s, e), insert(end, "\n\n/// A second holder.\nlet zzSecond: "+r.Name.Name+" = {}\n"))
		out = append(out, keeping(tg, second, holders(tg, r.Name.Name)...))
	}
	return out
}

// holders are what reaches record name in tg already: the fields typed with it, and the lets
// typed with a record holding one (the first path E1903 counts).
func holders(tg target, name string) []syntax.Node {
	var out []syntax.Node
	owners := map[string]bool{name: true}
	for _, r := range nodes[*syntax.RecordDecl](tg) {
		for _, f := range fieldsOf(r) {
			if textOr(tg, f.Type) == name {
				out = append(out, f.Name)
				owners[r.Name.Name] = true
			}
		}
	}
	for _, d := range nodes[*syntax.LetDecl](tg) {
		if owners[textOr(tg, d.Type)] {
			out = append(out, d.Name)
		}
	}
	return out
}

func fieldsOf(r *syntax.RecordDecl) []*syntax.FieldDecl {
	var out []*syntax.FieldDecl
	if r.Body == nil {
		return nil
	}
	for _, it := range r.Body.Items {
		if f, ok := it.(*syntax.FieldDecl); ok {
			out = append(out, f)
		}
	}
	return out
}

func hasInput(b *syntax.RecordBody) bool {
	for _, it := range b.Items {
		if f, ok := it.(*syntax.FieldDecl); ok && f.Input.Valid() {
			return true
		}
	}
	return false
}

// amendPath applies f to every amendment of a layer file.
func amendPath(f func(target, *syntax.Amendment) progen.Site) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		if !isLayer(tg) {
			return nil
		}
		var out []progen.Site
		for _, a := range nodes[*syntax.Amendment](tg) {
			if len(a.Path) > 0 {
				out = append(out, f(tg, a))
			}
		}
		return out
	}
}

func renameLastSegment(tg target, a *syntax.Amendment) progen.Site {
	last := a.Path[len(a.Path)-1]
	s, e := span(tg, last)
	first, _ := span(tg, a.Path[0])
	return seq(0, mark(tg, first, s), replace(s, e, "zz"+text(tg, last)))
}

func duplicateAmendment(tg target, a *syntax.Amendment) progen.Site {
	s, e := span(tg, a)
	at := lineEnd(tg, e) + 1
	return keeping(tg, seq(1, insert(at, indent(tg, s)), insert(at, text(tg, a)), insert(at, "\n")), a)
}

// unknownLayer checks a package with a layer no package declares; E1901 names no place.
func unknownLayer(tg target) []progen.Site {
	if !isSource(tg) || tg.file.Package == nil {
		return nil
	}
	s, e := span(tg, tg.file.Package)
	return []progen.Site{{Edits: []progen.Edit{mark(tg, s, e)}, Layers: []string{"zznowhere"}}}
}

func layerTwice(tg target) []progen.Site {
	if !isLayer(tg) || tg.file.Layer == nil || tg.file.Package == nil {
		return nil
	}
	name := path.Join(path.Dir(tg.path), "zz.layer.canon")
	body := "package " + text(tg, tg.file.Package) + "\nlayer "
	return []progen.Site{{Path: name, Edits: []progen.Edit{insert(0, body), insert(0, tg.file.Layer.Name), insert(0, "\n")}, Focus: 1}}
}

// amendForeign imports a public let of another package into tg's package and adds a layer
// amending it: the name is in scope, and of another package.
func amendForeign(tg target) []progen.Site {
	if !isSource(tg) || tg.file.Package == nil || tg.path != peers(tg)[0].path {
		return nil
	}
	var out []progen.Site
	taken := takenNames(tg)
	_, pe := span(tg, tg.file.Package)
	layer := path.Join(path.Dir(tg.path), "zz.layer.canon")
	head := "package " + text(tg, tg.file.Package) + "\nlayer zz\n\namend "
	for _, other := range *tg.all {
		if other.pkg == tg.pkg || !isSource(other) || slices.Contains(dependents(tg), other.pkg) {
			continue
		}
		for _, d := range nodes[*syntax.LetDecl](other) {
			if d.Mods != nil && d.Mods.Local.Valid() || taken[d.Name.Name] {
				continue
			}
			imp := "\n\nimport " + other.pkg + " { " + d.Name.Name + " }"
			src := string(tg.src[:pe]) + imp + string(tg.src[pe:])
			out = append(out, progen.Site{
				Path:  layer,
				Edits: []progen.Edit{insert(0, head), insert(0, d.Name.Name), insert(0, " {\n}\n")},
				Focus: 1,
				Add:   map[string][]byte{tg.path: []byte(src)},
			})
		}
	}
	return out
}

// takenNames are the names tg binds or imports, which no import may bind again (TYPES.md §3.1).
func takenNames(tg target) map[string]bool {
	out := boundNames(tg)
	for _, imp := range tg.file.Imports {
		for _, n := range append(slices.Clone(imp.Names), imp.Alias) {
			if n != nil {
				out[n.Name] = true
			}
		}
	}
	return out
}

func twoPackagesOneDir(tg target) []progen.Site {
	if tg.path != projectFile {
		return nil
	}
	return []progen.Site{{
		Path:     "zz/y/w/two.canon",
		Edits:    []progen.Edit{insert(0, "package "), insert(0, "zz.y"), insert(0, "\n")},
		Focus:    1,
		Add:      map[string][]byte{"zz/y/w/one.canon": []byte("package zz\n")},
		Packages: []string{"zz.y"},
	}}
}

// dependents are the packages of the corpus that import tg's package, directly or not: an
// import of one of them would close a cycle (E2002).
func dependents(tg target) []string {
	out := []string{}
	for next := []string{tg.pkg}; len(next) > 0; {
		pkg := next[0]
		next = next[1:]
		for _, other := range *tg.all {
			if slices.Contains(out, other.pkg) || !importsPackage(other, pkg) {
				continue
			}
			out = append(out, other.pkg)
			next = append(next, other.pkg)
		}
	}
	return out
}

// importsPackage tells a file that imports pkg.
func importsPackage(tg target, pkg string) bool {
	for _, imp := range nodes[*syntax.Import](tg) {
		if text(tg, imp.Path) == pkg {
			return true
		}
	}
	return false
}

// importers are the packages of the corpus that import tg's package.
func importers(tg target) []string {
	var out []string
	for _, other := range *tg.all {
		for _, imp := range nodes[*syntax.Import](other) {
			if other.pkg != tg.pkg && text(other, imp.Path) == tg.pkg {
				out = append(out, other.pkg)
			}
		}
	}
	return out
}

func importCycle(tg target) []progen.Site {
	if !isSource(tg) || tg.file.Package == nil {
		return nil
	}
	var out []progen.Site
	for _, q := range importers(tg) {
		if tokenExists(tg, path.Base(strings.ReplaceAll(q, ".", "/"))) {
			continue // its alias would be bound twice (E2005)
		}
		_, e := span(tg, tg.file.Package)
		out = append(out, seq(1, insert(e, "\n\nimport "), insert(e, q), insert(e, "\n")))
	}
	return out
}

// afterPackage inserts before+focus+after on a line of its own after the package clause and
// the imports.
func afterPackage(before, focus, after string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		if !isSource(tg) || tg.file.Package == nil {
			return nil
		}
		_, e := span(tg, tg.file.Package)
		if n := len(tg.file.Imports); n > 0 {
			_, e = span(tg, tg.file.Imports[n-1])
		}
		return []progen.Site{seq(1, insert(e, "\n"+before), insert(e, focus), insert(e, after))}
	}
}

func importName(name string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		return sitesOf(tg, func(i *syntax.Import) bool { return len(i.Names) > 0 }, func(i *syntax.Import) progen.Site {
			_, e := span(tg, i.Names[len(i.Names)-1])
			return seq(1, insert(e, ", "), insert(e, name))
		})
	}
}

func importTwice(tg target) []progen.Site {
	return sitesOf(tg, func(i *syntax.Import) bool { return len(i.Names) > 0 }, func(i *syntax.Import) progen.Site {
		_, e := span(tg, i)
		line := "\nimport " + text(tg, i.Path) + " { "
		return keeping(tg, seq(1, insert(e, line), insert(e, i.Names[0].Name), insert(e, " }")), i.Names[0])
	})
}

func descendantPackage(tg target) []progen.Site {
	if !isSource(tg) || tg.file.Package == nil {
		return nil
	}
	name := path.Join(path.Dir(tg.path), "zz.canon")
	pkg := tg.pkg + ".zz"
	return []progen.Site{{Path: name, Edits: []progen.Edit{insert(0, "package "), insert(0, pkg), insert(0, "\n")}, Focus: 1, Packages: []string{pkg}}}
}

// enumMembers maps each member name of tg's package's enums to its enum.
func enumMembers(tg target) map[string]string {
	out := map[string]string{}
	for _, p := range peers(tg) {
		for _, e := range nodes[*syntax.EnumDecl](p) {
			for _, m := range e.Members {
				out[m.Name.Name] = e.Name.Name
			}
		}
	}
	return out
}

// shadowMember declares, first in a function body, a local named like an enum member that the
// body names bare once: the bare use becomes ambiguous.
func shadowMember(tg target) []progen.Site {
	members := enumMembers(tg)
	var out []progen.Site
	for _, f := range nodes[*syntax.FnDecl](tg) {
		if f.Body == nil || len(f.Body.Stmts) == 0 || !startsLine(tg, f.Body.Stmts[0]) {
			continue
		}
		uses := map[string][]*syntax.IdentExpr{}
		syntax.Inspect(f.Body, func(n syntax.Node) bool {
			if id, ok := n.(*syntax.IdentExpr); ok && members[id.Name] != "" {
				uses[id.Name] = append(uses[id.Name], id)
			}
			return true
		})
		first, _ := span(tg, f.Body.Stmts[0])
		for _, name := range slices.Sorted(maps.Keys(uses)) {
			if ids := uses[name]; len(ids) == 1 {
				s, e := span(tg, ids[0])
				decl := "let " + name + " = " + members[name] + "." + name + "\n" + indent(tg, first)
				out = append(out, site(mark(tg, s, e), insert(first, decl)))
			}
		}
	}
	return out
}

// boundNames are the names a file binds: parameters, locals, loop and lambda variables, and its
// package's top-level lets and consts.
func boundNames(tg target) map[string]bool {
	out := map[string]bool{}
	for _, p := range nodes[*syntax.Param](tg) {
		out[p.Name.Name] = true
	}
	for _, l := range nodes[*syntax.LetStmt](tg) {
		out[l.Name.Name] = true
	}
	for _, name := range declared(tg, func(d *syntax.LetDecl) *syntax.Ident { return d.Name }) {
		out[name] = true
	}
	for _, name := range declared(tg, func(d *syntax.ConstDecl) *syntax.Ident { return d.Name }) {
		out[name] = true
	}
	return out
}

// unknownName misspells a bound name in a body, never beside a context-dependent operand (TYPES.md §5.1).
func unknownName(tg target) []progen.Site {
	bound := boundNames(tg)
	skip := contextOperands(tg, bound)
	var out []progen.Site
	for _, b := range nodes[*syntax.Block](tg) {
		syntax.Inspect(b, func(n syntax.Node) bool {
			if id, ok := n.(*syntax.IdentExpr); ok && bound[id.Name] && !skip[id] {
				s, e := span(tg, id)
				out = append(out, site(replace(s, e, "zz"+id.Name)))
			}
			return true
		})
	}
	return out
}

// checkedPairs are the comparison and arithmetic operators, whose operands TYPES.md §5.1 types as a pair.
var checkedPairs = []syntax.TokenKind{
	syntax.TokEq, syntax.TokNe, syntax.TokLt, syntax.TokLe, syntax.TokGt, syntax.TokGe,
	syntax.TokPlus, syntax.TokMinus, syntax.TokStar, syntax.TokSlash, syntax.TokPercent,
}

// contextOperands are the names beside a context-dependent operand, and those left of `in` (TYPES.md §5.1).
func contextOperands(tg target, bound map[string]bool) map[*syntax.IdentExpr]bool {
	skip := map[*syntax.IdentExpr]bool{}
	for _, b := range nodes[*syntax.BinaryExpr](tg) {
		x, y := bareIdent(b.X), bareIdent(b.Y)
		switch {
		case b.Op == syntax.KwIn:
			skip[x] = true
		case slices.Contains(checkedPairs, b.Op):
			skip[x] = skip[x] || contextDependent(b.Y, bound)
			skip[y] = skip[y] || contextDependent(b.X, bound)
		}
	}
	delete(skip, nil)
	return skip
}

// bareIdent is e's name through parentheses and unary operators, nil when e is not a name.
func bareIdent(e syntax.Expr) *syntax.IdentExpr {
	switch n := e.(type) {
	case *syntax.IdentExpr:
		return n
	case *syntax.ParenExpr:
		return bareIdent(n.X)
	case *syntax.UnaryExpr:
		return bareIdent(n.X)
	}
	return nil
}

// contextDependent tells an operand TYPES.md §5.1 calls context-dependent (an unbound name counts).
func contextDependent(e syntax.Expr, bound map[string]bool) bool {
	switch n := e.(type) {
	case *syntax.IdentExpr:
		return !bound[n.Name]
	case *syntax.NoneLit, *syntax.ListLit, *syntax.BraceLit, *syntax.IntLit, *syntax.FloatLit:
		return true
	case *syntax.ParenExpr:
		return contextDependent(n.X, bound)
	case *syntax.UnaryExpr:
		return contextDependent(n.X, bound)
	}
	return false
}

// uncollected are the records of tg's package no let or field holds a collection of (TYPES.md §10.2).
func uncollected(tg target) []string {
	var held []string
	for _, p := range peers(tg) {
		for _, d := range nodes[*syntax.LetDecl](p) {
			if d.Type != nil {
				held = append(held, text(p, d.Type))
			}
		}
		for _, t := range nodes[*syntax.TableType](p) {
			held = append(held, text(p, t.Name))
		}
		for _, k := range nodes[*syntax.KeyedType](p) {
			held = append(held, text(p, k.List))
		}
	}
	var out []string
	for _, name := range declared(tg, func(d *syntax.RecordDecl) *syntax.Ident { return d.Name }) {
		if !strings.Contains(strings.Join(held, " "), name) {
			out = append(out, name)
		}
	}
	return out
}

func refWithoutCollection(tg target) []progen.Site {
	var out []progen.Site
	for _, name := range uncollected(tg) {
		for _, r := range nodes[*syntax.RefType](tg) {
			s, e := span(tg, r.Name)
			out = append(out, site(replace(s, e, name)))
		}
	}
	return out
}

func fieldAndMethod(tg target) []progen.Site {
	var out []progen.Site
	for _, r := range nodes[*syntax.RecordDecl](tg) {
		if r.Body == nil || len(r.Body.Items) == 0 {
			continue
		}
		f, ok := r.Body.Items[0].(*syntax.FieldDecl)
		if !ok {
			continue
		}
		_, e := span(tg, r.Body.Items[len(r.Body.Items)-1])
		ind := indent(tg, e)
		out = append(out, seq(1, insert(e, "\n\n"+ind+"fn "), insert(e, f.Name.Name), insert(e, "(self) -> Int { return 1 }")))
	}
	return out
}

// tableElements are the record names of tg's package held by a table let.
func tableElements(tg target) map[string]bool {
	out := map[string]bool{}
	for _, p := range peers(tg) {
		for _, t := range nodes[*syntax.TableType](p) {
			out[text(p, t.Name)] = true
		}
	}
	return out
}

func idOnTableElement(tg target) []progen.Site {
	elems := tableElements(tg)
	return sitesOf(tg, func(r *syntax.RecordDecl) bool {
		return elems[r.Name.Name] && r.Body != nil && len(r.Body.Items) > 0 && startsLine(tg, r.Body.Items[0])
	}, func(r *syntax.RecordDecl) progen.Site {
		s, _ := span(tg, r.Body.Items[0])
		ind := indent(tg, s)
		at := lineStart(tg, s)
		if f, ok := r.Body.Items[0].(*syntax.FieldDecl); ok && f.Doc != nil {
			at = lineStart(tg, int(f.Doc.Start))
		}
		return seq(1, insert(at, ind+"/// Reserved.\n"+ind), insert(at, "id"), insert(at, ": Int = 0\n"))
	})
}

func constTwice(tg target) []progen.Site {
	return sitesOf(tg, func(d *syntax.ConstDecl) bool { return isSource(tg) && endsLine(tg, d) }, func(d *syntax.ConstDecl) progen.Site {
		s, e := span(tg, d)
		ns, _ := span(tg, d.Name)
		at := lineEnd(tg, e) + 1
		return keeping(tg, seq(1, insert(at, string(tg.src[s:ns])), insert(at, d.Name.Name), insert(at, string(tg.src[ns+len(d.Name.Name):e])+"\n")), d.Name)
	})
}

func localTwice(tg target) []progen.Site {
	return sitesOf(tg, func(l *syntax.LetStmt) bool { return startsLine(tg, l) && endsLine(tg, l) }, func(l *syntax.LetStmt) progen.Site {
		s, e := span(tg, l)
		ns, ne := span(tg, l.Name)
		at := lineEnd(tg, e) + 1
		return keeping(tg, seq(2, insert(at, indent(tg, s)), insert(at, string(tg.src[s:ns])), insert(at, l.Name.Name),
			insert(at, string(tg.src[ne:e])+"\n")), l.Name)
	})
}

// topFnStatement inserts before+focus as the first statement of every top-level function.
func topFnStatement(before, focus string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		if !isSource(tg) {
			return nil
		}
		var out []progen.Site
		for _, d := range tg.file.Decls {
			f, ok := d.(*syntax.FnDecl)
			if !ok || f.Body == nil || len(f.Body.Stmts) == 0 || !startsLine(tg, f.Body.Stmts[0]) {
				continue
			}
			s, _ := span(tg, f.Body.Stmts[0])
			out = append(out, seq(1, insert(s, before), insert(s, focus), insert(s, "\n"+indent(tg, s))))
		}
		return out
	}
}
