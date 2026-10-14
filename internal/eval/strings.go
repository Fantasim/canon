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
	position := false
	for _, part := range x.Parts {
		if part.Interp == nil {
			if kind == value.ProvComputed && !r.spend(len(part.Text), func() source.Span { return r.span(e) }) {
				return nil
			}
			b.WriteString(part.Text)
			continue
		}
		text, marked, ok := r.interpolated(part.Interp)
		if !ok {
			return nil
		}
		position = position || marked
		b.WriteString(text)
	}
	v := &value.Str{V: b.String(), T: types.StringType, P: r.provMarked(e, kind, position)}
	if kind == value.ProvLiteral {
		return r.literal(v)
	}
	return v
}

// interpolated is the text of one interpolated value, charged before it is made, and whether the value is marked.
func (r *run) interpolated(in *syntax.Interp) (string, bool, bool) {
	v := r.eval(in.X)
	at := func() source.Span { return r.span(in) }
	if v == nil || !r.spend(std.VisitedUpTo(v, r.remaining()), at) {
		return "", false, false
	}
	marked := value.IsPosition(v)
	if in.Spec != nil {
		f := std.Prepare(v, *in.Spec)
		if !r.spend(f.Len(), at) {
			return "", false, false
		}
		return f.String(), marked, true
	}
	if !r.spend(value.TextLenUpTo(v, r.remaining()), at) {
		return "", false, false
	}
	return v.CanonText(), marked, true
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
