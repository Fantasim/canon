package eval

import (
	"strings"

	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// evalString is a string template; formatting costs the values visited (STDLIB.md §9.1).
func evalString(r *run, e syntax.Expr, _ *vpath) value.Value {
	x := e.(*syntax.StringLit)
	var b strings.Builder
	for _, part := range x.Parts {
		if part.Interp == nil {
			b.WriteString(part.Text)
			continue
		}
		v := r.eval(part.Interp.X)
		if v == nil || !r.spend(std.Visited(v), func() source.Span { return r.span(part.Interp) }) {
			return nil
		}
		b.WriteString(interpolated(v, part.Interp.Spec))
	}
	kind := value.ProvLiteral
	if len(x.Parts) > 1 || len(x.Parts) == 1 && x.Parts[0].Interp != nil {
		kind = value.ProvComputed
	}
	return &value.Str{V: b.String(), T: types.StringType, P: r.prov(e, kind)}
}

// interpolated is the text of one interpolated value.
func interpolated(v value.Value, spec *syntax.FormatSpec) string {
	if spec != nil {
		return std.Format(v, std.Spec{Plus: spec.Plus, Comma: spec.Comma, Decimals: spec.Decimals})
	}
	return v.CanonText()
}

// templateText is a template's source without its quotes (EVALUATION.md §8.4).
func templateText(f *syntax.File, m syntax.StrLit) string {
	sp := f.Span(m)
	text := string(f.Src.Content[sp.Start:sp.End])
	return strings.Trim(text, quote)
}
