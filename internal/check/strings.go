package check

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// interpolations types every `{x}` of a string (STDLIB.md §9).
func (c *checker) interpolations(env *env, s *syntax.StringLit) {
	for _, p := range s.Parts {
		if p.Interp == nil {
			continue
		}
		c.interpolation(env, p.Interp)
	}
}

func (c *checker) interpolation(env *env, in *syntax.Interp) {
	t := c.synth(env, in.X)
	switch {
	case t.Kind() == types.Error:
	case t.Base().Kind() == types.Func:
		c.report(env, diag.E4503.AtPlain(env.span(in.X), t))
	case in.Spec != nil && !formattable(t, in.Spec):
		c.report(env, diag.E4503.AtSpec(env.span(in.X), t, specText(env, in.Spec)))
	}
}

// formattable reports Int, or Float without `x`/`X`, of any width (STDLIB.md §9.5).
func formattable(t types.Type, spec *syntax.FormatSpec) bool {
	k := t.Base().Kind()
	return k == types.Int || (k == types.Float && spec.Hex == 0)
}

// specText is a format spec as written, without its colon.
func specText(env *env, spec *syntax.FormatSpec) string {
	tok := env.file.Tokens[spec.Tok]
	return strings.TrimPrefix(string(env.file.Src.Content[tok.Start:tok.End]), colon)
}
