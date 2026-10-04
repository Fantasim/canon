package check

import (
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// textSite is one `@text` annotation at a top-level declaration of a package.
type textSite struct {
	env  *env
	decl syntax.Decl
	ann  *syntax.Annotation
}

// checkTexts is E8021 for p's `@text`s and reports whether p has one, for `textEmpty` (CODEGEN.md §2.9).
func (c *checker) checkTexts(p *pkgState) bool {
	sites := c.textSites(p)
	seen := map[string]string{} // a file name in lower case, to the fn writing it
	for _, s := range sites {
		file, ok := c.textFile(s)
		if !ok {
			continue
		}
		key := strings.ToLower(file)
		if prev, twice := seen[key]; twice {
			v := positional(s.ann)
			c.report(s.env, diag.E8021.AtTwice(s.env.span(v), writtenText(s.env.file, v), prev))
			continue
		}
		seen[key] = s.decl.(*syntax.FnDecl).Name.Name
	}
	return len(sites) > 0
}

// textSites are the `@text`s before p's top-level declarations; elsewhere is E1118 (GRAMMAR.md §8.3).
func (c *checker) textSites(p *pkgState) []textSite {
	var out []textSite
	for _, f := range p.files {
		for _, d := range f.Decls {
			if a := annotation(prefixAnnotations(d), syntax.AnnText); a != nil {
				out = append(out, textSite{env: c.fileEnv(p, f, nil), decl: d, ann: a})
			}
		}
	}
	return out
}

// prefixAnnotations are the annotations written before a declaration: its first children
// after its doc comment; a record, enum or variant has its own after its name.
func prefixAnnotations(d syntax.Decl) []*syntax.Annotation {
	var out []*syntax.Annotation
	for n := range syntax.Children(d) {
		switch a := n.(type) {
		case *syntax.DocComment:
		case *syntax.Annotation:
			out = append(out, a)
		default:
			return out
		}
	}
	return out
}

// textFile is a well-placed `@text`'s valid file name, else E8021 `position` or `file`; a name
// the parser refused (E1119) or a result type in error has its own finding.
func (c *checker) textFile(s textSite) (string, bool) {
	placed, known := c.textPlaced(s)
	if !known {
		return "", false
	}
	if !placed {
		c.report(s.env, diag.E8021.AtPosition(s.env.span(s.ann)))
		return "", false
	}
	v := positional(s.ann)
	x, isExpr := v.(syntax.Expr)
	file, ok := c.annString(v)
	if !ok || !isExpr || !interpolationFree(x) {
		return "", false
	}
	if !validTextName(file) {
		c.report(s.env, diag.E8021.AtFile(s.env.span(v), writtenText(s.env.file, v)))
		return "", false
	}
	return file, true
}

// textPlaced reports a `@text` at a public package-level export fn with no parameter whose
// result is String; known is false when the fn's result type is in error.
func (c *checker) textPlaced(s textSite) (placed, known bool) {
	d, isFn := s.decl.(*syntax.FnDecl)
	if !isFn || d.Name == nil {
		return false, true
	}
	if d.Mods == nil || d.Mods.Local.Valid() || !d.Mods.Export.Valid() || len(d.Params) > 0 || d.Self.Valid() {
		return false, true
	}
	o := s.env.pkg.names[d.Name.Name]
	if o == nil || o.decl != d {
		return false, false
	}
	sig, ok := o.typ.(*types.FuncType)
	if !ok || sig.Result == nil || isErrorTyped(sig.Result) {
		return false, false
	}
	return sig.Result.Base().Kind() == types.String, true
}

// validTextName reports a portable `@text` file name (CODEGEN.md §2.9, DECISIONS 297).
func validTextName(name string) bool {
	switch {
	case name == "" || name == dot || name == parentDir:
		return false
	case strings.HasSuffix(name, dot) || strings.HasSuffix(name, spaceSep):
		return false
	case strings.ContainsAny(name, slash+string(backslash)+unportableChars) || strings.ContainsFunc(name, unicode.IsControl):
		return false
	}
	stem, _, _ := strings.Cut(name, dot)
	return !windowsDevices[strings.ToUpper(stem)]
}

// writtenText is a string literal as written, between its delimiters: escapes kept, so no control character reaches a terminal (ERRORS.md §1, Text).
func writtenText(f *syntax.File, v syntax.Node) string {
	s := strings.TrimPrefix(writtenIn(f, v), rawMark)
	delim := quoteMark
	if triple := quoteMark + quoteMark + quoteMark; strings.HasPrefix(s, triple) {
		delim = triple
	}
	return strings.TrimSuffix(strings.TrimPrefix(s, delim), delim)
}
