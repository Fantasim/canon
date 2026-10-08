package edit

import (
	"errors"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// srcTable is a root table read from its sources alone, its root having no value (R6): its
// literal, the items stating its entries in evaluation order (W2), the literal's first, and the
// entry an operation names (pos, -1 for none).
type srcTable struct {
	x        *opCtx
	lit      *syntax.BraceLit
	entries  []item
	litCount int
	filed    bool // a new entry gets its own file (N1)
	tt       *types.TableType
	cause    error // the root's refusal, kept by what the sources cannot decide
	pos      int
	key      string
}

// sourceShape is an operation a srcTable applies: its kind and how many segments its path has.
type sourceShape struct {
	kind  Op
	depth int
}

// sourceOps apply, from its sources, the operations on a root table with no value that only
// remove or rewrite source, by kind and path depth (API.md E19; DECISIONS 309).
var sourceOps = map[sourceShape]func(*srcTable, Path) error{
	{OpRemove, 1}: removeSource, {OpMove, 1}: moveSource, {OpSet, 1}: setEntrySource, {OpRetire, 1}: retireSource,
	{OpSet, stableDepth}: setFieldSource, {OpReset, stableDepth}: resetSource, {OpAddEntry, 0}: addSource,
}

// poisonedRoot reports err, a resolution's refusal, as the root having no value (R6).
func poisonedRoot(err error) bool {
	var pe *PathError
	return errors.As(err, &pe) && pe.Seg == rootSeg && errors.Is(pe.Err, ErrNoValue)
}

// sourceOp plans op on a root without a value from its sources (API.md E1, E19; DECISIONS 309):
// on a table, the operations of sourceOps; on any other root, a Set of the whole root. Any
// other operation, or a root its sources do not state, keeps the root's refusal, cause.
func (a *applier) sourceOp(op Operation, p Path, cause error) (*work, error) {
	root, ok := a.sourceRoot(p)
	if !ok {
		return nil, cause
	}
	x := &opCtx{a: a, op: op, res: resolution{Resolved: Resolved{Canonical: root.qualified()}, root: root}, w: newWork()}
	tt, isTable := root.obj.Type().Base().(*types.TableType)
	if !isTable {
		if op.Kind != OpSet || len(p.Segs) != 0 {
			return nil, cause
		}
		return x.w, setRootSource(x, cause)
	}
	h, known := sourceOps[sourceShape{op.Kind, len(p.Segs)}]
	entries, stated := a.snap.statedEntries(root)
	if !known || !stated {
		return nil, cause
	}
	lit := braceOf(syntax.Unparen(initializer(root.obj.Decl())))
	t := &srcTable{x: x, lit: lit, entries: entries, tt: tt, cause: cause, pos: -1}
	t.litCount = len(entries) - len(a.snap.entryDecls(root))
	t.filed = filesLet(root.obj.Decl()) || slices.ContainsFunc(entries[t.litCount:], func(it item) bool { return it.file != root.obj.File() })
	if err := h(t, p); err != nil {
		return nil, err
	}
	return x.w, nil
}

// sourceRoot is the let p names, when its sources alone may be edited: no edit layer, no layer
// amending it, in a visible .canon file.
func (a *applier) sourceRoot(p Path) (rootRef, bool) {
	root, err := a.snap.lookup(p)
	if err != nil || root.enum != nil || a.env.EditLayer != "" || a.snap.amendedLets()[root.obj] {
		return rootRef{}, false
	}
	_, isLet := root.obj.Decl().(*syntax.LetDecl)
	if !isLet || !visible(root.obj.File().Src.Path) {
		return rootRef{}, false
	}
	a.named = append(a.named, root.at())
	return root, true
}

// entry names the entry p's first segment keys, the path then the entry's: ErrNoPath when the
// sources state none, the root's refusal for one in a hidden file.
func (t *srcTable) entry(p Path) error {
	key, isKey := entrySegKey(p.Segs[0])
	if !isKey {
		return t.cause
	}
	pos := slices.IndexFunc(t.entries, func(it item) bool { return statedKey(it.node) == key })
	switch {
	case pos < 0:
		return &PathError{Seg: 0, Err: ErrNoPath}
	case !visible(t.entries[pos].file.Src.Path):
		return t.cause
	}
	t.pos, t.key = pos, key
	t.x.res.Canonical = childPath(t.x.res.root.qualified(), entrySeg(value.Key{S: key}))
	return nil
}

// field is the field p's second segment names, as entry has named the entry: the root's refusal
// when the entry has a spread (W9) or the field's type or value needs the evaluated record
// (an input, a computed type, a driver of a dependent field, E15).
func (t *srcTable) field(p Path) (int, *types.Field, error) {
	if err := t.entry(p); err != nil {
		return 0, nil, err
	}
	fields := fieldsOf(t.tt.Elem)
	i := fieldIndex(fields, p.Segs[1].Name)
	switch {
	case p.Segs[1].Kind != SegField || i < 0:
		return 0, nil, &PathError{Seg: 1, Err: ErrNoPath}
	case hasSpread(braceOf(t.node().node)) || fields[i].Input != nil || computed(fields[i].Type) || drives(fields, i):
		return 0, nil, t.cause
	}
	return i, fields[i], nil
}

// statedEntries are the entries a root table's sources state, in evaluation order: its
// literal's, then its entry declarations (W2); false when its value is no literal.
func (s *Snapshot) statedEntries(r rootRef) ([]item, bool) {
	e := syntax.Unparen(initializer(r.obj.Decl()))
	lit := braceOf(e)
	if lit == nil || shape.SourceForm(s.info, e) != shape.FormLiteral || !visible(r.obj.File().Src.Path) {
		return nil, false
	}
	var out []item
	for _, it := range lit.Items {
		if ei, ok := it.(*syntax.EntryItem); ok {
			out = append(out, item{ei, r.obj.File()})
		}
	}
	return append(out, s.entryDecls(r)...), true
}

// statedType is the record type of the entry at path when s's sources state it, read from them
// for a root with no value: an entry the before state of a repair holds (API.md E19, E23).
func (s *Snapshot) statedType(path string) (types.Type, bool) {
	p, err := Parse(path)
	if err != nil || len(p.Segs) != 1 {
		return nil, false
	}
	root, err := s.lookup(p)
	key, isKey := entrySegKey(p.Segs[0])
	if err != nil || !isKey || root.enum != nil {
		return nil, false
	}
	tt, isTable := root.obj.Type().Base().(*types.TableType)
	entries, stated := s.statedEntries(root)
	if !isTable || !stated {
		return nil, false
	}
	return tt.Elem, slices.ContainsFunc(entries, func(it item) bool { return statedKey(it.node) == key })
}

// entrySegKey is the entry key a path segment names: `.key` or `["key"]`.
func entrySegKey(seg Seg) (string, bool) {
	switch {
	case seg.Kind == SegField:
		return seg.Name, true
	case seg.Kind == SegKey && seg.Key.Kind != KeyInt:
		return seg.Key.Text, true
	}
	return "", false
}

// statedKey is the key an entry item or declaration states.
func statedKey(n syntax.Node) string {
	switch x := n.(type) {
	case *syntax.EntryItem:
		return x.Key.Name
	case *syntax.EntryDecl:
		if id, ok := x.Key.(*syntax.Ident); ok {
			return id.Name
		}
	}
	return ""
}

// retiredItem reports an entry stated `retired`.
func retiredItem(n syntax.Node) bool {
	var mods *syntax.Modifiers
	switch x := n.(type) {
	case *syntax.EntryItem:
		mods = x.Mods
	case *syntax.EntryDecl:
		mods = x.Mods
	}
	return mods != nil && mods.Retired.Valid()
}
