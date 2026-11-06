package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// renameOp changes the key of a table entry, a keyed-list element or a map entry, and every
// reference to it (API.md E11-E13): all or nothing.
func renameOp(x *opCtx) error {
	parent, _, ok := x.sibling()
	switch {
	case !ok:
		return ErrBadOp
	case x.lockedKey():
		return ErrStableKey
	}
	kt, ok := keyType(parent)
	if !ok {
		return ErrBadOp
	}
	k, err := x.key(kt)
	if err != nil {
		return err
	}
	if err := x.refKeyHeld(k, kt); err != nil {
		return err
	}
	old := keyOfItem(parent, x.res.Target)
	_, isTable := parent.(*value.Table)
	if isTable {
		if err := x.tableKey(k.CanonText(), x.ownCanon()); err != nil {
			return err
		}
	}
	switch {
	case sameValue(old, k):
		return nil
	case holdsKey(parent, k):
		return ErrKeyExists
	}
	return x.renameTo(parent, old, k, kt)
}

// renameTo rewrites the key old of the target of parent to k, every reference to it and the
// keyed-list elements it keys (API.md E11, E12; DECISIONS 310).
func (x *opCtx) renameTo(parent, old, k value.Value, kt types.Type) error {
	refs, cascades, err := x.renameRefs(parent)
	if err != nil {
		return err
	}
	renamed := childPath(x.parentPath(), keySegOf(parent, k, kt))
	x.inverse(Operation{Kind: OpRename, Path: renamed, Key: keyLit(old)}) // it cascades as this one does
	x.w.rekey = append(x.w.rekey, pathMove{from: x.res.Canonical, to: renamed})
	r := &renaming{x: x, from: old, to: k, text: map[string][]format.Change{}, done: map[tokenPlace]bool{}, pkgs: map[string]string{}, moved: map[string]bool{}, items: map[*syntax.FieldItem]keyedName{}}
	if err := r.key(parent); err != nil {
		return err
	}
	for _, ref := range refs {
		if err := r.rewrite(ref); err != nil {
			return err
		}
	}
	if err := r.layerKeys(); err != nil {
		return err
	}
	if err := r.cascade(cascades); err != nil {
		return err
	}
	r.flush()
	return nil
}

// keyType is the type of the keys of a table (String), a keyed list or a map.
func keyType(c value.Value) (types.Type, bool) {
	switch x := c.(type) {
	case *value.Table:
		return types.StringType, true
	case *value.List:
		if lt, ok := x.T.Base().(*types.ListType); ok && lt.KeyedBy != nil {
			return lt.KeyedBy.Type, true
		}
	case *value.Map:
		kt, _, ok := mapTypes(x.T)
		return kt, ok
	}
	return nil, false
}

// keyOfItem is the key of item v of collection c.
func keyOfItem(c, v value.Value) value.Value {
	switch x := c.(type) {
	case *value.Table:
		return &value.Str{V: entryKey(v.(*value.Record)).S, T: types.StringType}
	case *value.List:
		lt, _ := x.T.Base().(*types.ListType)
		return keyField(v, lt.KeyedBy)
	case *value.Map:
		return x.Keys[position(x, v)]
	}
	return nil
}

// holdsKey reports a collection with an item keyed k.
func holdsKey(c, k value.Value) bool {
	switch x := c.(type) {
	case *value.Table:
		return entryIndex(x.Entries, k.CanonText()) >= 0
	case *value.List:
		lt, _ := x.T.Base().(*types.ListType)
		return hasElemKey(x, lt.KeyedBy, k)
	case *value.Map:
		return slices.ContainsFunc(x.Keys, func(o value.Value) bool { return sameValue(o, k) })
	}
	return false
}

// keySegOf is the canonical segment of key k in collection c (API.md P8).
func keySegOf(c, k value.Value, kt types.Type) Seg {
	if _, ok := c.(*value.Table); ok {
		return entrySeg(value.Key{S: k.CanonText()})
	}
	return keySeg(k, kt)
}

// renameRefs are the references a rename rewrites (E11), none for a map entry; one not editable refuses
// it (E12), a keyed-list key field moves its element (DECISIONS 310), and a computed one has no token:
// it, and its let's name, are never rewritten (DECISIONS 503).
func (x *opCtx) renameRefs(parent value.Value) ([]Ref, []resolution, error) {
	if _, isMap := parent.(*value.Map); isMap {
		return nil, nil, nil
	}
	l, err := x.a.snap.refsOf(x.a.ctx, x.res.Resolved) // the plan's: what the rename rewrites
	if err != nil {
		return nil, nil, x.otherRoot(err)
	}
	why, moved := x.a.planSide().judge(l)
	if why, err = x.sessionStuck(why, l); err != nil {
		return nil, nil, err
	}
	if stuck := why.all(); len(stuck) > 0 {
		return nil, nil, &NotEditableError{Reason: why.reason, Layer: why.layer, Refs: stuck}
	}
	return slices.DeleteFunc(slices.Clone(l.refs), func(r Ref) bool { return l.computed[r] }), moved, nil
}

// blame is r unless it is ReasonNone, else why; `layered` wins over any other, the more specific (DECISIONS 464).
func blame(r, why Reason) Reason {
	if r != ReasonNone && why != ReasonLayered {
		return r
	}
	return why
}

// renaming is one rename's token replacements, by .canon file, and its JSON edits.
type renaming struct {
	x        *opCtx
	from, to value.Value
	text     map[string][]format.Change
	done     map[tokenPlace]bool             // the tokens already rewritten
	pkgs     map[string]string               // the package of each file written, by display path
	moved    map[string]bool                 // the keyed-list elements a cascade moved, by old path (E11)
	items    map[*syntax.FieldItem]keyedName // the map items whose word key becomes a string (keyedItems)
}

// tokenPlace is a token of a file by its display path and offsets, whichever file set read it.
type tokenPlace struct {
	file       string
	start, end source.Pos
}

// retoken replaces the token at span, which names the old key, by the new key in the same
// form: a bare name, a string, an integer, a JSON string, number or member key (E11).
func (r *renaming) retoken(sp source.Span, pkg string) error {
	display := r.x.a.snap.display(sp.File)
	at := tokenPlace{file: display, start: sp.Start, end: sp.End}
	if r.done[at] {
		return nil
	}
	r.done[at] = true
	r.pkgs[display] = pkg
	if f := r.x.a.snap.files[sp.File]; f != nil {
		return r.canonToken(f, sp)
	}
	return r.jsonToken(display, sp)
}

// canonToken replaces a name, string or integer token of a .canon file.
func (r *renaming) canonToken(f *syntax.File, sp source.Span) error {
	n := tokenNode(f, sp)
	if n == nil {
		return &NotEditableError{Reason: ReasonComputed}
	}
	text, err := r.tokenText(f, n)
	if err != nil {
		return err
	}
	if id, isName := n.(*syntax.Ident); isName && !isWord(text) {
		if fi, _ := r.x.a.snap.mapItemOf(f, id); fi != nil {
			r.items[fi] = keyedName{f: f, key: text}
			return nil
		}
	}
	r.text[f.Src.Path] = append(r.text[f.Src.Path], format.Change{Kind: format.Replace, Node: n, Text: text})
	return nil
}

// tokenText is the new key written as node n of f writes the old one; a ref's word a name in scope
// at n would read is a string (DECISIONS 318), a member never: step 1 reads it first (490, 495, 496).
func (r *renaming) tokenText(f *syntax.File, n syntax.Node) (string, error) {
	word := r.to.CanonText()
	_, member := r.to.(*value.Member)
	switch x := n.(type) {
	case *syntax.StringLit, *syntax.RawStringLit:
		return quoted(r.to), nil
	case *syntax.IntLit:
		return word, nil
	case *syntax.IdentExpr: // a reference in value position: a word bare or quoted, else a string (E11)
		if text, ok := syntax.ValueText(word); ok && (member || text != word || !r.x.a.snap.scopeOf(f, x).names(word)) {
			return text, nil
		}
		return quoted(r.to), nil
	case *syntax.Ident: // a map literal's word key, read as the key's type reads a value (TYPES.md 4.1)
		if _, kt := r.x.a.snap.mapItemOf(f, x); kt != nil && isWord(word) {
			return r.x.a.snap.scopeOf(f, x).keyText(value.Key{S: word}, kt, word), nil
		}
	}
	if !isWord(word) {
		return "", badTemplate(r.to)
	}
	return word, nil
}

// quoted is a key as a string literal.
func quoted(k value.Value) string {
	if s, ok := k.(*value.Str); ok {
		return canonQuote(s.V)
	}
	return canonQuote(k.CanonText())
}

// tokenNode is the name, string or integer node of f spanning exactly sp; a selector's name.
func tokenNode(f *syntax.File, sp source.Span) syntax.Node {
	var found syntax.Node
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n == nil || found != nil {
			return false
		}
		s := f.Span(n)
		if s.Start > sp.Start || s.End < sp.End {
			return false
		}
		switch x := n.(type) {
		case *syntax.Ident, *syntax.IdentExpr, *syntax.StringLit, *syntax.RawStringLit, *syntax.IntLit:
			if s == sp {
				found = x
			}
		case *syntax.SelectorExpr:
			if s == sp {
				found = x.Name
			}
		}
		return found == nil
	})
	return found
}

// flush adds the replacements to the operation's work, by file.
func (r *renaming) flush() {
	r.keyedItems()
	for display, changes := range r.text { //canon:unordered each file's changes are added under its own name
		r.x.w.addCanon(display, r.pkgOf(display), changes)
	}
}

// refKeyHeld refuses a new key of type `ref C` that names no entry of C, whatever AllowErrors
// says: written, it would leave the keyed list without a value (API.md E3, V1; DECISIONS 316).
func (x *opCtx) refKeyHeld(k value.Value, kt types.Type) error {
	r, isRef := k.(*value.Ref)
	rt, typed := baseOf(kt).(*types.RefType)
	if !isRef || !typed || rt.Target == nil || rt.Target.Kind != types.CollLet {
		return nil
	}
	p := Path{Package: rt.Target.Pkg, Root: rt.Target.Name}
	for _, f := range rt.Target.FieldPath {
		p.Segs = append(p.Segs, Seg{Kind: SegField, Name: f})
	}
	p.Segs = append(p.Segs, entrySeg(r.Key))
	if _, err := Resolve(x.a.snap, p); err != nil {
		return &ValueError{Expected: kt.String(), Got: describe(x.op.Key), Detail: detailNoEntry}
	}
	return nil
}
