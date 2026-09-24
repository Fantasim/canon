package check

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// annotation is the annotation of a name among anns (E1120 keeps them unique), or nil.
func annotation(anns []*syntax.Annotation, name string) *syntax.Annotation {
	for _, a := range anns {
		if a.Name.Name == name {
			return a
		}
	}
	return nil
}

// named is the value of an annotation's argument name, or nil.
func named(a *syntax.Annotation, name string) syntax.AnnValue {
	if a == nil {
		return nil
	}
	for _, arg := range a.Args {
		if arg.Name != nil && arg.Name.Name == name {
			return arg.Value
		}
	}
	return nil
}

// positional is the first positional argument that is not a flag, or nil.
func positional(a *syntax.Annotation) syntax.AnnValue {
	if a == nil {
		return nil
	}
	for _, arg := range a.Args {
		if arg.Name != nil {
			continue
		}
		if _, isSymbol := arg.Value.(*syntax.QualifiedName); !isSymbol {
			return arg.Value
		}
	}
	return nil
}

// firstArg is the first positional argument, symbol or not, or nil.
func firstArg(a *syntax.Annotation) syntax.AnnValue {
	if a == nil {
		return nil
	}
	for _, arg := range a.Args {
		if arg.Name == nil {
			return arg.Value
		}
	}
	return nil
}

// templateText is a template argument as written: its text, `{name}` for each interpolation.
func (c *checker) templateText(v syntax.AnnValue) (string, bool) {
	s, ok := v.(*syntax.StringLit)
	if !ok {
		return c.annString(v)
	}
	var b strings.Builder
	for _, p := range s.Parts {
		b.WriteString(p.Text)
		if p.Interp == nil {
			continue
		}
		id, isName := p.Interp.X.(*syntax.IdentExpr)
		if !isName {
			return "", false
		}
		b.WriteString(openBrace + id.Name + closeBrace)
	}
	return b.String(), true
}

// flag reports a bare positional symbol (`inline`, `int`, `bits`, `codes`).
func flag(a *syntax.Annotation, name string) bool {
	return flagArg(a, name) != nil
}

// flagArg is the argument of a flag, for its span, or nil.
func flagArg(a *syntax.Annotation, name string) *syntax.AnnotationArg {
	if a == nil {
		return nil
	}
	for _, arg := range a.Args {
		if q, ok := arg.Value.(*syntax.QualifiedName); ok && arg.Name == nil && len(q.Parts) == 1 && q.Parts[0].Name == name {
			return arg
		}
	}
	return nil
}

// symbol is a symbol argument's word, "" when the value is not a one-word symbol.
func symbol(v syntax.AnnValue) string {
	if q, ok := v.(*syntax.QualifiedName); ok && len(q.Parts) == 1 {
		return q.Parts[0].Name
	}
	return ""
}

// annString is a string argument's text, and whether it is one; not one when it holds a
// lexer error, whose text is made up (DECISIONS 215).
func (c *checker) annString(v syntax.AnnValue) (string, bool) {
	s, ok := v.(syntax.StrLit)
	if !ok || c.lexError(v) {
		return "", false
	}
	return constText(s), true
}

// holdsLexError reports an annotation argument, or a list of them, holding a lexer error: what it
// says is unknown (DECISIONS 215).
func (c *checker) holdsLexError(v syntax.AnnValue) bool {
	bad := false
	syntax.Inspect(v, func(n syntax.Node) bool {
		bad = bad || c.lexError(n)
		return !bad
	})
	return bad
}

// deprecation is the reason of a `@deprecated` annotation, and whether there is one.
func (c *checker) deprecation(anns []*syntax.Annotation) (string, bool) {
	a := annotation(anns, annotDeprecated)
	if a == nil {
		return "", false
	}
	why, _ := c.annString(positional(a))
	return why, true
}
