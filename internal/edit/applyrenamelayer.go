package edit

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// layerKeys rewrites, in every layer file of the root's package, active or not, the key segment of
// each amend line reaching the renamed item and the key field of a line stating it whole (API.md
// E11; DECISIONS 310; EVALUATION.md 9.2); a token a reference already rewrote is kept.
func (r *renaming) layerKeys() error {
	last := len(r.x.res.Steps) - 1
	if last < 0 {
		return nil
	}
	for _, f := range r.x.res.root.pkg.Files {
		if f.FileKind != syntax.FileLayer {
			continue
		}
		for _, b := range f.Amends {
			if err := r.blockKeys(f, b, last); err != nil {
				return err
			}
		}
	}
	return nil
}

// blockKeys is layerKeys on block b of f, when b amends the renamed item's root: each line whose
// first last+1 segments name the item's path.
func (r *renaming) blockKeys(f *syntax.File, b *syntax.AmendBlock, last int) error {
	if b.Target == nil || b.Target.Name != r.x.res.root.obj.Name() {
		return nil
	}
	for _, it := range b.Items {
		if err := r.lineKeys(f, it, last); err != nil {
			return err
		}
	}
	return nil
}

// lineKeys is blockKeys on line it: above the item, the key field its literal states; reaching
// it, the key segment, and the key field of a line stating it whole.
func (r *renaming) lineKeys(f *syntax.File, it *syntax.Amendment, last int) error {
	if len(it.Path) <= last {
		return r.aboveKeyField(f, it)
	}
	if _, ok := r.x.a.snap.prefixOf(it.Path[:last+1], r.x.res); !ok || r.x.a.snap.scopedSegs(it.Path[:last+1]) {
		return nil // a key read from a name in scope names no key by its text (DECISIONS 318)
	}
	if err := r.segKey(f, it, last); err != nil {
		return err
	}
	if len(it.Path) == last+1 && it.Value != nil {
		return r.lineKeyField(f, syntax.Unparen(it.Value))
	}
	return nil
}

// aboveKeyField rewrites the renamed element's key field where line it states one of its
// ancestors whole and its literal holds the element's: the layer's own instance stays one (E11).
func (r *renaming) aboveKeyField(f *syntax.File, it *syntax.Amendment) error {
	k, ok := r.x.a.snap.prefixOf(it.Path, r.x.res)
	if !ok || it.Value == nil || r.x.a.snap.scopedSegs(it.Path) {
		return nil
	}
	node := syntax.Node(syntax.Unparen(it.Value))
	for _, st := range r.x.res.Steps[k:] {
		if node, ok = childNode(node, st); !ok {
			return nil
		}
	}
	return r.lineKeyField(f, node)
}

// segKey rewrites segment i of line it, which names the old key: a line's first, a word that does
// not continue the line above (else `layer`, naming the line); a later `.k` bare, else `["…"]`
// (API.md 6.4); `[k]` in its form (a string stays a string, E11); `[#n]` names no key.
func (r *renaming) segKey(f *syntax.File, it *syntax.Amendment, i int) error {
	seg, text := it.Path[i], r.to.CanonText()
	switch {
	case i == 0 && !lineWord(text):
		return r.lineRefusal(f, it)
	case seg.Name != nil && (i == 0 || isWord(text)):
		return r.replace(f, seg.Name, text, false)
	case seg.Name != nil:
		return r.replace(f, seg, string(bracketOpen)+quoted(r.to)+bracketClose, false)
	case seg.Key != nil:
		var n syntax.Node = syntax.Unparen(seg.Key)
		if sel, ok := n.(*syntax.SelectorExpr); ok {
			n = sel.Name // `[Q.m]` keeps its qualifier (E11)
		}
		text, err := r.tokenText(n)
		if err != nil {
			return err
		}
		return r.replace(f, n, text, false)
	}
	return nil
}

// lineKeyField rewrites the key field of n, a line's literal stating a keyed-list element whole,
// as the rename rewrites the element's own (E11).
func (r *renaming) lineKeyField(f *syntax.File, n syntax.Node) error {
	lt, ok := baseOf(r.x.res.Steps[len(r.x.res.Steps)-1].Container).(*types.ListType)
	if !ok || lt.KeyedBy == nil {
		return nil
	}
	fi := fieldItem(n, lt.KeyedBy.Name)
	if fi == nil {
		return nil
	}
	text, err := r.x.a.canonText(r.to)
	if err != nil {
		return err
	}
	return r.replace(f, syntax.Unparen(fi.Value), text, false)
}

// lineRefusal is `layer` naming line it of f, which no amend line can start with the new key: the
// way out, to remove that line first, only when no other line starts with the key (DECISIONS 305).
func (r *renaming) lineRefusal(f *syntax.File, it *syntax.Amendment) error {
	from, to := f.Span(it.Path[0]), f.Span(it.Path[len(it.Path)-1])
	line := r.x.res.root.pkg.Path + packageMark + r.x.res.root.obj.Name() + string(fieldMark) + string(f.Src.Content[from.Start:to.End])
	e := &NotEditableError{Reason: ReasonLayer, File: f.Src.Path, Refs: []string{line}}
	if r.keyLines() == 1 {
		e.Detail = wayLine
	}
	if f.Layer != nil {
		e.Layer = f.Layer.Name
	}
	return e
}

// keyLines counts the lines of the package's layer files, active or not, starting with the
// renamed item's key: each refuses the rename as lineRefusal's does.
func (r *renaming) keyLines() int {
	n := 0
	for _, f := range r.x.res.root.pkg.Files {
		if f.FileKind != syntax.FileLayer {
			continue
		}
		for _, b := range f.Amends {
			n += r.blockKeyLines(b)
		}
	}
	return n
}

// blockKeyLines is keyLines in block b, when b amends the renamed item's root.
func (r *renaming) blockKeyLines(b *syntax.AmendBlock) int {
	if b.Target == nil || b.Target.Name != r.x.res.root.obj.Name() {
		return 0
	}
	n := 0
	for _, it := range b.Items {
		if _, ok := r.x.a.snap.prefixOf(it.Path[:1], r.x.res); ok {
			n++
		}
	}
	return n
}

// placeOf is node n of f as a token place.
func placeOf(f *syntax.File, n syntax.Node) tokenPlace {
	sp := f.Span(n)
	return tokenPlace{file: f.Src.Path, start: sp.Start, end: sp.End}
}
