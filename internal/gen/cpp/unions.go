package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// unionTarget is the value a data loader checks the keys of union t against in class c: where a plain ref of its ref arm resolves at load (CODEGEN.md §5.8); nil elsewhere, the key read unchecked as that ref's.
func (g *gen) unionTarget(t ir.TypeRef, c class) *ir.Value {
	if r := ir.UnionRef(t); r != nil {
		return g.resolvedTarget(*r, c)
	}
	return nil
}

// leafTarget is the value a key of t, a union or a ref no resolved getter holds, is checked against in class c; nil for none.
func (g *gen) leafTarget(t ir.TypeRef, c class) *ir.Value {
	if t.Kind == types.Ref {
		return g.resolvedTarget(t, c)
	}
	return g.unionTarget(t, c)
}

// elemsChecked reports a field or stored result of class c whose keys the loader checks where no slot resolves them: a union, a list of unions, or a list of optional unions or refs, each present element checked (CODEGEN.md §5.8; DECISIONS 387, log-2026-10-20).
func (g *gen) elemsChecked(t ir.TypeRef, c class) bool {
	if t.Kind == types.List && t.Elem != nil {
		t = *t.Elem
	}
	if t.Kind == types.Optional && t.Elem != nil {
		return g.leafTarget(*t.Elem, c) != nil
	}
	return g.unionTarget(t, c) != nil
}

// differs are the conditions that k is none of wires.
func differs(k string, wires []string) []string {
	out := make([]string, len(wires))
	for i, w := range wires {
		out[i] = fmt.Sprintf(differsFormat, k, quote(w))
	}
	return out
}

// literalGuard is the condition, then `&&`, that k is none of union t's literals: a literal is accepted as it is (DECISIONS 387).
func literalGuard(t ir.TypeRef, k string) string {
	return strings.Join(differs(k, t.Literals), andSep) + andSep
}

// missing is the condition that key k of t, a ref's key or a union's text, names no entry find looks it up in: a union's literal is accepted as it is, and its text keyed by an enum is read with the enum's FromWire first, which init declares (DECISIONS 387).
func (g *gen) missing(t ir.TypeRef, find, k string) (init, cond string) {
	r := ir.UnionRef(t)
	if r == nil {
		return "", find + k + nullCompare
	}
	guard := literalGuard(t, k)
	if e, ok := ir.RefKeyEnum(*r); ok {
		return fmt.Sprintf(enumKeyInitFormat, g.enumHelpers(e).FromWire, k), guard + fmt.Sprintf(enumKeyMissingFormat, find)
	}
	return "", guard + find + k + nullCompare
}

// leafText is the format turning a key of t into the text of a load error.
func (g *gen) leafText(t ir.TypeRef) string {
	if t.Kind == types.Ref {
		return g.keyText(t.Key)
	}
	return stringOfFormat
}

// leafCheck refuses, in a Resolve, a key k of t at key naming no entry of target: the plain ref's check and text (CODEGEN.md §5.8, DECISIONS 312, 387).
func (g *gen) leafCheck(depth int, t ir.TypeRef, target *ir.Value, k, key string) {
	init, cond := g.missing(t, g.findFor(target), k)
	g.c.linef(depth, checkReturnFormat, init, cond, key, fmt.Sprintf(g.leafText(t), k))
}

// foreignRefCheck refuses, in a reader of another package's class, a key k of ref r naming no entry of the owner's container its hook resolves r into, where no hook member's check reaches it (a map key or value, a nested list, an optional element, a union): CODEGEN.md §2.8's every position.
func (g *gen) foreignRefCheck(depth int, t, r ir.TypeRef, k, key string) {
	if g.view == "" || g.view == g.p.Name || !g.pl.OwnerChecks(r, g.view) {
		return
	}
	find := g.qualifier(r.Ref.Pkg) + g.ownerAccessor(r) + findCallPrefix
	init, cond := g.missing(t, find, k)
	g.c.linef(depth, checkFailFormat, init, cond, key, fmt.Sprintf(g.leafText(t), k))
}

// foreignUnionCheck refuses, in a data loader, a union key that is none of its literals and names no entry of its ref arm's target, as that ref's own check does: FromWire for a table keyed by an id enum, `unknown value <key>` in a class of this package, as Go's own loader says, else in another package's class the owner's container (CODEGEN.md §2.8, §5.3, §5.8; DECISIONS 312, 387); a types-mode decoder checks no key (§5.13).
func (g *gen) foreignUnionCheck(depth int, t ir.TypeRef, k, key string) {
	r := ir.UnionRef(t)
	if r == nil || g.types() {
		return
	}
	if id, ok := g.idEnum(*r); ok {
		format := unionEntryFormat
		if g.ownClass() {
			format = unionOwnEntryFormat
		}
		g.c.linef(depth, format, literalGuard(t, k), g.pl.EnumHelpers(id).FromWire, k, key, k)
		return
	}
	g.foreignRefCheck(depth, t, *r, k, key)
}
