package check

import (
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// amendPath is a path a layer sets, as written (messages) and by canonical segment (API.md §6.5).
type amendPath struct {
	text  string
	canon []string
}

// amendPaths are the paths a layer sets on each let, in source order.
type amendPaths map[*object][]amendPath

// overlap is E1908: a let's path set twice in a layer, or a prefix of another (EVALUATION.md §9.2).
func (c *checker) overlap(env *env, a *syntax.Amendment, let *object, p amendPath, seen amendPaths) {
	layer := env.owner.name
	for _, other := range seen[let] {
		n := min(len(p.canon), len(other.canon))
		switch {
		case !slices.Equal(p.canon[:n], other.canon[:n]):
			continue
		case len(p.canon) == len(other.canon):
			c.report(env, diag.E1908.AtTwice(env.span(a), p.text, layer))
		default:
			c.report(env, diag.E1908.AtOverlap(env.span(a), p.text, other.text, layer))
		}
		return
	}
	seen[let] = append(seen[let], p)
}

// canonSegment is seg on a t as API.md §6.5 writes it; a keyed position stays `[#n]`, its key unknown.
func (c *checker) canonSegment(env *env, t types.Type, seg *syntax.AmendSegment) string {
	switch {
	case seg.Position != nil && plainList(t):
		return openBracket + seg.Position.Value.String() + closeBracket
	case seg.Position != nil:
		return openBracket + hash + seg.Position.Value.String() + closeBracket
	case seg.Name != nil && !keyedBy(t):
		return dot + seg.Name.Name
	case seg.Name != nil:
		return openBracket + strconv.Quote(seg.Name.Name) + closeBracket
	}
	return openBracket + c.canonKey(env, t, seg.Key) + closeBracket
}

// plainList reports a list without keys, indexed by position.
func plainList(t types.Type) bool {
	l, ok := t.Base().(*types.ListType)
	return ok && l.KeyedBy == nil
}

// keyedBy reports a table or keyed list, whose `.k` names an entry.
func keyedBy(t types.Type) bool {
	_, _, ok := collectionElem(t)
	return ok
}

// canonKey is a typed key by its value: a word, member, entry or string by its text (a literal
// of a literal union apart), an integer in decimal; any other expression as written, equal only
// to itself.
func (c *checker) canonKey(env *env, t types.Type, k syntax.Expr) string {
	k = inner(k)
	if s, ok := k.(syntax.StrLit); ok && interpolationFree(k) && !c.lexError(k) {
		text := constText(s)
		if u, isUnion := unwrap(keyType(t)).(*types.LitUnionType); isUnion && slices.Contains(u.Literals, text) {
			return unionMark + strconv.Quote(text)
		}
		return strconv.Quote(text)
	}
	if n, ok := k.(*syntax.IntLit); ok && !c.lexError(k) {
		return n.Value.String()
	}
	if name, ok := c.keyName(k); ok {
		return strconv.Quote(name)
	}
	return openParen + env.written(k) + closeParen
}

// keyType is the key type of a map, else nil.
func keyType(t types.Type) types.Type {
	if m, ok := t.Base().(*types.MapType); ok {
		return m.Key
	}
	return nil
}

// keyName is the name a key written as a name stands for: a symbolic key, an enum member, a
// case or a static entry, bare or qualified.
func (c *checker) keyName(k syntax.Expr) (string, bool) {
	var name string
	var o Object
	switch x := k.(type) {
	case *syntax.IdentExpr:
		name, o = x.Name, c.info.Uses[x]
		if c.info.Symbols[x] {
			return name, true
		}
	case *syntax.SelectorExpr:
		name, o = x.Name.Name, c.info.NameUses[x.Name]
	default:
		return "", false
	}
	if c.info.Keys[k] != nil {
		return name, true
	}
	obj, ok := o.(*object)
	if !ok || obj == nil || obj.kind != ObjMember && obj.kind != ObjCase && obj.kind != ObjEntry {
		return "", false
	}
	return name, true
}
