package edit

import (
	"path"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// key changes the renamed item's own key (API.md E11): the entry's name, the `entry` line, the
// key field or the map key; a load.dir table's file is its key (LOD-05), and an @files
// file named by the old key is renamed (N8).
func (r *renaming) key(parent value.Value) error {
	x := r.x
	c, pc := x.j.last(), x.j.holder()
	switch {
	case pc.files && c.mode == ModeJSON:
		return r.fileKey(parent)
	case c.mode == ModeJSON:
		return r.jsonKey(parent)
	}
	if err := r.canonKey(parent, c, pc); err != nil {
		return err
	}
	return r.renameFile(c)
}

// canonKey replaces the key token of a table entry, an `entry` line, a keyed element's key
// field or a map item's key.
func (r *renaming) canonKey(parent value.Value, c, pc cursor) error {
	switch n := c.node.(type) {
	case *syntax.EntryItem:
		return r.replace(c.file, n.Key, r.to.CanonText(), true)
	case *syntax.EntryDecl:
		text, err := entryKeyText(r.to)
		if err != nil {
			return err
		}
		return r.replace(c.file, n.Key, text, false)
	}
	if l, ok := parent.(*value.List); ok {
		lt, _ := l.T.Base().(*types.ListType)
		fi := fieldItem(c.node, lt.KeyedBy.Name)
		if fi == nil {
			return &NotEditableError{Reason: ReasonKey}
		}
		text, err := r.x.a.canonText(r.to)
		if err != nil {
			return err
		}
		return r.replace(c.file, syntax.Unparen(fi.Value), text, false)
	}
	return r.mapKey(itemOf(pc.node, c.node), pc.file)
}

// mapKey replaces a map item's key: an expression, or a bare name.
func (r *renaming) mapKey(item syntax.Node, f *syntax.File) error {
	switch it := item.(type) {
	case *syntax.MapItem:
		text, err := r.x.a.canonText(r.to)
		if err != nil {
			return err
		}
		return r.replace(f, syntax.Unparen(it.Key), text, false)
	case *syntax.FieldItem:
		return r.replace(f, it.Name, r.to.CanonText(), true)
	}
	return &NotEditableError{Reason: ReasonComputed}
}

// replace writes text over n; a word checks the text is one.
func (r *renaming) replace(f *syntax.File, n syntax.Node, text string, word bool) error {
	if word && !isWord(text) {
		return badTemplate(r.to)
	}
	sp := f.Span(n)
	at := tokenPlace{file: f.Src.Path, start: sp.Start, end: sp.End}
	if r.done[at] {
		return nil // a reference's token a cascade reaches again (E11)
	}
	r.done[at] = true
	r.text[f.Src.Path] = append(r.text[f.Src.Path], format.Change{Kind: format.Replace, Node: n, Text: text})
	return nil
}

// renameFile renames the entry's own file when its path is the @files template of the old
// entry: to the template of the renamed one (API.md E11, N8).
func (r *renaming) renameFile(c cursor) error {
	x := r.x
	let, ok := x.res.root.obj.Decl().(*syntax.LetDecl)
	if _, isDecl := c.node.(*syntax.EntryDecl); !ok || !isDecl {
		return nil
	}
	tpl, ok := filesTemplate(let)
	rec, isRec := x.res.Target.(*value.Record)
	if !ok || !isRec {
		return nil
	}
	return r.moveByTemplate(tpl, rec, c.file.Src.Path)
}

// moveByTemplate renames file when the template names it after the old key.
func (r *renaming) moveByTemplate(tpl string, rec *value.Record, file string) error {
	x := r.x
	dir := packageDir(x.res.root.pkg)
	if !r.namedBy(tpl, rec, file) {
		return nil
	}
	newKey, err := templateText(r.to)
	if err != nil {
		return err
	}
	newRel, err := x.expand(tpl, r.renamed(rec), newKey)
	if err != nil {
		return err
	}
	return r.move(file, path.Join(dir, newRel), dir)
}

// namedBy reports a file whose path is the template expanded for the old entry (API.md E11).
func (r *renaming) namedBy(tpl string, rec *value.Record, file string) bool {
	oldKey, keyErr := templateText(r.from)
	if keyErr != nil {
		return false
	}
	oldRel, tplErr := r.x.expand(tpl, rec, oldKey)
	return tplErr == nil && path.Join(packageDir(r.x.res.root.pkg), oldRel) == file
}

// renamed is rec with its key field set to the new key, for a keyed list's element.
func (r *renaming) renamed(rec *value.Record) *value.Record {
	lt, ok := baseOf(r.x.res.Steps[len(r.x.res.Steps)-1].Container).(*types.ListType)
	if !ok || lt.KeyedBy == nil {
		return rec
	}
	return withField(rec, lt.KeyedBy.Index, r.to)
}

// move renames file to dest, which must be free (N3, N8).
func (r *renaming) move(file, dest, stop string) error {
	if file == dest {
		return nil
	}
	if err := r.x.a.free(dest); err != nil {
		return err
	}
	r.x.w.moves = append(r.x.w.moves, fileMove{from: file, to: dest, stop: stop, json: r.x.a.snap.tree(file) == nil})
	r.x.w.own(dest, r.x.res.root.pkg.Path)
	r.x.w.own(file, r.x.res.root.pkg.Path)
	return nil
}

// fileKey renames a load.dir element: a table's file, whose stem is the key; a keyed list's
// key field, and its file when an @files template names it.
func (r *renaming) fileKey(parent value.Value) error {
	file := r.x.jsonFileOf()
	if _, isTable := parent.(*value.Table); isTable {
		dest := path.Join(path.Dir(file), r.to.CanonText()+path.Ext(file))
		return r.move(file, dest, packageDir(r.x.res.root.pkg))
	}
	if err := r.jsonKey(parent); err != nil {
		return err
	}
	let, ok := r.x.res.root.obj.Decl().(*syntax.LetDecl)
	rec, isRec := r.x.res.Target.(*value.Record)
	if tpl, has := filesTemplate(let); ok && has && isRec {
		return r.moveByTemplate(tpl, rec, file)
	}
	return nil
}

// jsonKey changes the key in a JSON source: an object member's name, or a keyed element's key field.
func (r *renaming) jsonKey(parent value.Value) error {
	x := r.x
	n, display, err := x.jsonAt(x.res.Target)
	if err != nil {
		return err
	}
	if l, ok := parent.(*value.List); ok {
		lt, _ := l.T.Base().(*types.ListType)
		m, _, _ := memberAt(n, lt.KeyedBy.WirePath)
		if m == nil {
			return &NotEditableError{Reason: ReasonKey}
		}
		d := &jsonDiff{a: x.a}
		if err := d.set(r.to, m, lt.KeyedBy); err != nil {
			return err
		}
		r.done[tokenPlace{file: display, start: m.Span.Start, end: m.Span.End}] = true
		x.w.addJSON(display, x.res.root.pkg.Path, d.out)
		return nil
	}
	item, display, err := x.jsonItem()
	if err != nil {
		return err
	}
	return r.renameMember(display, item)
}

// renameMember gives the member holding n the new key, at its place (FORMATTER.md §14.2).
func (r *renaming) renameMember(display string, n *jsonsrc.Node) error {
	holder, err := r.x.a.jsonHolder(display, n)
	if err != nil {
		return err
	}
	key, err := wireKey(r.to)
	if err != nil {
		return err
	}
	i := memberOf(holder, n)
	if i < 0 {
		return errNoMember
	}
	ks := holder.Members[i].KeySpan
	r.done[tokenPlace{file: display, start: ks.Start, end: ks.End}] = true
	rn := &keyRename{holder: holder.Pointer(), from: holder.Members[i].Key, to: key}
	r.x.w.addJSON(display, r.pkgOf(display), []jsonEdit{{anchor: int(ks.Start), rename: rn}})
	return nil
}

// memberOf is the index of the member whose value is n, -1 when there is none.
func memberOf(holder, n *jsonsrc.Node) int {
	for i, m := range holder.Members {
		if m.Value.Span == n.Span {
			return i
		}
	}
	return -1
}

// jsonToken replaces a reference in a JSON source: a string or number value, or a member key.
func (r *renaming) jsonToken(display string, sp source.Span) error {
	root, err := r.x.a.jsonRoot(display)
	if err != nil {
		return err
	}
	if n := nodeAt(root, sp); n != nil {
		node, err := r.x.a.wireNode(r.to, nil)
		if err != nil {
			return err
		}
		r.x.w.addJSON(display, r.pkgOf(display), []jsonEdit{{e: jsonsrc.Edit{Kind: jsonsrc.Set, Pointer: n.Pointer(), Value: node}, anchor: int(sp.Start)}})
		return nil
	}
	if holder, v := memberKeyAt(root, sp); holder != nil {
		return r.renameMember(display, v)
	}
	return &NotEditableError{Reason: ReasonComputed}
}

// nodeAt is the scalar node of a JSON tree spanning exactly sp.
func nodeAt(n *jsonsrc.Node, sp source.Span) *jsonsrc.Node {
	if n.Span.Start == sp.Start && n.Span.End == sp.End && n.Kind != jsonsrc.Object && n.Kind != jsonsrc.Array {
		return n
	}
	for _, e := range n.Elems {
		if found := nodeAt(e, sp); found != nil {
			return found
		}
	}
	for _, m := range n.Members {
		if found := nodeAt(m.Value, sp); found != nil {
			return found
		}
	}
	return nil
}

// memberKeyAt is the object and member value whose key token spans exactly sp.
func memberKeyAt(n *jsonsrc.Node, sp source.Span) (*jsonsrc.Node, *jsonsrc.Node) {
	for _, m := range n.Members {
		if m.KeySpan.Start == sp.Start && m.KeySpan.End == sp.End {
			return n, m.Value
		}
		if h, v := memberKeyAt(m.Value, sp); h != nil {
			return h, v
		}
	}
	for _, e := range n.Elems {
		if h, v := memberKeyAt(e, sp); h != nil {
			return h, v
		}
	}
	return nil, nil
}

// wireKey is a key as a JSON member name, a symbol's the text it was read as (WIRE.md §5.8).
func wireKey(k value.Value) (string, error) {
	switch x := k.(type) {
	case *value.Str:
		return x.V, nil
	case *value.Symbol:
		return x.Name, nil
	}
	return wire.KeyText(k)
}

// pkgOf is the package that owns the file at display: a reference's, else the root's.
func (r *renaming) pkgOf(display string) string {
	if pkg, ok := r.pkgs[display]; ok {
		return pkg
	}
	return r.x.res.root.pkg.Path
}
