package format

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

func (b *builder) namedType(n *syntax.NamedType) *doc {
	return cat(b.node(n.Name), b.typeArgs(n.Args))
}

// past is the `past` of a pastType and its space, nil when absent (GRAMMAR.md §5.9).
func (b *builder) past(t syntax.Tok) *doc {
	if t == syntax.NoTok {
		return nil
	}
	return cat(b.tok(t), spaceDoc)
}

// first is the first token of type n after its `past`.
func (b *builder) first(n syntax.Node) syntax.Tok {
	if p := syntax.PastOf(n); p != syntax.NoTok {
		return b.after(p)
	}
	return n.First()
}

func (b *builder) typeArgs(n *syntax.TypeArgs) *doc {
	if n == nil {
		return nil
	}
	return b.node(n)
}

func (b *builder) typeArgList(n *syntax.TypeArgs) *doc {
	return b.flatList(n.First(), n.Last(), partsOf(b, n.Args))
}

func (b *builder) listType(n *syntax.ListType) *doc {
	closeTok := b.after(n.Elem.Last())
	return cat(b.tok(b.first(n)), b.node(n.Elem), b.closing(closeTok, true), b.closer(closeTok), b.typeArgs(n.Args))
}

func (b *builder) keyedType(n *syntax.KeyedType) *doc {
	keyed := b.after(n.List.Last())
	return cat(b.node(n.List), spaceDoc, b.tok(keyed), spaceDoc, b.tok(b.after(keyed)), spaceDoc, b.node(n.Key))
}

func (b *builder) mapType(n *syntax.MapType) *doc {
	colon, closeTok := b.after(n.Key.Last()), b.after(n.Value.Last())
	return cat(b.tok(n.First()), b.node(n.Key), b.typed(colon, n.Value), b.closing(closeTok, true),
		b.closer(closeTok), b.typeArgs(n.Args))
}

func (b *builder) depMapType(n *syntax.DepMapType) *doc {
	in, colon, closeTok := b.after(n.Var.Last()), b.after(n.Domain.Last()), b.after(n.Value.Last())
	return cat(b.tok(n.First()), b.node(n.Var), spaceDoc, b.tok(in), spaceDoc, flat(b.node(n.Domain)),
		b.typed(colon, n.Value), b.closing(closeTok, true), b.closer(closeTok), b.typeArgs(n.Args))
}

func (b *builder) tableType(n *syntax.TableType) *doc {
	var ds []*doc
	if n.Stable != syntax.NoTok {
		ds = append(ds, b.tok(n.Stable), spaceDoc)
	}
	return cat(append(ds, b.tok(b.before(n.Name.First())), spaceDoc, b.node(n.Name))...)
}

func (b *builder) refType(n *syntax.RefType) *doc {
	return cat(b.tok(b.before(n.Name.First())), spaceDoc, b.node(n.Name))
}

// optionalType is "T?"; "T??" is two optional types over one token (E3401 is the checker's).
func (b *builder) optionalType(n *syntax.OptionalType) *doc {
	if inner, ok := n.Elem.(*syntax.OptionalType); ok && inner.Last() == n.Last() {
		return b.node(inner)
	}
	return cat(b.node(n.Elem), b.tok(n.Last()))
}

func (b *builder) whereType(n *syntax.WhereType) *doc {
	return cat(b.node(n.Base), spaceDoc, b.tok(b.before(n.Pred.First())), spaceDoc, flat(b.node(n.Pred)))
}

func (b *builder) unionType(n *syntax.UnionType) *doc {
	var ds []*doc
	for i, alt := range n.Alts {
		if i > 0 {
			ds = append(ds, spaceDoc, b.tok(b.before(alt.First())), spaceDoc)
		}
		ds = append(ds, b.node(alt))
	}
	return cat(ds...)
}

func (b *builder) literalType(n *syntax.LiteralType) *doc { return b.node(n.Value) }

// assetType is `asset("dir", ext: [names])`, never broken.
func (b *builder) assetType(n *syntax.AssetType) *doc {
	dir, trail := b.parts(n.Dir, false)
	ds := []*doc{dir}
	if n.Brackets.Open != syntax.NoTok {
		colon := b.before(n.Brackets.Open)
		ds = append(ds, separator(b.keeps(n.Dir.Last())), trail, spaceDoc, b.tok(b.before(colon)), b.tok(colon), spaceDoc,
			b.flatList(n.Brackets.Open, n.Brackets.Close, partsOf(b, n.Exts)))
	} else {
		ds = append(ds, trail)
	}
	ds = append(ds, b.closing(n.Parens.Close, true))
	return flat(cat(b.tok(b.first(n)), b.tok(n.Parens.Open), indent(ds...), b.closer(n.Parens.Close)))
}

func (b *builder) fnType(n *syntax.FnType) *doc {
	arrow := b.after(n.Parens.Close)
	return cat(b.tok(b.first(n)), b.flatList(n.Parens.Open, n.Parens.Close, partsOf(b, n.Params)), spaceDoc, b.tok(arrow),
		spaceDoc, b.node(n.Result))
}

func (b *builder) parenType(n *syntax.ParenType) *doc {
	return cat(b.tok(n.First()), b.node(n.Type), b.closing(n.Last(), true), b.closer(n.Last()))
}
