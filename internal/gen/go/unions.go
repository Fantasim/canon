package gogen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// unionExpr is a literal union's value as its string holds it: a literal, or its ref's key as the wire writes it (CODEGEN.md §4.1; DECISIONS 387).
func (g *gen) unionExpr(t ir.TypeRef, v value.Value) string {
	if r, ok := v.(*value.Ref); ok {
		if arm := ir.UnionRef(t); arm != nil {
			return strconv.Quote(ir.KeyWire(*arm, r.Key))
		}
	}
	return strconv.Quote(as[value.Str](g, v).V)
}

// literalGuard is the condition that s is none of union t's literals: a literal is accepted as it is (DECISIONS 387).
func literalGuard(t ir.TypeRef, s string) string {
	conds := make([]string, len(t.Literals))
	for i, w := range t.Literals {
		conds[i] = s + differs + strconv.Quote(w)
	}
	return strings.Join(conds, andSep)
}

// unionIDCheck refuses, in a data loader, a union's string that is none of its literals and no id of its ref arm's table, when that package's go emit gives ids an enum its reader parses; the ref check's own text: `unknown value`, `no entry` in another package's class (CODEGEN.md §5.3, §5.8, §2.8; DECISIONS 387). A types-mode decoder checks no key (§5.13).
func (g *gen) unionIDCheck(b *strings.Builder, l leaf, r ir.TypeRef, s string, loc location) {
	if g.isTypes() || !isTableRef(r.Ref) || !g.enumIDs(r.Ref.Pkg) {
		return
	}
	text := unknownValueText
	if l.owned || g.holder() != g.p.Name {
		text = noEntryText
	}
	parse := g.qualify(r.Ref.Pkg, g.names.ParseName(g.idType(r.Ref.Elem)))
	fmt.Fprintf(b, ifOpenFormat, literalGuard(l.t, s))
	fmt.Fprintf(b, keyCheckFormat, g.temp(tempOK), parse, s, g.errAt(loc, text, s))
	b.WriteString(closeBrace)
}

// unionCheck refuses a union's string s that is none of its literals and that find, the Find of the table or keyed list its ref arm resolves into, finds no entry for, failing with fail (CODEGEN.md §5.8; DECISIONS 387); an enum key is read with the enum's Parse first.
func (g *gen) unionCheck(b *strings.Builder, t ir.TypeRef, find, s, fail string) {
	r := ir.UnionRef(t)
	if r == nil {
		return
	}
	found := g.temp(tempOK)
	fmt.Fprintf(b, ifOpenFormat, literalGuard(t, s))
	if e, ok := ir.RefKeyEnum(*r); ok && !ir.OpenEnum(g.p, g.e, e) {
		k := g.temp(tempKey)
		fmt.Fprintf(b, parsedKeyFormat, k, found, g.qualify(e.Pkg, g.names.ParseName(g.goName(e))), s)
		fmt.Fprintf(b, findAgainFormat, found, find, k)
	} else {
		fmt.Fprintf(b, findKeyFormat, found, find, g.keyOfText(*r, s))
	}
	fmt.Fprintf(b, failUnlessFormat, found, fail)
	b.WriteString(closeBrace)
}

// keyOfText is string s as the key type of ref r, whose table's rows are found by it: its id type, a string, or an opened enum.
func (g *gen) keyOfText(r ir.TypeRef, s string) string {
	if typ := g.keyType(r); typ != goString {
		return typ + lparen + s + rparen
	}
	return s
}

// walkUnion checks a union value at loc where its ref arm resolves in the class being walked; nothing elsewhere, the key read unchecked as a plain ref's.
func (g *gen) walkUnion(b *strings.Builder, t ir.TypeRef, expr string, loc location) {
	if target := g.names.UnionTarget(t, g.walkClass); target != nil {
		g.unionCheck(b, t, g.findIn(target, g.walkSnap), expr, g.errAt(loc, noEntryText, expr))
	}
}

// ownerFind is the Find of the owner's keyed list ref r targets, the owner's accessor called (CODEGEN.md §5.9).
func (g *gen) ownerFind(r ir.TypeRef) string {
	return g.qualify(r.Ref.Pkg, g.names.OwnerAccessor(r.Ref)) + callSuffix + dot + ir.GoFind
}

// ownerChecked reports a ref a reader of another package's class checks against the owner's keyed list, in a position no getter its hook fills reaches: not one of a slot its owner resolves (l.owned, checked by ownerChecks), nor in a class of this package (CODEGEN.md §2.8 Refs).
func (g *gen) ownerChecked(l leaf, r ir.TypeRef) bool {
	return !l.owned && g.holder() != g.p.Name && g.names.OwnerChecks(r, g.holder())
}

// foreignKeyCheck refuses, in a reader of another package's class, a key x of ref r naming no entry of the owner's keyed list its hook resolves r into, in a map key or value, a nested list or an optional element, with the loader text `no entry <key>` at loc, in any mode (CODEGEN.md §2.8, §5.8).
func (g *gen) foreignKeyCheck(b *strings.Builder, l leaf, r ir.TypeRef, x string, loc location) {
	if g.ownerChecked(l, r) {
		fmt.Fprintf(b, keyCheckFormat, g.temp(tempOK), g.ownerFind(r), x, g.errAt(loc, noEntryText, x))
	}
}

// unionOwnerCheck refuses, in a data loader's reader of another package's class, a union's string that is no literal and names no entry of the owner's keyed list its ref arm targets, as that ref's own check does; a types-mode decoder checks no union key (CODEGEN.md §2.8, §5.13; DECISIONS 387).
func (g *gen) unionOwnerCheck(b *strings.Builder, l leaf, r ir.TypeRef, s string, loc location) {
	if !g.isTypes() && g.ownerChecked(l, r) {
		g.unionCheck(b, l.t, g.ownerFind(r), s, g.errAt(loc, noEntryText, s))
	}
}

// walkChecked checks the unions and optional refs of a field's, stored result's or lookup cell's type t exactly where a plain ref is checked (CODEGEN.md §5.8; DECISIONS 385, 387): t itself, or the elements of a list one level deep; never under a map value or in a nested list, where a plain ref is a key only.
func (g *gen) walkChecked(b *strings.Builder, t ir.TypeRef, expr string, loc location) {
	if t.Kind == types.LitUnion {
		g.walkUnion(b, t, expr, loc)
		return
	}
	if t.Kind != types.List || !g.names.LeafChecked(t, g.walkClass) {
		return
	}
	i := g.temp(tempIndex)
	fmt.Fprintf(b, walkListFormat, i, expr)
	g.walkLeaf(b, g.sub(t.Elem), expr+atCall+i+rparen, loc.index(i))
	b.WriteString(closeBrace)
}

// walkLeaf checks the element e of a list at loc: a union, or a present optional union or ref.
func (g *gen) walkLeaf(b *strings.Builder, e ir.TypeRef, expr string, loc location) {
	if e.Kind != types.Optional {
		g.walkUnion(b, e, expr, loc)
		return
	}
	inner := g.sub(e.Elem)
	fmt.Fprintf(b, ifNotNilFormat, expr)
	if inner.Kind == types.Ref {
		g.walkOptionalRef(b, inner, expr, loc)
	} else {
		g.walkUnion(b, inner, fmt.Sprintf(derefFormat, expr), loc)
	}
	b.WriteString(closeBrace)
}

// walkOptionalRef refuses a present optional ref element, expr non-nil, that names no entry of the table its ref resolves into in the class being walked (CODEGEN.md §5.8; DECISIONS 385).
func (g *gen) walkOptionalRef(b *strings.Builder, r ir.TypeRef, expr string, loc location) {
	target := g.names.ElemRefTarget(r, g.walkClass)
	if target == nil {
		return
	}
	key := pointer + expr
	fmt.Fprintf(b, keyCheckFormat, g.temp(tempOK), g.findIn(target, g.walkSnap), key, g.errAt(loc, noEntryText, key))
}
