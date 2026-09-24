package format

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

// head is what precedes a declaration's keyword: its prefix annotations, one per line above
// it, then its modifiers; kw is the keyword's token.
func (b *builder) head(n syntax.Node, anns []*syntax.Annotation, mods *syntax.Modifiers) (d *doc, kw syntax.Tok) {
	var ds []*doc
	kw = n.First()
	for _, a := range anns {
		ds = append(ds, b.node(a), hardlineDoc)
		kw = b.after(a.Last())
	}
	if mods != nil {
		ds = append(ds, b.node(mods), spaceDoc)
		kw = b.after(mods.Last())
	}
	return cat(ds...), kw
}

// modifiers are "local", "export" and "retired" as written.
func (b *builder) modifiers(n *syntax.Modifiers) *doc { return b.words(n.First(), n.Last()) }

// words are the tokens from first to last, one space between them.
func (b *builder) words(first, last syntax.Tok) *doc {
	ds := make([]*doc, 0, int(last-first)+1)
	for t := first; t <= last; t = b.after(t) {
		ds = append(ds, b.tok(t))
	}
	return join(ds, spaceDoc)
}

// leaf is a node of adjacent tokens, printed as written: a name, a literal, a folded "-".
func (b *builder) leaf(n syntax.Node) *doc {
	ds := make([]*doc, 0, int(n.Last()-n.First())+1)
	for t := n.First(); t <= n.Last(); t = b.after(t) {
		ds = append(ds, b.tok(t))
	}
	return cat(ds...)
}

// assign is "op value" by rule A, the operator spaced unless it is a colon; a value
// after own-line comments continues one level deeper.
func (b *builder) assign(op syntax.Tok, value syntax.Node) *doc {
	spaced := b.f.Tokens[op].Kind != syntax.TokColon
	v := b.node(value)
	if b.leads(value.First()) {
		v = indent(v)
	}
	return rhs(b.tok(op), v, spaced, isBracket(value))
}

// typed is ": type" after a name.
func (b *builder) typed(colon syntax.Tok, t syntax.Type) *doc {
	return cat(b.tok(colon), spaceDoc, b.node(t))
}

func (b *builder) constDecl(n *syntax.ConstDecl) *doc {
	h, kw := b.head(n, n.Annotations, n.Mods)
	return cat(h, b.tok(kw), spaceDoc, b.node(n.Name), b.assign(b.after(n.Name.Last()), n.Value))
}

func (b *builder) letDecl(n *syntax.LetDecl) *doc {
	h, kw := b.head(n, n.Annotations, n.Mods)
	return cat(h, b.tok(kw), spaceDoc, b.binding(n.Name, n.Type, n.Value))
}

// binding is "name [: type] = value", of a let or var.
func (b *builder) binding(name *syntax.Ident, t syntax.Type, value syntax.Expr) *doc {
	ds := []*doc{b.node(name)}
	eq := b.after(name.Last())
	if t != nil {
		ds = append(ds, b.typed(eq, t))
		eq = b.after(t.Last())
	}
	return cat(append(ds, b.assign(eq, value))...)
}

func (b *builder) typeDecl(n *syntax.TypeDecl) *doc {
	h, kw := b.head(n, n.Annotations, n.Mods)
	ds := []*doc{h, b.tok(kw), spaceDoc, b.node(n.Name)}
	eq := b.after(n.Name.Last())
	if n.Parens.Open != syntax.NoTok {
		ds = append(ds, b.parenList(n.Parens.Open, n.Parens.Close, partsOf(b, n.Params)))
		eq = b.after(n.Parens.Close)
	}
	return cat(append(ds, b.assign(eq, n.Type))...)
}

func (b *builder) param(n *syntax.Param) *doc {
	ds := []*doc{b.node(n.Name), b.typed(b.after(n.Name.Last()), n.Type)}
	if n.Default != nil {
		ds = append(ds, b.assign(b.after(n.Type.Last()), n.Default))
	}
	return cat(ds...)
}

// fnDecl is "fn name(params) -> type block" (§7.2).
func (b *builder) fnDecl(n *syntax.FnDecl) *doc {
	h, kw := b.head(n, n.Annotations, n.Mods)
	params := partsOf(b, n.Params)
	if n.Self != syntax.NoTok {
		params = append([]part{b.tokParts(n.Self)}, params...)
	}
	arrow := b.after(n.Parens.Close)
	return cat(h, b.tok(kw), spaceDoc, b.node(n.Name), b.parenList(n.Parens.Open, n.Parens.Close, params),
		spaceDoc, b.tok(arrow), spaceDoc, b.node(n.Result), spaceDoc, b.node(n.Body))
}

// tokParts is a token as an item of a parenthesized list.
func (b *builder) tokParts(t syntax.Tok) part {
	wasTrail := b.heldTrail[t]
	b.heldTrail[t] = true
	body := b.tok(t)
	b.heldTrail[t] = wasTrail
	return part{body: body, trail: b.trail(t), kept: b.keeps(t)}
}

func (b *builder) entryDecl(n *syntax.EntryDecl) *doc {
	h, kw := b.head(n, n.Annotations, n.Mods)
	return cat(h, b.tok(kw), spaceDoc, b.node(n.Table), b.tok(b.after(n.Table.Last())), b.node(n.Key),
		spaceDoc, b.node(n.Value))
}

// checkDecl is a check block, or a one-line check that breaks before "else" (§7.2).
func (b *builder) checkDecl(n *syntax.CheckDecl) *doc {
	h, kw := b.head(n, n.Annotations, nil)
	if n.Body != nil {
		return cat(h, b.tok(kw), spaceDoc, b.node(n.Body))
	}
	ds := []*doc{b.tok(kw), spaceDoc}
	if n.Name != nil {
		ds = append(ds, b.node(n.Name), b.tok(b.after(n.Name.Last())), spaceDoc)
	}
	ds = append(ds, b.node(n.Cond))
	if n.At != nil {
		ds = append(ds, spaceDoc, b.tok(b.before(n.At.First())), spaceDoc, b.node(n.At))
	}
	msg := cat(b.tok(b.before(n.Message.First())), spaceDoc, b.node(n.Message))
	return cat(h, group(false, cat(ds...), indent(lineDoc, msg)))
}

func (b *builder) viewDecl(n *syntax.ViewDecl) *doc {
	h, kw := b.head(n, n.Annotations, nil)
	ds := []*doc{h, b.tok(kw), spaceDoc, b.node(n.Type)}
	if n.Case != nil {
		ds = append(ds, b.tok(b.before(n.Case.First())), b.node(n.Case))
	}
	return cat(append(ds, spaceDoc, b.braceList(n.Braces.Open, n.Braces.Close, entries(b, n.Items)))...)
}

func (b *builder) widgetDecl(n *syntax.WidgetDecl) *doc {
	h, kw := b.head(n, n.Annotations, nil)
	ds := []*doc{h, b.tok(kw), spaceDoc, b.node(n.Name), b.parenList(n.Parens.Open, n.Parens.Close, partsOf(b, n.Params))}
	if n.Default != syntax.NoTok {
		ds = append(ds, spaceDoc, b.tok(n.Default))
	}
	return cat(ds...)
}

func (b *builder) testDecl(n *syntax.TestDecl) *doc {
	h, kw := b.head(n, n.Annotations, nil)
	return cat(h, b.tok(kw), spaceDoc, b.node(n.Name), spaceDoc, b.node(n.Body))
}

func (b *builder) emitDecl(n *syntax.EmitDecl) *doc {
	h, kw := b.head(n, n.Annotations, nil)
	return cat(h, b.tok(kw), spaceDoc, b.node(n.Target), spaceDoc, b.node(n.Options))
}

// annotation is "@name(args)", never broken (§6.2).
func (b *builder) annotation(n *syntax.Annotation) *doc {
	ds := []*doc{b.tok(n.First()), b.node(n.Name)}
	if n.Parens.Open != syntax.NoTok {
		ds = append(ds, b.flatList(n.Parens.Open, n.Parens.Close, partsOf(b, n.Args)))
	}
	return flat(cat(ds...))
}

func (b *builder) annotationArg(n *syntax.AnnotationArg) *doc {
	if n.Name == nil {
		return b.node(n.Value)
	}
	return cat(b.node(n.Name), b.tok(b.after(n.Name.Last())), spaceDoc, b.node(n.Value))
}

func (b *builder) annotationList(n *syntax.AnnotationList) *doc {
	return b.flatList(n.First(), n.Last(), partsOf(b, n.Items))
}

// trailingAnnotations are the annotations after a name, on its line.
func (b *builder) trailingAnnotations(anns []*syntax.Annotation) *doc {
	var ds []*doc
	for _, a := range anns {
		ds = append(ds, spaceDoc, b.node(a))
	}
	return cat(ds...)
}
