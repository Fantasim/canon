package eval

import (
	"strings"

	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// evalString is a template: values visited, then a byte each, before each part (STDLIB.md §9.1).
func evalString(r *run, e syntax.Expr, _ *vpath) value.Value {
	x := e.(*syntax.StringLit)
	kind := value.ProvLiteral
	if len(x.Parts) > 1 || len(x.Parts) == 1 && x.Parts[0].Interp != nil {
		kind = value.ProvComputed
	}
	var b strings.Builder
	for _, part := range x.Parts {
		if part.Interp == nil {
			if kind == value.ProvComputed && !r.spend(len(part.Text), func() source.Span { return r.span(e) }) {
				return nil
			}
			b.WriteString(part.Text)
			continue
		}
		text, ok := r.interpolated(part.Interp)
		if !ok {
			return nil
		}
		b.WriteString(text)
	}
	return &value.Str{V: b.String(), T: types.StringType, P: r.prov(e, kind)}
}

// interpolated is the text of one interpolated value, its values visited and its bytes
// charged before it is made.
func (r *run) interpolated(in *syntax.Interp) (string, bool) {
	v := r.eval(in.X)
	at := func() source.Span { return r.span(in) }
	if v == nil || !r.spend(std.VisitedUpTo(v, r.remaining()), at) {
		return "", false
	}
	if in.Spec != nil {
		text := std.Format(v, std.Spec{Plus: in.Spec.Plus, Comma: in.Spec.Comma, Decimals: in.Spec.Decimals})
		return text, r.spend(len(text), at)
	}
	if !r.spend(value.TextLenUpTo(v, r.remaining()), at) {
		return "", false
	}
	return v.CanonText(), true
}

// templateText is a template's source without its own delimiters (EVALUATION.md §8.4).
func templateText(f *syntax.File, m syntax.StrLit) string {
	sp := f.Span(m)
	text := string(f.Src.Content[sp.Start:sp.End])
	open, closing := quote, quote
	switch x := m.(type) {
	case *syntax.StringLit:
		if x.Multiline {
			open, closing = tripleQuote, tripleQuote
		}
	case *syntax.RawStringLit:
		open = rawPrefix + quote
		if x.Multiline {
			open, closing = rawPrefix+tripleQuote, tripleQuote
		}
	}
	return strings.TrimSuffix(strings.TrimPrefix(text, open), closing)
}
