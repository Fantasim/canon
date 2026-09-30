package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// pairDecls pairs sw's declarations: one both versions hold is kept, an entry or annotated let
// changed only in its value is checked again, anything else refuses; sharing or a let checked
// again needs a new version free of parse findings.
func (c *checker) pairDecls(sw swap, bag *diag.Bag) (swap, bool) {
	objs := map[syntax.Node]*object{}
	for _, o := range sw.pkg.all {
		if o.file == sw.old {
			objs[o.decl] = o
		}
	}
	for _, at := range sw.pkg.entries {
		if at.file == sw.old {
			objs[at.decl] = at.obj
		}
	}
	wide := false
	for i, d := range sw.old.Decls {
		pair, ok := c.pairOf(sw, objs, d, sw.new.Decls[i])
		if !ok {
			return swap{}, false
		}
		_, isLet := d.(*syntax.LetDecl)
		wide = wide || pair.shared() || isLet
		sw.pairs = append(sw.pairs, pair)
	}
	if wide && !parseClean(bag, sw.new) {
		return swap{}, false
	}
	sw.shared = sharedNodes(sw)
	return sw, true
}

// pairOf is the pair of one declaration of sw's old version and its new one.
func (c *checker) pairOf(sw swap, objs map[syntax.Node]*object, a, b syntax.Decl) (declPair, bool) {
	if a == b {
		return declPair{old: a, new: b}, !c.syntaxHeld[a]
	}
	o := objs[a]
	if o == nil {
		return declPair{}, false
	}
	switch x := a.(type) {
	case *syntax.EntryDecl:
		y, ok := b.(*syntax.EntryDecl)
		ok = ok && sameSignature(sw.old, sw.new, x, y) && typeless(x.Value) && typeless(y.Value)
		return declPair{old: a, new: b, obj: o}, ok
	case *syntax.LetDecl:
		y, ok := b.(*syntax.LetDecl)
		if !ok || !c.sameLet(sw, o, x, y) {
			return declPair{}, false
		}
		rows, whole := c.rowsOf(x)
		return declPair{old: a, new: b, obj: o, rows: rows}, whole
	}
	return declPair{}, false
}

// sameLet reports two versions of an annotated let alike but for their value and doc comment:
// name, modifiers, annotations and a `where`-free type as written, and a value of the same form,
// writing no type, whose table rows or keyed list keys are written alike, in order.
func (c *checker) sameLet(sw swap, o *object, a, b *syntax.LetDecl) bool {
	of, nf := sw.old, sw.new
	if a.Type == nil || b.Type == nil || !o.resolvedLet() {
		return false
	}
	if writtenIn(of, a.Name) != writtenIn(nf, b.Name) || writtenIn(of, a.Mods) != writtenIn(nf, b.Mods) ||
		writtenIn(of, a.Type) != writtenIn(nf, b.Type) || !sameAnnotations(of, nf, a.Annotations, b.Annotations) {
		return false
	}
	if holdsWhere(a.Type) || !typeless(a.Value) || !typeless(b.Value) || valueForm(a.Value) != valueForm(b.Value) {
		return false
	}
	return sameRows(of, nf, a.Value, b.Value) && slices.Equal(c.writtenKeyTexts(o, a.Value), c.writtenKeyTexts(o, b.Value))
}

// sameRows reports table literal rows keeping keys, modifiers and annotations (TYPES.md §16).
func sameRows(of, nf *syntax.File, a, b syntax.Expr) bool {
	x, isLit := a.(*syntax.BraceLit)
	y, isNew := b.(*syntax.BraceLit)
	if !isLit || !isNew || !isTableLiteral(x) {
		return true
	}
	if len(x.Items) != len(y.Items) {
		return false
	}
	for i, it := range x.Items {
		ex, ey := it.(*syntax.EntryItem), y.Items[i].(*syntax.EntryItem)
		if ex.Key.Name != ey.Key.Name || writtenIn(of, ex.Mods) != writtenIn(nf, ey.Mods) ||
			!sameAnnotations(of, nf, ex.Annotations, ey.Annotations) {
			return false
		}
	}
	return true
}

// resolvedLet reports a let whose annotation resolved, typed at that.
func (o *object) resolvedLet() bool {
	return o.kind == ObjLet && o.state == stateDone && o.typ != nil
}

// holdsWhere reports a written type holding a `where` predicate, which step 3 checks.
func holdsWhere(t syntax.Type) bool {
	found := false
	syntax.Inspect(t, func(n syntax.Node) bool {
		_, isWhere := n.(*syntax.WhereType)
		found = found || isWhere
		return !found
	})
	return found
}

// valueForm is the form of a let's value: a table literal, a list literal, `load.defines`, or other.
func valueForm(v syntax.Expr) int {
	switch x := v.(type) {
	case *syntax.BraceLit:
		if isTableLiteral(x) {
			return formTable
		}
	case *syntax.ListLit:
		return formList
	case *syntax.LoadExpr:
		if x.Method != nil && x.Method.Name == loadDefines {
			return formDefines
		}
	}
	return formOther
}

// writtenKeyTexts are the key values a keyed list literal writes, in order, "" for a computed one.
func (c *checker) writtenKeyTexts(o *object, v syntax.Expr) []string {
	x, ok := v.(*syntax.ListLit)
	if !ok {
		return nil
	}
	_, keyed, isColl := collectionElem(o.typ)
	if !isColl || keyed == nil {
		return nil
	}
	out := make([]string, 0, len(x.Elems))
	for _, el := range x.Elems {
		k, _ := literalKey(el, keyed)
		out = append(out, keyText(k))
	}
	return out
}

// keyText is a written key's text, "" for none.
func keyText(v value.Value) string {
	if v == nil {
		return ""
	}
	return v.CanonText()
}

// rowsOf are the objects of a let's table literal rows, in order, none for another value; false
// when a row has none.
func (c *checker) rowsOf(d *syntax.LetDecl) ([]*object, bool) {
	lit, ok := d.Value.(*syntax.BraceLit)
	if !ok || !isTableLiteral(lit) {
		return nil, true
	}
	var out []*object
	for _, it := range lit.Items {
		o, ok := c.info.Defs[it.(*syntax.EntryItem).Key].(*object)
		if !ok || o == nil {
			return nil, false
		}
		out = append(out, o)
	}
	return out, true
}

// sharedNodes are the nodes both versions of sw hold: the header parts and declarations a new
// parse shared (project.Reuse).
func sharedNodes(sw swap) map[syntax.Node]bool {
	out := map[syntax.Node]bool{}
	of, nf := sw.old, sw.new
	if of.Doc != nil && of.Doc == nf.Doc {
		out[of.Doc] = true
	}
	if of.Package == nf.Package {
		out[of.Package] = true
	}
	for i, imp := range of.Imports {
		if imp == nf.Imports[i] {
			out[imp] = true
		}
	}
	for _, d := range sw.pairs {
		if d.shared() {
			out[d.old] = true
		}
	}
	return out
}

// pairSignature pairs the nodes of a let's two versions outside its value and doc comment, in
// walk order, into sig; false when their shapes differ.
func pairSignature(sig map[syntax.Node]syntax.Node, a, b *syntax.LetDecl) bool {
	sig[a] = b
	if (a.Mods == nil) != (b.Mods == nil) || len(a.Annotations) != len(b.Annotations) {
		return false
	}
	if a.Mods != nil && !pairNodes(sig, a.Mods, b.Mods) {
		return false
	}
	if !pairNodes(sig, a.Name, b.Name) || !pairNodes(sig, a.Type, b.Type) {
		return false
	}
	for i, an := range a.Annotations {
		if !pairNodes(sig, an, b.Annotations[i]) {
			return false
		}
	}
	return true
}

// pairNodes pairs every node under a with the one at its place under b; false when the two
// shapes differ.
func pairNodes(sig map[syntax.Node]syntax.Node, a, b syntax.Node) bool {
	na, nb := walkedNodes(a), walkedNodes(b)
	if len(na) != len(nb) {
		return false
	}
	for i, x := range na {
		if x.Kind() != nb[i].Kind() {
			return false
		}
		sig[x] = nb[i]
	}
	return true
}

// walkedNodes is every node under n, n first, in walk order.
func walkedNodes(n syntax.Node) []syntax.Node {
	var out []syntax.Node
	syntax.Inspect(n, func(x syntax.Node) bool {
		if x != nil {
			out = append(out, x)
		}
		return true
	})
	return out
}

// syntaxCode reports a code the lexer or the parser owns (DECISIONS 214).
func syntaxCode(code diag.Code) bool {
	i := slices.IndexFunc(diag.Registry, func(d diag.Def) bool { return d.Code == code })
	return i >= 0 && diag.Registry[i].Package == syntaxOwner
}

// letKeyed is the key field of a keyed list let, nil for any other let.
func letKeyed(o *object) *types.Field {
	_, keyed, _ := collectionElem(o.typ)
	return keyed
}
