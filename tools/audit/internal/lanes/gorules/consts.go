package gorules

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"maps"
	"path"
	"slices"
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
)

// constPlacement reports const blocks, and package-level literal-only var blocks, outside
// <pkg>/constants.go. Error sentinels are err-placement's; a SQL statement stays beside the
// function that runs it.
func (s *scan) constPlacement() {
	for _, f := range s.files {
		if f.TestCode() || f.base() == constantsFile || f.base() == errorsFile {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			g, ok := n.(*ast.GenDecl)
			if !ok {
				return true
			}
			top := slices.Contains(f.AST.Decls, ast.Decl(g))
			if g.Tok == token.CONST || g.Tok == token.VAR && top && literalVars(f, g) {
				s.placed(f, g, top)
			}
			return false
		})
	}
}

// placed reports one misplaced block, named by its first name, counting its specs that are
// not SQL statements; a function-local block is named by its function, which the runner
// fills in.
func (s *scan) placed(f *src, g *ast.GenDecl, top bool) {
	first := firstName(g)
	n := len(g.Specs) - s.sqlSpecs(f, g)
	if first == "" || n == 0 {
		return
	}
	target := path.Join(f.Dir, constantsFile)
	fd := finding.Finding{
		Rule: ruleConstPlace, File: f.Path, Line: s.line(g.Pos()), Detail: g.Tok.String() + " " + first,
		Value: n, Message: fmt.Sprintf(msgConstPlace, g.Tok, n, target),
		Fix: fmt.Sprintf(fixMoveTo, target),
	}
	if top {
		fd.Symbol = fd.Detail
	}
	s.emit(fd)
}

// sqlSpecs counts g's specs whose every value folds to a SQL statement.
func (s *scan) sqlSpecs(f *src, g *ast.GenDecl) int {
	s.consts()
	ev := s.evals[f.Dir]
	if ev == nil {
		ev = newEvaluator()
	}
	n := 0
	for _, sp := range g.Specs {
		vs, ok := sp.(*ast.ValueSpec)
		if ok && len(vs.Values) > 0 && !slices.ContainsFunc(vs.Values, func(v ast.Expr) bool {
			str, ok := ev.sqlText(v)
			return !ok || !sqlStatement(str)
		}) {
			n++
		}
	}
	return n
}

// sqlStatement reports a whole SQL statement: after blanks and SQL comments, its first word
// is a statement verb, upper- or lower-case (Title case is prose: "Select a server"), and
// the value is more than a run of keywords ("SELECT ", "DELETE FROM").
func sqlStatement(v string) bool {
	rest := sqlLead.ReplaceAllString(v, "")
	m := sqlVerb.FindStringSubmatch(rest)
	return m != nil && (m[1] == strings.ToUpper(m[1]) || m[1] == strings.ToLower(m[1])) && !sqlFragment.MatchString(v)
}

// firstName is the first declared name of a const or var block.
func firstName(g *ast.GenDecl) string {
	for _, sp := range g.Specs {
		if vs, ok := sp.(*ast.ValueSpec); ok && len(vs.Names) > 0 {
			return vs.Names[firstArg].Name
		}
	}
	return ""
}

// literalVars reports a var block whose every spec has values, all built from literals.
func literalVars(f *src, g *ast.GenDecl) bool {
	return len(g.Specs) > 0 && !slices.ContainsFunc(g.Specs, func(sp ast.Spec) bool {
		vs, ok := sp.(*ast.ValueSpec)
		return !ok || !f.literalSpec(vs)
	})
}

func (f *src) literalSpec(vs *ast.ValueSpec) bool {
	return len(vs.Values) > 0 && !slices.ContainsFunc(vs.Values, func(v ast.Expr) bool { return !f.literal(v) })
}

// literal reports an expression built only from literals: basic literals, true/false,
// composites and unary/binary operations of them, and regexp.MustCompile of a literal.
func (f *src) literal(x ast.Expr) bool {
	switch x := x.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return x.Name == identTrue || x.Name == identFalse
	case *ast.ParenExpr:
		return f.literal(x.X)
	case *ast.UnaryExpr:
		return f.literal(x.X)
	case *ast.BinaryExpr:
		return f.literal(x.X) && f.literal(x.Y)
	case *ast.CompositeLit:
		return !slices.ContainsFunc(x.Elts, func(e ast.Expr) bool { return !f.literalElt(e) })
	case *ast.CallExpr:
		return f.calls(x, pkgRegexp, fnMustCompile, fnMustCompilePX) && len(x.Args) == 1 && f.literal(x.Args[firstArg])
	}
	return false
}

func (f *src) literalElt(e ast.Expr) bool {
	kv, ok := e.(*ast.KeyValueExpr)
	if !ok {
		return f.literal(e)
	}
	_, field := kv.Key.(*ast.Ident)
	return (field || f.literal(kv.Key)) && f.literal(kv.Value)
}

// constDup reports every string constant whose value an earlier constant of the same type
// already declares, SQL keyword fragments aside. A one-word value ("status", "Icon") also needs the same name, and two
// enum blocks must share a second member: two enums sharing a word are not one fact.
func (s *scan) constDup() {
	type key struct{ typ, val, name string }
	first := map[key]constDecl{}
	decls := slices.Clone(s.consts())
	slices.SortStableFunc(decls, keepFirst)
	for _, d := range decls {
		v, ok := s.constValue(d)
		if !ok || d.file.TestCode() || trivialString(v) || sqlFragment.MatchString(v) {
			continue
		}
		k := key{typ: s.constType(d), val: v}
		if oneWord(v) {
			k.name = nameKey(d.name.Name)
		}
		orig, seen := first[k]
		if !seen {
			first[k] = d
			continue
		}
		if k.name != "" && s.distinctEnums(orig.decl, d.decl) {
			continue
		}
		fix := fmt.Sprintf(fixConstDup, orig.name.Name, orig.file.Path)
		if std, ok := d.file.stdFix(v); ok {
			fix = fmt.Sprintf(fixUse, std)
		}
		s.emit(finding.Finding{
			Rule: ruleConstDup, File: d.file.Path, Line: s.line(d.name.Pos()), Symbol: constSymbol(d),
			Detail:  clip(finding.Clean(v), detailMax),
			Message: fmt.Sprintf(msgConstDup, clip(v, snippetMax), d.name.Name, orig.name.Name, orig.file.Path, s.line(orig.name.Pos())),
			Fix:     fix,
		})
	}
}

// keepFirst orders the declaration to keep first: in a config package's constants.go, then
// exported, then in any constants.go; ties keep file and position order.
func keepFirst(a, b constDecl) int {
	inConst := func(d constDecl) bool { return d.file.base() == constantsFile }
	inConfig := func(d constDecl) bool { return inConst(d) && path.Base(d.file.Dir) == configDirName }
	exported := func(d constDecl) bool { return ast.IsExported(d.name.Name) }
	return cmp.Or(before(inConfig(a), inConfig(b)), before(exported(a), exported(b)), before(inConst(a), inConst(b)))
}

// before sorts true ahead of false.
func before(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return -1
	}
	return 1
}

// distinctEnums reports two enum blocks (every name sharing one leading word) that share
// fewer than two name and value pairs.
func (s *scan) distinctEnums(a, b *ast.GenDecl) bool {
	pa, pb := s.blockPairs(a), s.blockPairs(b)
	if !enumBlock(pa) || !enumBlock(pb) {
		return false
	}
	shared := 0
	for name, v := range pa {
		if w, ok := pb[name]; ok && w == v {
			shared++
		}
	}
	return shared < pairLen
}

// blockPairs maps each name of a const block (case-folded) to its string value.
func (s *scan) blockPairs(g *ast.GenDecl) map[string]string {
	out := map[string]string{}
	for _, d := range s.consts() {
		if d.decl != g {
			continue
		}
		if v, ok := s.constValue(d); ok {
			out[nameKey(d.name.Name)] = v
		}
	}
	return out
}

func enumBlock(pairs map[string]string) bool {
	lead := ""
	for name := range pairs {
		w := leadWord(name)
		if lead != "" && w != lead {
			return false
		}
		lead = w
	}
	return len(pairs) >= pairLen
}

// leadWord is a camel-case name's first word, case-folded: statusPending -> status.
func leadWord(name string) string {
	i := strings.IndexFunc(name, unicode.IsUpper)
	if i <= 0 {
		return strings.ToLower(name)
	}
	return strings.ToLower(name[:i])
}

// consts lists the repo's const names in file and position order, once per run.
func (s *scan) consts() []constDecl {
	if s.constList == nil {
		s.constList = constDecls(s.files)
		s.evals = evaluators(s.constList)
		slices.SortStableFunc(s.constList, func(a, b constDecl) int {
			return cmp.Or(strings.Compare(a.file.Path, b.file.Path), cmp.Compare(a.name.Pos(), b.name.Pos()))
		})
	}
	return s.constList
}

// constValue folds d's string value; aliases have none of their own.
func (s *scan) constValue(d constDecl) (string, bool) {
	if alias(d.value) {
		return "", false
	}
	return s.evals[d.file.Dir].str(d.value)
}

// constSymbol names a top-level const itself rather than its block's first name.
func constSymbol(d constDecl) string {
	if d.local {
		return ""
	}
	return token.CONST.String() + " " + d.name.Name
}

// constType names a const's declared type; untyped and string are one type.
func (s *scan) constType(d constDecl) string {
	if d.spec.Type == nil {
		return ""
	}
	t := s.source(d.file, d.spec.Type)
	switch {
	case t == identString:
		return ""
	case strings.Contains(t, dirHere):
		return t
	}
	return d.file.Dir + pathSep + t
}

// stdFix is the standard-library constant v spells out in f, if any.
func (f *src) stdFix(v string) (string, bool) {
	if std, ok := stdLayouts[v]; ok {
		return std, true
	}
	std, ok := httpMethods[v]
	return std, ok && slices.Contains(slices.Collect(maps.Values(f.imports)), pkgHTTP)
}

// nameKey folds the case of a name's first rune, so RoleAdmin and roleAdmin are one name.
func nameKey(name string) string { return strings.ToLower(name[:1]) + name[1:] }

// oneWord reports a short identifier-like token with a letter in it: such a value names
// something only together with the constant's name. "2006-01-02" or a UUID stand alone.
func oneWord(v string) bool {
	return len(v) <= maxWordLen && wordish.MatchString(v) && strings.ContainsFunc(v, unicode.IsLetter)
}
