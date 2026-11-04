package edit

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
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

// linePart is the part of a line an edit rewrites (literalRoot): its node, its typed value, its depth,
// the steps to the edit's value, fillAt the first that reads a field left out (fillLeftOut); absent, a
// field the line leaves out beside a load or an expression (absentField), old its default.
type linePart struct {
	node   syntax.Node
	old    value.Value
	steps  []Step
	depth  int
	fillAt int
	absent bool
}

// lineValue is the line's part an edit `upto` steps down the path rewrites (DECISIONS 462).
func (x *opCtx) lineValue(at staticSite, upto int) (linePart, error) {
	node, d, ok := x.a.snap.literalRoot(at.line, at.n, x.res, max(upto, at.n))
	if !ok {
		return x.absentPart(at, upto)
	}
	sp := at.f.Span(node)
	old, err := x.typed(Source(at.f.Src.Content[sp.Start:sp.End]), x.typeAt(d))
	if err != nil {
		return linePart{}, &NotEditableError{Reason: ReasonComputed}
	}
	w, fillAt := walk{cur: old, typ: x.typeAt(d)}, 0
	for _, st := range x.res.Steps[d:upto] {
		if x.fillLeftOut(w.cur, st.Seg) && fillAt == 0 {
			fillAt = len(w.steps) + 1
		}
		if err := w.next(st.Seg); err != nil {
			return linePart{}, &NotEditableError{Reason: ReasonLayer}
		}
	}
	return linePart{node: node, old: old, steps: w.steps, depth: d, fillAt: fillAt}, nil
}

// staticEdit applies mutate to the value `upto` steps down the path inside the edit layer's
// line (W11a): its literal is compared with the new value (M1); a Set of a field to its default
// leaves it out of that literal (E6); the inverse sets the value the literal held (E23).
func (x *opCtx) staticEdit(at staticSite, upto int, mutate func(value.Value) (value.Value, error)) error {
	part, err := x.lineValue(at, upto)
	if err != nil {
		return err
	}
	if part.absent {
		return x.fieldInsert(at, part, mutate)
	}
	old, steps := part.old, part.steps
	cur := old
	if len(steps) > 0 {
		cur = steps[len(steps)-1].Value
	}
	if err := x.partInverse(part, cur, upto); err != nil {
		return err
	}
	nv, err := mutate(cur)
	if err != nil {
		return err
	}
	if rec, i, ok := fieldOf(old, steps); ok && nv != nil && x.a.isDefault(withField(rec, i, nv), i) {
		nv = nil
	}
	if fi, ok := x.leftOut(at, part, nv); ok {
		x.w.addCanon(at.f.Src.Path, x.res.root.pkg.Path, []format.Change{{Kind: format.Remove, Node: fi}})
		return nil
	}
	d := &canonDiff{a: x.a}
	if err := d.value(old, withChange(old, steps, nv), part.node); err != nil {
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
	lit, err := x.a.sourceLit(cur, x.scopeAt(k, true), stated{}) // typed from the line: its tokens kept
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
func (x *opCtx) typeAt(n int) types.Type { return typeAtDepth(x.res, n) }

// amendLine writes `path: text` in the edit layer file of the root's package, the path of
// canonical, whose steps are given: into its block for the root, replacing the lines of the
// path's descendants, or in a new block, in a new file when the package has none (W11, W11a).
func (x *opCtx) amendLine(canonical string, steps []Step, text string) error {
	rel, ok := amendPathText(steps)
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
		if x.a.snap.namesBelow(it.Path, x.res) {
			out = append(out, format.Change{Kind: format.Remove, Node: it})
		}
	}
	return out
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

// amendPathText is the path of steps as an amend path, which starts with a name (startsLine), a
// first key bare; each later key is a Canon key of its container's key type (API.md 6.4).
func amendPathText(steps []Step) (string, bool) {
	if len(steps) == 0 || !startsLine(steps[0]) {
		return "", false
	}
	var b strings.Builder
	for i, st := range steps {
		switch {
		case st.Seg.Kind != SegKey:
			b.WriteString(Path{Segs: []Seg{st.Seg}}.String())
		case i == 0:
			b.WriteString(st.Seg.Key.Text)
		default:
			b.WriteString(string(bracketOpen) + amendKeyText(st.Seg.Key, keyTypeIn(st.Container)) + bracketClose)
		}
	}
	return strings.TrimPrefix(b.String(), string(fieldMark)), true
}

// startsLine reports st a step an amend line can start with (API.md 6.4): a field, a table entry, or
// a keyed list's key whose ref chain ends at a String or a table id, as check's `.k` (DECISIONS 450,
// 456; EVALUATION.md 9.2), its name a lineWord.
func startsLine(st Step) bool {
	if st.Seg.Kind == SegField {
		return lineWord(st.Seg.Name)
	}
	l, isList := baseOf(st.Container).(*types.ListType)
	if !isList || l.KeyedBy == nil || st.Seg.Kind != SegKey || st.Seg.Key.Kind != KeyWord {
		return false
	}
	return lineWord(st.Seg.Key.Text) && baseOf(baseKeyType(l.KeyedBy.Type)).Kind() == types.String
}

// lineContinuers are the reserved words that, first on a line, continue the line above (GRAMMAR.md
// 3.1 rule 3): no amend line after another starts with one.
var lineContinuers = []syntax.TokenKind{syntax.KwAnd, syntax.KwOr, syntax.KwIn, syntax.KwIs, syntax.KwElse, syntax.KwWhere}

// lineWord reports text a WORD an amend line can start with (GRAMMAR.md 2.3, 5.7): any but a lineContinuer.
func lineWord(text string) bool {
	return isWord(text) && !slices.Contains(lineContinuers, syntax.LookupWord(text))
}

// amendKeyText is key lit, of type kt, as an amend key (API.md §6.4).
func amendKeyText(lit KeyLit, kt types.Type) string {
	if lit.Kind == KeyInt {
		return litText(lit)
	}
	if lit.Kind == KeyWord && !stringKey(lit, baseKeyType(kt)) {
		if text, ok := syntax.ValueText(lit.Text); ok {
			return text
		}
	}
	return canonQuote(lit.Text)
}

// keyTypeIn is the key type of container type ct: a table's String, a list's Int or its key
// field's, a map's; nil for another type (API.md P9).
func keyTypeIn(ct types.Type) types.Type {
	switch b := baseOf(ct).(type) {
	case *types.TableType:
		return types.StringType
	case *types.ListType:
		if b.KeyedBy != nil {
			return b.KeyedBy.Type
		}
		return types.IntType
	}
	kt, _, _ := mapTypes(ct)
	return kt
}

// baseKeyType is kt, a ref followed to its target's key type until a type that is no ref, or a
// ref back to a collection already passed (RES-09; DECISIONS 316).
func baseKeyType(kt types.Type) types.Type {
	seen := map[*types.Collection]bool{}
	for {
		r, isRef := baseOf(kt).(*types.RefType)
		if !isRef || seen[r.Target] {
			return kt
		}
		seen[r.Target], kt = true, refKeyType(r)
	}
}

// stringKey reports lit a String key of kt, or a literal of kt's literal union (API.md §6.4).
func stringKey(lit KeyLit, kt types.Type) bool {
	switch b := baseOf(kt).(type) {
	case types.Basic:
		return b.K == types.String
	case *types.LitUnionType:
		return slices.Contains(b.Literals, lit.Text) || stringKey(lit, baseKeyType(b.Of))
	}
	return false
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
