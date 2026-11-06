package edit

import (
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// dirTable is a root table a load.dir reads, its root having no value (R6), read from its files
// alone (API.md E19; DECISIONS 309): the JSON files its walk matches, in path order (WIRE.md
// 6.5), and the one an operation names (pos, -1 for none).
type dirTable struct {
	x     *opCtx
	files []load.File
	tt    *types.TableType
	cause error // the root's refusal, kept by what the files cannot decide
	pos   int
	key   string
}

// dirOps apply, from its files, the operations on a load.dir table with no value that only remove
// or rewrite source, by kind and path depth (API.md E19; DECISIONS 309).
var dirOps = map[sourceShape]func(*dirTable, Path) error{
	{OpRemove, 1}: removeDir, {OpSet, 1}: setEntryDir, {OpSet, stableDepth}: setFieldDir,
	{OpReset, stableDepth}: resetDir, {OpAddEntry, 0}: addDir, {OpMove, 1}: moveDir,
	{OpRetire, 1}: retireDir,
}

// dirSource plans x's operation on its root, a table of type tt with no value, when a load.dir
// without `at:` reads it; false for any other initializer. An operation dirOps lacks, or a walk
// the loader refuses, keeps the root's refusal, cause.
func dirSource(x *opCtx, tt *types.TableType, p Path, cause error) (bool, error) {
	e, isDir := dirLoad(x.res.root)
	if !isDir {
		return false, nil
	}
	h, known := dirOps[sourceShape{x.op.Kind, len(p.Segs)}]
	files := dirFiles(x.a.overlay(), x.a.snap, x.res.root, e)
	if !known || files == nil || slices.ContainsFunc(e.Args, func(a *syntax.Arg) bool { return a.Name != nil && a.Name.Name == loadAtOption }) {
		return true, cause
	}
	return true, h(&dirTable{x: x, files: files, tt: tt, cause: cause, pos: -1}, p)
}

// dirLoad is the load.dir of a literal glob r's let is initialized with, false for any other
// initializer.
func dirLoad(r rootRef) (*syntax.LoadExpr, bool) {
	e, isLoad := syntax.Unparen(initializer(r.obj.Decl())).(*syntax.LoadExpr)
	if !isLoad {
		return nil, false
	}
	_, _, isDir := load.DirGlob(e)
	return e, isDir
}

// dirFiles are the JSON files the load.dir e of root r matches on fsys, in path order, by the
// loader's own walk (WIRE.md 6.5); nil when the loader refuses their formats.
func dirFiles(fsys project.FS, s *Snapshot, r rootRef, e *syntax.LoadExpr) []load.File {
	return load.JSONFiles(fsys, s.a.Layout(), path.Dir(r.obj.File().Src.Path), e, diag.NewBag(nil, ""))
}

// statedDirType is the record type of the entry at path when a file of the base's load.dir walk
// states it, read from the files alone for a root with no value: an entry the before state of a
// repair holds (API.md E19, E23).
func (a *applier) statedDirType(at string) (types.Type, bool) {
	p, err := Parse(at)
	if err != nil || len(p.Segs) != 1 {
		return nil, false
	}
	root, err := a.base.lookup(p)
	key, isKey := entrySegKey(p.Segs[0])
	if err != nil || !isKey || root.enum != nil {
		return nil, false
	}
	tt, isTable := root.obj.Type().Base().(*types.TableType)
	e, isDir := dirLoad(root)
	if !isTable || !isDir {
		return nil, false
	}
	files := dirFiles(a.env.Project.FS(), a.base, root, e)
	return tt.Elem, slices.ContainsFunc(files, func(f load.File) bool { return stemOf(f.Display) == key })
}

// entry names the entry p's first segment keys, the path then the entry's: the file of that
// stem (WIRE.md 6.5), ErrNoPath when none has it, the root's refusal when two do (E3102).
func (t *dirTable) entry(p Path) error {
	key, isKey := entrySegKey(p.Segs[0])
	if !isKey {
		return t.cause
	}
	var at []int
	for i, f := range t.files {
		if stemOf(f.Display) == key {
			at = append(at, i)
		}
	}
	switch {
	case len(at) == 0:
		return &PathError{Seg: 0, Err: ErrNoPath}
	case len(at) > 1:
		return t.cause
	}
	t.pos, t.key = at[0], key
	t.x.res.Canonical = childPath(t.x.res.root.qualified(), entrySeg(value.Key{S: key}))
	return nil
}

// field is the field p's second segment names, as entry has named the entry: the root's refusal
// when its wire form is no plain member of the entry's object (WIRE.md 5.5, 5.6, 5.14) or its
// type or value needs the evaluated record (an input, a computed type, a driver, E15).
func (t *dirTable) field(p Path) (*types.Field, error) {
	if err := t.entry(p); err != nil {
		return nil, err
	}
	fields := fieldsOf(t.tt.Elem)
	i := fieldIndex(fields, p.Segs[1].Name)
	if p.Segs[1].Kind != SegField || i < 0 {
		return nil, &PathError{Seg: 1, Err: ErrNoPath}
	}
	f := fields[i]
	if f.Input != nil || f.Inline || f.Pairs != nil || len(f.WirePath) != 1 || computed(f.Type) || drives(fields, i) {
		return nil, t.cause
	}
	return f, nil
}

// display is the named entry's file.
func (t *dirTable) display() string { return t.files[t.pos].Display }

// raw is the named entry's file as edited so far.
func (t *dirTable) raw() ([]byte, error) {
	s, err := t.x.a.state(t.display())
	if err != nil {
		return nil, err
	}
	return s.cur, nil
}

// doc is the named entry's object and the bytes it is read from: the root's refusal when its
// file does not parse, its value then not being computed (DECISIONS 485), or states no object.
func (t *dirTable) doc() (*jsonsrc.Node, []byte, error) {
	raw, err := t.raw()
	if err != nil {
		return nil, nil, err
	}
	n, err := t.x.a.jsonRoot(t.display())
	if err != nil || n.Kind != jsonsrc.Object {
		return nil, nil, t.cause
	}
	return n, raw, nil
}

// write adds the edits d made to the named entry's file, doc parsed from raw, with the numbers
// the file states (M9).
func (t *dirTable) write(d *jsonDiff, doc *jsonsrc.Node, raw []byte) {
	t.noteNumbers(doc, raw)
	t.x.w.addJSON(t.display(), t.x.res.root.pkg.Path, d.out)
}

// removeDir deletes the entry's file, whatever it holds, E4 judged first (API.md N6, DECISIONS
// 309): the edit writes no JSON there (M9). Its inverse adds the entry back, then retires it again
// when it was (E23).
func removeDir(t *dirTable, p Path) error {
	if err := t.entry(p); err != nil {
		return err
	}
	if heldID(t.x, t.tt, t.key) {
		return ErrStableKey
	}
	old, retired, err := t.oldEntry()
	if err != nil {
		return err
	}
	add := Operation{Kind: OpAddEntry, Path: t.x.res.root.qualified(), Key: Key(t.key)}
	carried, err := t.carry(add, old, t.tt.Elem, nil, nil)
	switch {
	case err != nil:
		return err
	case carried && retired:
		t.x.inverse(Operation{Kind: OpRetire, Path: t.x.res.Canonical})
	}
	return t.x.removeFile(t.display(), packageDir(t.x.res.root.pkg))
}

// setEntryDir writes the whole entry, a file that parses and is not retired; its inverse sets
// the old bytes back (API.md E5, E23).
func setEntryDir(t *dirTable, p Path) error {
	if err := t.entry(p); err != nil {
		return err
	}
	doc, raw, err := t.doc()
	if err != nil {
		return err
	}
	if retired, rerr := retiredMember(); rerr != nil || objMember(doc, retired.Key) >= 0 {
		return t.cause
	}
	v, err := typedValue(t.x, t.x.op.Value, t.tt.Elem, nil)
	if err != nil {
		return err
	}
	rec, isRec := v.(*value.Record)
	if !isRec {
		return ErrBadOp
	}
	node, err := t.x.a.wireNode(rec, nil)
	if err != nil {
		return err
	}
	if _, err := t.carry(Operation{Kind: OpSet, Path: t.x.res.Canonical}, raw, t.tt.Elem, nil, nil); err != nil {
		return err
	}
	d := &jsonDiff{a: t.x.a}
	d.setNode(doc, node)
	t.write(d, doc, raw)
	lockPending(t.x, t.tt, t.key)
	return nil
}

// setFieldDir writes a field of the entry's object, replaced or inserted in declaration order
// (FORMATTER.md 14.2), a default explicitly (E19). Its inverse sets the old member back, resets
// it, or for a required field the entry lacked sets the whole old entry (API.md E23; DECISIONS 257).
func setFieldDir(t *dirTable, p Path) error {
	f, err := t.field(p)
	if err != nil {
		return err
	}
	doc, raw, err := t.doc()
	if err != nil {
		return err
	}
	v, err := typedValue(t.x, t.x.op.Value, f.Type, f)
	if err != nil {
		return err
	}
	node, err := t.x.a.wireNode(v, f)
	if err != nil {
		return err
	}
	fp, d := childPath(t.x.res.Canonical, Seg{Kind: SegField, Name: f.Name}), &jsonDiff{a: t.x.a}
	switch i := objMember(doc, f.Wire); {
	case i >= 0:
		m := doc.Members[i]
		_, err = t.carry(Operation{Kind: OpSet, Path: fp}, raw, f.Type, f, m.Value)
		d.setNode(m.Value, node)
	case hasDefault(f):
		t.x.inverse(Operation{Kind: OpReset, Path: fp})
		d.insert(doc, jsonsrc.Place(doc, memberKeys(t.tt.Elem), f.Wire), f.Wire, node)
	default:
		var old []byte
		if old, _, err = t.oldEntry(); err == nil {
			_, err = t.carry(Operation{Kind: OpSet, Path: t.x.res.Canonical}, old, t.tt.Elem, nil, nil)
		}
		d.insert(doc, jsonsrc.Place(doc, memberKeys(t.tt.Elem), f.Wire), f.Wire, node)
	}
	if err != nil {
		return err
	}
	t.write(d, doc, raw)
	lockPending(t.x, t.tt, t.key)
	return nil
}

// resetDir removes a field that has a default from the entry's object (API.md E2, E8); its
// inverse sets its old member back.
func resetDir(t *dirTable, p Path) error {
	f, err := t.field(p)
	switch {
	case err != nil:
		return err
	case !hasDefault(f):
		return ErrBadOp
	}
	doc, raw, err := t.doc()
	if err != nil {
		return err
	}
	i := objMember(doc, f.Wire)
	if i < 0 {
		return nil
	}
	m := doc.Members[i]
	if _, err := t.carry(Operation{Kind: OpSet, Path: childPath(t.x.res.Canonical, Seg{Kind: SegField, Name: f.Name})}, raw, f.Type, f, m.Value); err != nil {
		return err
	}
	d := &jsonDiff{a: t.x.a}
	d.remove(m.Value, int(m.KeySpan.Start))
	t.write(d, doc, raw)
	lockPending(t.x, t.tt, t.key)
	return nil
}

// moveDir moves no entry: file paths order a load.dir's entries (API.md E9, N1), as moveOp judges
// them, its position checked first.
func moveDir(t *dirTable, p Path) error {
	if err := t.entry(p); err != nil {
		return err
	}
	switch to := t.x.op.Index; {
	case to < 0:
		return &PathError{Seg: 1, Err: ErrBadPath}
	case to >= len(t.files):
		return &PathError{Seg: 1, Err: ErrNoPath}
	case to == t.pos:
		return nil
	}
	return &NotEditableError{Reason: ReasonOrder}
}

// retireDir retires an entry of a stable table: `"$retired": true` as its object's first key
// (API.md E10); an entry already retired stays as it is, and the inverse is none (E4).
func retireDir(t *dirTable, p Path) error {
	if err := t.entry(p); err != nil {
		return err
	}
	if !t.tt.Stable {
		return ErrBadOp
	}
	doc, raw, err := t.doc()
	if err != nil {
		return err
	}
	mark, err := retiredMember()
	if err != nil {
		return err
	}
	switch i := objMember(doc, mark.Key); {
	case i >= 0 && doc.Members[i].Value.Text == mark.Value.Text:
		return nil
	case i >= 0:
		return t.cause // `"$retired": false`, which no Retire rewrites from the file alone
	}
	t.x.w.locked = append(t.x.w.locked, Locked{Name: t.x.res.root.lockName(), Key: t.key})
	d := &jsonDiff{a: t.x.a}
	d.insert(doc, 0, mark.Key, mark.Value)
	t.write(d, doc, raw)
	return nil
}

// addDir adds an entry in a file of its own, as a load.dir element always is (API.md N1, N4).
func addDir(t *dirTable, _ Path) error {
	stated := func(key string) bool {
		return slices.ContainsFunc(t.files, func(f load.File) bool { return stemOf(f.Display) == key })
	}
	return addStated(t.x, t.tt, stated, nil, true)
}

// stemOf is the key a load.dir file gives its table entry: its name without its last extension
// (WIRE.md 6.5).
func stemOf(display string) string {
	base := path.Base(display)
	return strings.TrimSuffix(base, path.Ext(base))
}

// memberKeys are the wire keys of the fields of record type rt that write one member each, in
// declaration order.
func memberKeys(rt types.Type) []string {
	var out []string
	for _, f := range fieldsOf(rt) {
		if f.Input == nil && !f.Inline && f.Pairs == nil && len(f.WirePath) == 1 {
			out = append(out, f.Wire)
		}
	}
	return out
}
