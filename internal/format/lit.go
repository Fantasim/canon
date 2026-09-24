package format

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// stringLit is a string as written, a multiline one with its lines re-based (§3, §10).
func (b *builder) stringLit(n syntax.Node, multi bool) *doc {
	src := b.f.Src.Content[b.f.Tokens[n.First()].Start:b.f.Tokens[n.Last()].End]
	lines := strings.Split(string(src), newlineText)
	last := len(lines) - 1
	if !multi || last == 0 || !strings.HasSuffix(lines[last], tripleQuote) {
		return text(string(src))
	}
	prefix := strings.TrimSuffix(lines[last], tripleQuote)
	out := make([]string, 0, len(lines))
	out = append(out, strings.TrimRight(lines[0], trailingBlanks))
	for _, l := range lines[1:last] {
		body, ok := strings.CutPrefix(l, prefix)
		if !ok {
			body = ""
		}
		out = append(out, body)
	}
	return multiline(append(out, tripleQuote))
}

// duration is a duration literal in its canonical text (FORMATTER.md §10), after its folded "-".
func (b *builder) duration(n *syntax.DurationLit) *doc {
	if n.First() != n.Last() {
		return cat(b.tok(n.First()), b.word(n.Last(), durationText(n.Millis)))
	}
	return b.word(n.Last(), durationText(n.Millis))
}

// durationText is the canonical text of |v| milliseconds. It works on -|v|, which always
// fits, and each unit's count is small enough to negate.
func durationText(v int64) string {
	if v == 0 {
		return zeroDuration
	}
	v = min(v, -v)
	var sb strings.Builder
	for _, u := range durationUnits {
		if q := v / u.millis; q != 0 {
			sb.WriteString(strconv.FormatInt(-q, decimalBase))
			sb.WriteString(u.name)
		}
		v %= u.millis
	}
	return sb.String()
}

// braceLit is a brace list, or a brace comprehension broken before each clause (§7.2).
func (b *builder) braceLit(n *syntax.BraceLit) *doc {
	if len(n.Clauses) == 0 {
		return b.braceList(n.First(), n.Last(), entries(b, n.Items))
	}
	var ds []*doc
	ds = append(ds, lineDoc, b.node(n.Items[0]))
	for _, c := range n.Clauses {
		ds = append(ds, lineDoc, b.node(c))
	}
	ds = append(ds, b.closing(n.Last(), true))
	single := b.oneLine(n.First(), n.Last()) && !b.inner(n.First(), n.Last())
	return listGroup(single, b.tok(n.First()), indent(ds...), lineDoc, b.closer(n.Last()))
}

// listLit is PL("[", elements, "]") (§7.2).
func (b *builder) listLit(n *syntax.ListLit) *doc {
	return b.parenList(n.First(), n.Last(), partsOf(b, n.Elems))
}

// listComp breaks before each clause (§6.2, §7.2).
func (b *builder) listComp(n *syntax.ListComp) *doc {
	var ds []*doc
	ds = append(ds, softlineDoc, b.node(n.Elem))
	for _, c := range n.Clauses {
		ds = append(ds, lineDoc, b.node(c))
	}
	ds = append(ds, b.closing(n.Last(), true))
	return group(false, b.tok(n.First()), indent(ds...), softlineDoc, b.closer(n.Last()))
}

// compClause is "for vars in x", "if x" or "let v = x".
func (b *builder) compClause(n *syntax.CompClause) *doc {
	ds := []*doc{b.tok(n.First()), spaceDoc}
	if len(n.Vars) > 0 {
		ds = append(ds, b.commaList(nodes(n.Vars)), spaceDoc, b.tok(b.before(n.X.First())), spaceDoc)
	}
	return cat(append(ds, b.node(n.X))...)
}

func (b *builder) fieldItem(n *syntax.FieldItem) *doc {
	return cat(b.node(n.Name), b.assign(b.after(n.Name.Last()), n.Value))
}

func (b *builder) mapItem(n *syntax.MapItem) *doc {
	return cat(b.node(n.Key), b.assign(b.before(n.Value.First()), n.Value))
}

func (b *builder) entryItem(n *syntax.EntryItem) *doc {
	h, _ := b.head(n, nil, n.Mods)
	return cat(h, b.node(n.Key), b.trailingAnnotations(n.Annotations), spaceDoc, b.node(n.Value))
}

func (b *builder) spreadItem(n *syntax.SpreadItem) *doc {
	return cat(b.tok(n.First()), b.node(n.X))
}

func (b *builder) typedLit(n *syntax.TypedLit) *doc {
	return cat(b.node(n.Type), spaceDoc, b.node(n.Lit))
}

func (b *builder) matchArm(n *syntax.MatchArm) *doc { return b.arm(n.Patterns, n.Body) }

func (b *builder) typeArm(n *syntax.TypeArm) *doc { return b.arm(n.Patterns, n.Type) }

// pattern is "_", "none" or "Name[(binder)]".
func (b *builder) pattern(n *syntax.Pattern) *doc {
	if n.Name == nil {
		return b.tok(n.First())
	}
	if n.Parens.Open == syntax.NoTok {
		return b.node(n.Name)
	}
	return cat(b.node(n.Name), b.tok(n.Parens.Open), b.node(n.Binder), b.closing(n.Parens.Close, true), b.closer(n.Parens.Close))
}
