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
	case x.stableTable():
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
	old := keyOfItem(parent, x.res.Target)
	_, isTable := parent.(*value.Table)
	switch {
	case isTable && !isWord(k.CanonText()):
		return &ValueError{Expected: kt.String(), Got: describe(x.op.Key), Detail: detailNotWord}
	case sameValue(old, k):
		return nil
	case holdsKey(parent, k):
		return ErrKeyExists
	}
	refs, err := x.renameRefs(parent)
	if err != nil {
		return err
	}
	x.inverse(Operation{Kind: OpRename, Path: childPath(x.parentPath(), keySegOf(parent, k, kt)), Key: keyLit(old)})
	r := &renaming{x: x, from: old, to: k, text: map[string][]format.Change{}, done: map[tokenPlace]bool{}, pkgs: map[string]string{}}
	if err := r.key(parent); err != nil {
		return err
	}
	for _, ref := range refs {
		if err := r.retoken(ref.Span, ref.Package); err != nil {
			return err
		}
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

// renameRefs are the references a rename rewrites (E11): none for a map entry, which nothing
// references; a value or key reference that cannot be edited refuses the rename (E12).
func (x *opCtx) renameRefs(parent value.Value) ([]Ref, error) {
	if _, isMap := parent.(*value.Map); isMap {
		return nil, nil
	}
	refs, err := x.a.snap.Refs(x.a.ctx, x.res.Resolved)
	if err != nil {
		return nil, err
	}
	var stuck []string
	reason := ReasonNone
	for _, r := range refs {
		if r.Kind != RefValue && r.Kind != RefKey {
			continue
		}
		if why := x.a.refReason(r); why != ReasonNone {
			stuck = append(stuck, r.Package+packageMark+r.Path)
			reason = cmpOr(reason, why)
		}
	}
	if len(stuck) > 0 {
		return nil, &NotEditableError{Reason: reason, Refs: stuck}
	}
	return refs, nil
}

// cmpOr is r unless it is ReasonNone, else why.
func cmpOr(r, why Reason) Reason {
	if r != ReasonNone {
		return r
	}
	return why
}

// refReason is why the value or map entry a reference states cannot be edited, ReasonNone
// when it can (API.md E12).
func (a *applier) refReason(r Ref) Reason {
	res, err := a.resolve(r.Package + packageMark + r.Path)
	if err != nil {
		return ReasonComputed
	}
	return a.snap.judge(res, OpSet, a.env.EditLayer).reason()
}

// renaming is one rename's token replacements, by .canon file, and its JSON edits.
type renaming struct {
	x        *opCtx
	from, to value.Value
	text     map[string][]format.Change
	done     map[tokenPlace]bool // the tokens already rewritten
	pkgs     map[string]string   // the package of each file written, by display path
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
	text, err := r.tokenText(n)
	if err != nil {
		return err
	}
	r.text[f.Src.Path] = append(r.text[f.Src.Path], format.Change{Kind: format.Replace, Node: n, Text: text})
	return nil
}

// tokenText is the new key written as node n writes the old one.
func (r *renaming) tokenText(n syntax.Node) (string, error) {
	word := r.to.CanonText()
	switch n.(type) {
	case *syntax.StringLit, *syntax.RawStringLit:
		return quoted(r.to), nil
	case *syntax.IntLit:
		return word, nil
	case *syntax.IdentExpr:
		if !isWord(word) {
			return quoted(r.to), nil
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
	for display, changes := range r.text { //canon:unordered each file's changes are added under its own name
		r.x.w.addCanon(display, r.pkgOf(display), changes)
	}
}
