package edit

import (
	"strings"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// staticSite is an amendment line of the edit layer, in its file, naming the first n steps of
// the path: read from its text, whether the layer is active or not (W11a).
type staticSite struct {
	f    *syntax.File
	line *syntax.Amendment
	n    int
}

// layerLine is layer x's amendment line naming res's path or an ancestor, its file and how
// many steps it names; nil when there is none.
func (s *Snapshot) layerLine(res resolution, x string) (*syntax.File, *syntax.Amendment, int) {
	for _, f := range layerFiles(res.root.pkg, x) {
		for _, b := range f.Amends {
			if b.Target == nil || b.Target.Name != res.root.obj.Name() {
				continue
			}
			if it, n := s.prefixLine(b, res); it != nil {
				return f, it, n
			}
		}
	}
	return nil, nil, 0
}

// prefixLine is block b's line whose path is a prefix of res's, and its length.
func (s *Snapshot) prefixLine(b *syntax.AmendBlock, res resolution) (*syntax.Amendment, int) {
	for _, it := range b.Items {
		if n, ok := s.prefixOf(it.Path, res); ok {
			return it, n
		}
	}
	return nil, 0
}

// lineValue is the value the line's literal states, typed at its path, and the steps to the
// value `upto` steps down the path, read in that literal; the last may be a field it leaves out.
func (x *opCtx) lineValue(at staticSite, upto int) (value.Value, []Step, error) {
	rhs := syntax.Unparen(at.line.Value)
	if x.a.snap.nodeForm(rhs) != shape.FormLiteral {
		return nil, nil, &NotEditableError{Reason: ReasonComputed}
	}
	sp := at.f.Span(rhs)
	old, err := x.typed(Source(at.f.Src.Content[sp.Start:sp.End]), x.typeAt(at.n))
	if err != nil {
		return nil, nil, &NotEditableError{Reason: ReasonComputed}
	}
	w := walk{cur: old, typ: x.typeAt(at.n)}
	for _, st := range x.res.Steps[at.n:upto] {
		if err := w.next(st.Seg); err != nil {
			return nil, nil, &NotEditableError{Reason: ReasonLayer}
		}
	}
	return old, w.steps, nil
}

// staticEdit applies mutate to the value `upto` steps down the path inside the edit layer's
// line (W11a): its literal is compared with the new value (M1); a Set of a field to its default
// leaves it out of that literal (E6); the inverse sets the value the literal held (E23).
func (x *opCtx) staticEdit(at staticSite, upto int, mutate func(value.Value) (value.Value, error)) error {
	old, steps, err := x.lineValue(at, upto)
	if err != nil {
		return err
	}
	cur := old
	if len(steps) > 0 {
		cur = steps[len(steps)-1].Value
	}
	if err := x.undoStatic(cur, upto); err != nil {
		return err
	}
	nv, err := mutate(cur)
	if err != nil {
		return err
	}
	if rec, i, ok := fieldOf(old, steps); ok && nv != nil && x.a.isDefault(withField(rec, i, nv), i) {
		nv = nil
	}
	d := &canonDiff{a: x.a}
	if err := d.value(old, withChange(old, steps, nv), syntax.Unparen(at.line.Value)); err != nil {
		return err
	}
	x.w.addCanon(at.f.Src.Path, x.res.root.pkg.Path, d.out)
	return nil
}

// undoStatic is the inverse of an edit inside a line (E23): Set of the value the line held at
// the edited path, or at the collection for a Remove or an AddEntry, whose keys only the layer
// may hold; Reset when the line left the field out.
func (x *opCtx) undoStatic(cur value.Value, upto int) error {
	path, k := x.res.Canonical, len(x.res.Steps)
	if upto < len(x.res.Steps) {
		path, k = x.parentPath(), k-1
	}
	if cur == nil {
		x.inverse(Operation{Kind: OpReset, Path: path})
		return nil
	}
	lit, err := x.a.sourceLit(cur, x.scopeAt(k, true))
	if err != nil {
		return err
	}
	x.inverse(Operation{Kind: OpSet, Path: path, Value: lit})
	return nil
}

// fieldOf is the record the last step reads a field of, and the field's index.
func fieldOf(root value.Value, steps []Step) (*value.Record, int, bool) {
	if len(steps) == 0 || steps[len(steps)-1].Seg.Kind != SegField {
		return nil, 0, false
	}
	parent := root
	if len(steps) > 1 {
		parent = steps[len(steps)-parentStepBack].Value
	}
	rec, ok := parent.(*value.Record)
	if !ok {
		return nil, 0, false
	}
	i := fieldIndex(fieldsOf(rec.T), steps[len(steps)-1].Seg.Name)
	return rec, i, i >= 0
}

// withoutSeg is table or map cur without the item seg names.
func (x *opCtx) withoutSeg(cur value.Value, st Step) (value.Value, error) {
	w := walk{cur: cur, typ: st.Container}
	if err := w.next(st.Seg); err != nil {
		return nil, &NotEditableError{Reason: ReasonLayer}
	}
	pos := position(cur, w.cur)
	if pos < 0 {
		return nil, &NotEditableError{Reason: ReasonLayer}
	}
	switch cur.(type) {
	case *value.Table, *value.Map:
		return without(cur, pos), nil
	}
	return nil, &NotEditableError{Reason: ReasonLayer}
}

// typeAt is the declared type of the value the path's first n steps reach.
func (x *opCtx) typeAt(n int) types.Type {
	if n == 0 {
		return x.res.root.obj.Type()
	}
	st := x.res.Steps[n-1]
	if rec, ok := x.res.parent(n - 1).(*value.Record); ok {
		fields := fieldsOf(rec.T)
		if i := fieldIndex(fields, st.Seg.Name); i >= 0 {
			return fields[i].Type
		}
	}
	if _, vt, ok := mapTypes(st.Container); ok {
		return vt
	}
	return elemType(st.Container)
}

// amendLine writes `path: text` in the edit layer file of the root's package: into its block
// for the root, replacing the lines of the path's descendants, or in a new block, in a new file
// when the package has none (W11, W11a).
func (x *opCtx) amendLine(canonical, text string) error {
	rel, ok := amendPath(canonical)
	if !ok {
		return &NotEditableError{Reason: ReasonLayer}
	}
	item := rel + colonSp + text
	root := x.res.root.obj.Name()
	files := layerFiles(x.res.root.pkg, x.a.env.EditLayer)
	if len(files) == 0 {
		return x.newLayerFile(root, item)
	}
	f := files[0]
	for _, b := range f.Amends {
		if b.Target != nil && b.Target.Name == root {
			var changes []format.Change
			if canonical == x.res.Canonical {
				changes = x.descendantLines(b) // an entry's new line has no descendant (log-2026-10-01 M4.1 ruling d)
			}
			changes = append(changes, format.Change{Kind: format.Insert, List: b.Braces.Open, At: len(b.Items), Text: item})
			x.w.addCanon(f.Src.Path, x.res.root.pkg.Path, changes)
			return nil
		}
	}
	block := amendWord + space + root + space + braceOpenSp + item + braceCloseSp
	top := len(f.Decls) + len(f.Amends) + len(f.Entries)
	x.w.addCanon(f.Src.Path, x.res.root.pkg.Path, []format.Change{{Kind: format.Insert, List: syntax.NoTok, At: top, Text: block}})
	return nil
}

// descendantLines removes the lines of block b that amend a path below the edited one (W11a).
func (x *opCtx) descendantLines(b *syntax.AmendBlock) []format.Change {
	var out []format.Change
	for _, it := range b.Items {
		if len(it.Path) > len(x.res.Steps) && x.namesPrefix(it.Path) {
			out = append(out, format.Change{Kind: format.Remove, Node: it})
		}
	}
	return out
}

// namesPrefix reports an amend path whose first segments name the path's steps.
func (x *opCtx) namesPrefix(segs []*syntax.AmendSegment) bool {
	if len(segs) < len(x.res.Steps) {
		return false
	}
	for i := range x.res.Steps {
		if !x.a.snap.segMatches(segs[i], x.res, i) {
			return false
		}
	}
	return true
}

// newLayerFile creates <package dir>/<layer>.layer.canon holding the one amendment (W11).
func (x *opCtx) newLayerFile(root, item string) error {
	display := layerFile(x.res.root.pkg, x.a.env.EditLayer)
	if err := x.a.free(display); err != nil {
		return err
	}
	src := packageWord + space + x.res.root.pkg.Path + newline + layerWord + space + x.a.env.EditLayer + blankLine +
		amendWord + space + root + space + braceOpenSp + item + braceCloseSp + newline
	content, err := freshFile(x.res.root.pkg.Path, src)
	if err != nil {
		return err
	}
	x.w.creates = append(x.w.creates, newFile{display: display, content: content})
	x.w.own(display, x.res.root.pkg.Path)
	return nil
}

// amendPath is a canonical path as an amend path, which starts with a name (API.md §6.4).
func amendPath(canonical string) (string, bool) {
	p, err := Parse(canonical)
	if err != nil || len(p.Segs) == 0 || p.Segs[0].Kind != SegField {
		return "", false
	}
	rest := Path{Segs: p.Segs}.String()
	return strings.TrimPrefix(rest, string(fieldMark)), true
}

// lineRemoval removes amendment line from f, and its block with it when it is the block's only
// line: no empty `amend x {}` is left (log-2026-09-29 M4 U4b-r3).
func lineRemoval(f *syntax.File, line *syntax.Amendment) format.Change {
	for _, b := range f.Amends {
		if len(b.Items) == 1 && b.Items[0] == line {
			return format.Change{Kind: format.Remove, Node: b}
		}
	}
	return format.Change{Kind: format.Remove, Node: line}
}
