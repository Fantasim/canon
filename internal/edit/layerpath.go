package edit

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// staticAmendment is where layer x's amendment of res's path or an ancestor, active or not, puts an edit (W11a).
func (s *Snapshot) staticAmendment(res resolution, x string) (Editability, bool) {
	for _, f := range layerFiles(res.root.pkg, x) {
		for _, b := range f.Amends {
			if e, ok := s.amends(f, b, res); ok {
				return e, true
			}
		}
	}
	return Editability{}, false
}

// amends is the editability block b gives res's path, when one of its paths is a prefix of it.
func (s *Snapshot) amends(f *syntax.File, b *syntax.AmendBlock, res resolution) (Editability, bool) {
	if b.Target == nil || b.Target.Name != res.root.obj.Name() {
		return Editability{}, false
	}
	for _, a := range b.Items {
		if n, ok := s.prefixOf(a.Path, res); ok {
			return s.inAmendment(item{syntax.Unparen(a.Value), f}, res, n), true
		}
	}
	return Editability{}, false
}

// inAmendment follows res's steps from step i inside an amendment (API.md W11a): into a literal, else leaves.
func (s *Snapshot) inAmendment(it item, res resolution, i int) Editability {
	start := i
	for ; i < len(res.Steps); i++ {
		if e, stop := s.leaves(it.node); stop {
			return e
		}
		n, ok := childNode(it.node, res.Steps[i])
		if !ok {
			return missingChild(it, res, i)
		}
		it.node = n
	}
	if e, stop := s.leaves(it.node); stop && i > start {
		return e // the target is a load or an expression nested in the literal
	}
	return Editability{Mode: ModeCanon, File: it.file.Src.Path, Span: it.file.Span(it.node)}
}

// leaves is the verdict on node n of an amendment when it is no literal: a load of any format
// `layer`, its data outside the layer file (DECISIONS 462); any other expression `computed`.
func (s *Snapshot) leaves(n syntax.Node) (Editability, bool) {
	switch {
	case s.nodeForm(n) == shape.FormLiteral:
		return Editability{}, false
	case s.isLoad(n):
		return Editability{Reason: ReasonLayer}, true
	}
	return Editability{Reason: ReasonComputed}, true
}

// missingChild is step i, absent from the amendment literal it: a record field is inserted into
// it (W7, W8), or overridden beside the spread supplying it; anything below a spread-supplied
// field is computed; an element, entry or key the RHS lacks has no amendment form.
func missingChild(it item, res resolution, i int) Editability {
	if _, isRec := res.parent(i).(*value.Record); !isRec || res.Steps[i].Seg.Kind != SegField {
		return Editability{Reason: ReasonLayer}
	}
	if hasSpread(it.node) && i < len(res.Steps)-1 {
		return Editability{Reason: ReasonComputed}
	}
	for j := i + 1; j < len(res.Steps); j++ {
		if _, isRec := res.parent(j).(*value.Record); !isRec || res.Steps[j].Seg.Kind != SegField {
			return Editability{Reason: ReasonComputed} // W8 materializes records only
		}
	}
	return Editability{Mode: ModeCanon, File: it.file.Src.Path, Span: it.file.Span(it.node)}
}

// nodeForm is sourceForm of a node, an entry being a literal.
func (s *Snapshot) nodeForm(n syntax.Node) shape.Form {
	if _, ok := n.(*syntax.EntryItem); ok {
		return shape.FormLiteral
	}
	e, _ := n.(syntax.Expr)
	return shape.SourceForm(s.info, e)
}

// prefixOf reports an amend path naming the first steps of res's path, and their number.
func (s *Snapshot) prefixOf(segs []*syntax.AmendSegment, res resolution) (int, bool) {
	if len(segs) > len(res.Steps) {
		return 0, false
	}
	for i, seg := range segs {
		if !s.segMatches(seg, res, i) {
			return 0, false
		}
	}
	return len(segs), true
}

// namesBelow reports an amend path longer than res's whose first segments name res's steps.
func (s *Snapshot) namesBelow(segs []*syntax.AmendSegment, res resolution) bool {
	if len(segs) <= len(res.Steps) {
		return false
	}
	for i := range res.Steps {
		if !s.segMatches(segs[i], res, i) {
			return false
		}
	}
	return true
}

// lineBelow reports a line of layer x below res's path for which bad holds.
func (s *Snapshot) lineBelow(res resolution, x string, bad func(*syntax.File, *syntax.Amendment) bool) bool {
	for _, f := range layerFiles(res.root.pkg, x) {
		if slices.ContainsFunc(f.Amends, func(b *syntax.AmendBlock) bool { return s.blockBelow(f, b, res, bad) }) {
			return true
		}
	}
	return false
}

// blockBelow reports block b of f amending res's root with a line below its path for which bad holds.
func (s *Snapshot) blockBelow(f *syntax.File, b *syntax.AmendBlock, res resolution, bad func(*syntax.File, *syntax.Amendment) bool) bool {
	if b.Target == nil || b.Target.Name != res.root.obj.Name() {
		return false
	}
	return slices.ContainsFunc(b.Items, func(it *syntax.Amendment) bool { return s.namesBelow(it.Path, res) && bad(f, it) })
}

// computedBelow reports a line of layer x below res's path that no Source states: a Set
// replacing it would have to write it back (DECISIONS 273).
func (s *Snapshot) computedBelow(ctx context.Context, res resolution, x string) bool {
	r := res.root.at()
	return s.lineBelow(res, x, func(f *syntax.File, it *syntax.Amendment) bool { return !s.lineSource(ctx, f, r, it) })
}

// unseenBelow reports a line of layer x below res's path naming an entry s does not show: a Set
// replacing it would need an AddEntry no path of s reaches to undo it (API.md W11a).
func (s *Snapshot) unseenBelow(res resolution, x string) bool {
	r := res.root.at()
	return s.lineBelow(res, x, func(f *syntax.File, it *syntax.Amendment) bool {
		p, err := Parse(s.linePathOf(f, r, it.Path))
		if err != nil {
			return true
		}
		_, err = s.open(p)
		return err != nil
	})
}

// segMatches reports an amend segment naming step i: a word, a key, or a position (EVALUATION.md §9.2).
func (s *Snapshot) segMatches(a *syntax.AmendSegment, res resolution, i int) bool {
	st, parent := res.Steps[i], res.parent(i)
	switch {
	case a.Name != nil:
		return nameIs(st.Seg, a.Name.Name)
	case a.Position != nil:
		return a.Position.Value.IsInt64() && int64(position(parent, st.Value)) == a.Position.Value.Int64()
	case a.Key != nil:
		return s.keyMatches(syntax.Unparen(a.Key), parent, st)
	}
	return false
}

// keyMatches reports an amend key naming step st of parent: a negative index of a plain list
// counts from the end (fromEndIndex); a qualified member names its member.
func (s *Snapshot) keyMatches(e syntax.Expr, parent value.Value, st Step) bool {
	switch x := e.(type) {
	case *syntax.IntLit:
		if i, ok := fromEndIndex(parent, x); ok {
			return int64(position(parent, st.Value)) == i
		}
	case *syntax.SelectorExpr:
		if s.info.Selections[x] != nil {
			return false
		}
	}
	return keyExprIs(e, st.Seg)
}

// fromEndIndex is the index a negative `[k]` names on c, counted from the end, when c is a plain
// list; on a keyed list `[k]` is a key, never a position (EVALUATION.md 9.2).
func fromEndIndex(c value.Value, k *syntax.IntLit) (int64, bool) {
	l, ok := c.(*value.List)
	if !ok || keyedContainer(l.T) || k.Value.Sign() >= 0 || !k.Value.IsInt64() {
		return 0, false
	}
	return int64(len(l.Elems)) + k.Value.Int64(), true
}

// nameIs reports a canonical segment naming name: a field, an entry, or a key written as a word.
func nameIs(s Seg, name string) bool {
	return s.Kind == SegField && s.Name == name || s.Kind == SegKey && s.Key.Kind != KeyInt && s.Key.Text == name
}

// keyExprIs reports a key written in source naming the canonical segment s.
func keyExprIs(e syntax.Expr, s Seg) bool {
	switch x := e.(type) {
	case *syntax.IntLit:
		return s.Kind == SegKey && s.Key.Kind == KeyInt && x.Value.IsInt64() && s.Key.Int == x.Value.Int64()
	case *syntax.IdentExpr:
		return nameIs(s, x.Name)
	case *syntax.SelectorExpr:
		return nameIs(s, x.Name.Name)
	case *syntax.StringLit:
		return len(x.Parts) == 1 && x.Parts[0].Interp == nil && nameIs(s, x.Parts[0].Text)
	}
	return false
}
