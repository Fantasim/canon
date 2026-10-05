package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// rootCursor is the source of a let's or const's value (W1): its literal, the JSON its load
// reads, a load of another format, or an expression.
func (s *Snapshot) rootCursor(res resolution) cursor {
	obj := res.root.obj
	e := syntax.Unparen(initializer(obj.Decl()))
	entries := s.entryDecls(res.root)
	files := len(entries) > 0 || filesLet(obj.Decl())
	switch shape.SourceForm(s.info, e) {
	case shape.FormLiteral:
		f := obj.File()
		c := cursor{state: stTree, mode: ModeCanon, node: e, file: f, span: f.Span(e), files: files, entries: entries}
		c.newFile = filesLet(obj.Decl()) || slices.ContainsFunc(entries, func(it item) bool { return it.file != f })
		return c
	case shape.FormJSON:
		c := loadCursor(s.info, e, res.val)
		c.entries, c.files = entries, c.files || files
		c.newFile = c.files
		return c
	case shape.FormFormat:
		return cursor{state: stFormat}
	default:
		return cursor{state: stComputed}
	}
}

// loadCursor is the source of the value a load item reads: its JSON, or the format row.
func loadCursor(info *check.Info, e syntax.Expr, v value.Value) cursor {
	if shape.SourceForm(info, e) == shape.FormFormat {
		return cursor{state: stFormat}
	}
	load, _ := e.(*syntax.LoadExpr)
	c := cursor{state: stTree, mode: ModeJSON, files: isLoadDir(info, e), load: load}
	if p := provOf(v); p != nil && p.Kind == value.ProvJSON {
		c.span = p.Span
	}
	return c
}

// treeStep moves from a source-tree node to the child step i reads (W3).
func treeStep(j *judge, c cursor, i int) cursor {
	if c.mode == ModeJSON {
		return j.jsonStep(c, j.res.Steps[i].Value)
	}
	return j.canonStep(c, i)
}

// jsonStep is a child in a JSON source: a JSON value, or a field its object omits (W3, W7).
func (j *judge) jsonStep(c cursor, v value.Value) cursor {
	p := provOf(v)
	if p == nil {
		return c.to(stComputed)
	}
	switch p.Kind {
	case value.ProvJSON:
		return cursor{state: stTree, mode: ModeJSON, span: p.Span, layer: c.layer, load: c.load}
	case value.ProvDefault:
		return c.to(stAbsent)
	case value.ProvLiteral:
		if it, ok := matchItem(c, v); ok {
			return j.itemCursor(c, it, v) // an `entry` declared beside the loaded collection (W2)
		}
	case value.ProvCSV, value.ProvDefines, value.ProvText:
		return c.to(stFormat)
	default:
	}
	return c.to(stComputed)
}

// canonStep is a child in a .canon literal: a written field, a field the literal omits
// (default or spread, W3), or the item stating an element, entry or map value.
func (j *judge) canonStep(c cursor, i int) cursor {
	st, parent := j.res.Steps[i], j.res.parent(i)
	if _, isRec := parent.(*value.Record); isRec && st.Seg.Kind == SegField {
		fi := fieldItem(c.node, st.Seg.Name)
		switch {
		case fi != nil:
			return j.itemCursor(c, item{syntax.Unparen(fi.Value), c.file}, st.Value)
		case hasSpread(c.node):
			return c.to(stSpread)
		}
		return c.to(stAbsent)
	}
	if it, ok := matchItem(c, st.Value); ok {
		return j.itemCursor(c, it, st.Value)
	}
	if it, ok := itemAt(c, parent, st.Value); ok {
		if _, isLoad := it.node.(*syntax.LoadExpr); isLoad {
			return j.itemCursor(c, it, st.Value)
		}
	}
	return c.to(stComputed)
}

// itemCursor is the source of v stated by item it of c's literal: a nested load, or a literal
// whose provenance is that very node; anything else leaves the tree.
func (j *judge) itemCursor(c cursor, it item, v value.Value) cursor {
	if e, ok := it.node.(syntax.Expr); ok && e.Kind() == syntax.KindLoadExpr {
		next := loadCursor(j.s.info, e, v)
		next.layer = c.layer
		return next
	}
	p := provOf(v)
	if p == nil || p.Kind != value.ProvLiteral || p.Span != it.file.Span(it.node) {
		return c.to(stComputed)
	}
	return cursor{state: stTree, mode: ModeCanon, node: it.node, file: it.file, span: p.Span, layer: c.layer}
}

// matchItem is the item of the current literal (or root entry) whose node v's provenance names.
func matchItem(c cursor, v value.Value) (item, bool) {
	p := provOf(v)
	if p == nil || p.Kind != value.ProvLiteral || c.file == nil && len(c.entries) == 0 {
		return item{}, false
	}
	for _, it := range append(items(c.node, c.file), c.entries...) {
		if it.file.Span(it.node) == p.Span {
			return it, true
		}
	}
	return item{}, false
}

// itemAt is the item at v's position in its list or map literal.
func itemAt(c cursor, parent, v value.Value) (item, bool) {
	all := items(c.node, c.file)
	i := position(parent, v)
	if i < 0 || i >= len(all) {
		return item{}, false
	}
	return all[i], true
}

// absentStep is a field of a record that exists only through its default: W8 materializes it.
func absentStep(j *judge, c cursor, i int) cursor {
	if _, isRec := j.res.parent(i).(*value.Record); isRec && j.res.Steps[i].Seg.Kind == SegField {
		return c
	}
	return c.to(stComputed)
}

// leave is a step below a spread-supplied field or an amendment's expression: the path goes
// through that expression.
func leave(_ *judge, c cursor, _ int) cursor { return c.to(stComputed) }

// stay keeps a walk that already left the tree, or reached a format source or a layered value.
func stay(_ *judge, c cursor, _ int) cursor { return c }
