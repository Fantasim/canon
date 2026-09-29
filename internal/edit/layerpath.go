package edit

import (
	"path"

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

// inAmendment follows res's steps from step i inside an amendment: into a literal, a JSON load's file, else computed.
func (s *Snapshot) inAmendment(it item, res resolution, i int) Editability {
	for ; i < len(res.Steps); i++ {
		switch s.nodeForm(it.node) {
		case shape.FormLiteral:
		case shape.FormJSON:
			return jsonEditability(loadDisplay(it.file, it.node))
		default:
			return Editability{Reason: ReasonComputed}
		}
		n, ok := childNode(it.node, res.Steps[i])
		if !ok {
			return missingChild(it, res, i)
		}
		it.node = n
	}
	return Editability{Mode: ModeCanon, File: it.file.Src.Path, Span: it.file.Span(it.node)}
}

// jsonEditability is an edit of the JSON source at display, whose name must be `.json` (API.md W4).
func jsonEditability(display string) Editability {
	if display != "" && !jsonFileName(display) {
		return Editability{Reason: ReasonFormat}
	}
	return Editability{Mode: ModeJSON, File: display}
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

// loadDisplay is the display path of the file a plain load reads (WIRE.md §2.3), "" for load.dir.
func loadDisplay(f *syntax.File, n syntax.Node) string {
	x, ok := n.(*syntax.LoadExpr)
	if !ok || x.Method != nil || len(x.Args) == 0 || x.Args[0].Name != nil {
		return ""
	}
	lit, ok := x.Args[0].Value.(*syntax.StringLit)
	if !ok || len(lit.Parts) != 1 {
		return ""
	}
	p := lit.Parts[0].Text
	if p != "" && p[0] == rootMark {
		return path.Clean(p)
	}
	return path.Join(path.Dir(f.Src.Path), p)
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

// keyMatches reports an amend key naming step st of parent: a negative list index counts from
// the end, a qualified member names its member.
func (s *Snapshot) keyMatches(e syntax.Expr, parent value.Value, st Step) bool {
	switch x := e.(type) {
	case *syntax.IntLit:
		if x.Value.Sign() < 0 && x.Value.IsInt64() {
			l, ok := parent.(*value.List)
			return ok && int64(position(parent, st.Value)) == int64(len(l.Elems))+x.Value.Int64()
		}
	case *syntax.SelectorExpr:
		if s.info.Selections[x] != nil {
			return false
		}
	}
	return keyExprIs(e, st.Seg)
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
