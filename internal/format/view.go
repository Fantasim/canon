package format

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

// viewText is `title "text"`, `subtitle`, `singular` or `plural`.
func (b *builder) viewText(n syntax.Node, s syntax.StrLit) *doc {
	return cat(b.tok(n.First()), spaceDoc, b.node(s))
}

func (b *builder) viewMenu(n *syntax.ViewMenu) *doc {
	return cat(b.tok(n.First()), spaceDoc, b.node(n.Menu), spaceDoc, b.tok(b.before(n.Icon.First())), spaceDoc, b.node(n.Icon))
}

// viewBraces is `columns`, `search` or `filters` with its brace list.
func (b *builder) viewBraces(n syntax.Node, braces syntax.Delims, items []entry) *doc {
	return cat(b.tok(n.First()), spaceDoc, b.braceList(braces.Open, braces.Close, items))
}

func (b *builder) viewColumn(n *syntax.ViewColumn) *doc {
	if n.Width == nil {
		return b.node(n.Name)
	}
	return cat(b.node(n.Name), spaceDoc, b.node(n.Width))
}

func (b *builder) viewFilter(n *syntax.ViewFilter) *doc {
	if n.Multi == syntax.NoTok {
		return b.node(n.Name)
	}
	return cat(b.node(n.Name), spaceDoc, b.tok(n.Multi))
}

func (b *builder) viewPreview(n *syntax.ViewPreview) *doc {
	return cat(b.tok(n.First()), spaceDoc, b.node(n.X))
}

func (b *builder) viewShow(n *syntax.ViewShow) *doc {
	ds := []*doc{b.tok(n.First()), spaceDoc}
	if n.ID != nil {
		ds = append(ds, b.node(n.ID), spaceDoc)
	}
	return cat(append(ds, b.node(n.Label), spaceDoc, b.node(n.Template))...)
}

func (b *builder) viewGroup(n *syntax.ViewGroup) *doc {
	ds := []*doc{b.tok(n.First()), spaceDoc, b.node(n.ID), spaceDoc, b.node(n.Label)}
	if n.Help != nil {
		ds = append(ds, spaceDoc, b.node(n.Help))
	}
	if n.Advanced != syntax.NoTok {
		ds = append(ds, spaceDoc, b.tok(n.Advanced))
	}
	if n.When != nil {
		ds = append(ds, spaceDoc, b.tok(b.before(n.When.First())), spaceDoc, b.node(n.When))
	}
	return cat(append(ds, spaceDoc, b.braceList(n.Braces.Open, n.Braces.Close, entries(b, n.Members)))...)
}

func (b *builder) viewField(n *syntax.ViewField) *doc {
	var ds []*doc
	if n.Field != syntax.NoTok {
		ds = append(ds, b.tok(n.Field), spaceDoc)
	}
	ds = append(ds, b.node(n.Name))
	if n.Label != nil {
		ds = append(ds, spaceDoc, b.node(n.Label))
	}
	if n.Props != nil {
		ds = append(ds, spaceDoc, b.node(n.Props))
	}
	return cat(ds...)
}

// amendBlock is "amend target { items }" in a layer file.
func (b *builder) amendBlock(n *syntax.AmendBlock) *doc {
	return cat(b.tok(n.First()), spaceDoc, b.node(n.Target), spaceDoc, b.braceList(n.Braces.Open, n.Braces.Close, entries(b, n.Items)))
}

// amendment is "path: value" by rule A (§7.2).
func (b *builder) amendment(n *syntax.Amendment) *doc {
	ds := make([]*doc, 0, len(n.Path)+1)
	for _, s := range n.Path {
		ds = append(ds, b.node(s))
	}
	return cat(append(ds, b.assign(b.before(n.Value.First()), n.Value))...)
}

// amendSegment is a word, ".word", "[key]" or "[#position]".
func (b *builder) amendSegment(n *syntax.AmendSegment) *doc {
	switch {
	case n.Name != nil && n.First() == n.Name.First():
		return b.node(n.Name)
	case n.Name != nil:
		return cat(b.tok(n.First()), b.node(n.Name))
	case n.Position != nil:
		return cat(b.tok(n.First()), b.tok(b.before(n.Position.First())), b.node(n.Position), b.closing(n.Last(), true), b.closer(n.Last()))
	default:
		return cat(b.tok(n.First()), b.node(n.Key), b.closing(n.Last(), true), b.closer(n.Last()))
	}
}

func (b *builder) translationEntry(n *syntax.TranslationEntry) *doc {
	return cat(b.node(n.Key), spaceDoc, b.node(n.Text))
}
