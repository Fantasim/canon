package edit

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// layerStep is a value an active layer set (W10): layered, or with that layer as the edit layer
// editable inside its amendment (W11a).
func (j *judge) layerStep(i int) cursor {
	p := j.res.Steps[i].Value.Prov()
	if p.Layer != j.layer {
		return cursor{state: stLayered, layer: p.Layer}
	}
	it, ok := j.s.amendment(j.res.root, p)
	if !ok {
		return cursor{state: stComputed}
	}
	return j.rhsCursor(it, p.Layer)
}

// rhsCursor is an amendment's right-hand side: a literal is a source tree, a load of JSON reads
// its file, any other expression is replaced whole and leaves the tree below it.
func (j *judge) rhsCursor(it item, layer string) cursor {
	c := cursor{state: stTree, mode: ModeCanon, node: it.node, file: it.file, span: it.file.Span(it.node), layer: layer}
	e, _ := it.node.(syntax.Expr)
	switch shape.SourceForm(j.s.info, e) {
	case shape.FormLiteral:
	case shape.FormJSON:
		c.mode = ModeJSON
	default:
		c.state = stOpaque
	}
	return c
}

// amendment is the right-hand side of the amendment p records, in the root's package.
func (s *Snapshot) amendment(r rootRef, p *value.Prov) (item, bool) {
	for _, f := range layerFiles(r.pkg, p.Layer) {
		for _, b := range f.Amends {
			if it, ok := amendedAt(f, b, p); ok {
				return it, true
			}
		}
	}
	return item{}, false
}

// amendedAt is the right-hand side of block b whose span p records.
func amendedAt(f *syntax.File, b *syntax.AmendBlock, p *value.Prov) (item, bool) {
	for _, a := range b.Items {
		if f.Span(a.Value) == p.Span {
			return item{unparen(a.Value), f}, true
		}
	}
	return item{}, false
}

// layerFiles are the package's files of layer x.
func layerFiles(pkg *check.Package, x string) []*syntax.File {
	var out []*syntax.File
	for _, f := range pkg.Files {
		if f.FileKind == syntax.FileLayer && f.Layer != nil && f.Layer.Name == x {
			out = append(out, f)
		}
	}
	return out
}
