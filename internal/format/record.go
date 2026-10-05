package format

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

func (b *builder) recordDecl(n *syntax.RecordDecl) *doc {
	h, kw := b.head(n, nil, n.Mods)
	ds := []*doc{h, b.tok(kw), spaceDoc, b.node(n.Name)}
	if n.Parens.Open != syntax.NoTok {
		ds = append(ds, b.parenList(n.Parens.Open, n.Parens.Close, partsOf(b, n.Params)))
	}
	return cat(append(ds, b.trailingAnnotations(n.Annotations), spaceDoc, b.node(n.Body))...)
}

// recordBody is the item list of a record or case. A field followed by a method or a check
// keeps its annotations on its line.
func (b *builder) recordBody(n *syntax.RecordBody) *doc {
	for i, it := range n.Items {
		if f, ok := it.(*syntax.FieldDecl); ok && i+1 < len(n.Items) && isMember(n.Items[i+1]) {
			b.glued[f] = true
		}
	}
	return b.braceList(n.First(), n.Last(), entries(b, n.Items))
}

func isMember(it syntax.RecordItem) bool {
	switch it.(type) {
	case *syntax.FnDecl, *syntax.CheckDecl:
		return true
	default:
		return false
	}
}

// fieldDecl is group(name ": " type [rhs("=", default)], indent(line, annotations…)): the
// annotations break onto continuation lines when the field does not fit.
func (b *builder) fieldDecl(n *syntax.FieldDecl) *doc {
	colon := b.after(n.Name.Last())
	ds := []*doc{b.node(n.Name), b.tok(colon), spaceDoc}
	if n.Input != syntax.NoTok {
		ds = append(ds, b.tok(n.Input), spaceDoc)
	}
	ds = append(ds, b.node(n.Type))
	if n.Env != nil {
		env := b.before(n.Env.First())
		ds = append(ds, spaceDoc, b.tok(b.before(env)), spaceDoc, b.tok(env), spaceDoc, b.node(n.Env))
	}
	if n.Default != nil && !redundantNone(n) {
		ds = append(ds, b.assign(b.before(n.Default.First()), n.Default))
	}
	sep := lineDoc
	if b.glued[n] {
		sep = spaceDoc
	}
	var anns []*doc
	for _, a := range n.Annotations {
		anns = append(anns, sep, b.node(a))
	}
	return group(false, cat(ds...), indent(anns...))
}

func (b *builder) enumDecl(n *syntax.EnumDecl) *doc {
	h, kw := b.head(n, nil, n.Mods)
	ds := []*doc{h, b.tok(kw), spaceDoc, b.node(n.Name)}
	if n.Ordered != syntax.NoTok {
		ds = append(ds, spaceDoc, b.tok(n.Ordered))
	}
	list := b.braceList(n.Braces.Open, n.Braces.Close, entries(b, n.Members))
	return cat(append(ds, b.trailingAnnotations(n.Annotations), spaceDoc, list)...)
}

func (b *builder) enumMember(n *syntax.EnumMember) *doc {
	h, _ := b.head(n, nil, n.Mods)
	ds := []*doc{h, b.node(n.Name)}
	if n.Value != nil {
		ds = append(ds, spaceDoc, b.tok(b.after(n.Name.Last())), spaceDoc, b.node(n.Value))
	}
	return cat(append(ds, b.trailingAnnotations(n.Annotations))...)
}

func (b *builder) variantDecl(n *syntax.VariantDecl) *doc {
	h, kw := b.head(n, nil, n.Mods)
	list := b.braceList(n.Braces.Open, n.Braces.Close, entries(b, n.Items))
	return cat(h, b.tok(kw), spaceDoc, b.node(n.Name), b.trailingAnnotations(n.Annotations), spaceDoc, list)
}

func (b *builder) variantCase(n *syntax.VariantCase) *doc {
	h, _ := b.head(n, nil, n.Mods)
	ds := []*doc{h, b.node(n.Name), b.trailingAnnotations(n.Annotations)}
	if n.Body != nil {
		ds = append(ds, spaceDoc, b.node(n.Body))
	}
	return cat(ds...)
}
