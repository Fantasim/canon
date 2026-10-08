package edit

import (
	"strings"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// spellings maps each scalar to the token it is printed as, when not canonical (DECISIONS 337).
type spellings map[value.Value]string

// stated is the .canon literal stating a value an Undo restores, with a root table's entry
// declarations; zero when no literal states it.
type stated struct {
	node    syntax.Node
	file    *syntax.File
	entries []item
}

// statedBy is the literal cursor c stands in, zero when c is none.
func statedBy(c cursor) stated {
	if c.state != stTree || c.mode != ModeCanon || c.file == nil {
		return stated{}
	}
	return stated{node: c.node, file: c.file, entries: c.entries}
}

// field is the literal s writes its field name with, zero when it writes none.
func (s stated) field(name string) stated {
	if s.node == nil {
		return stated{}
	}
	if fi := fieldItem(s.node, name); fi != nil {
		return stated{node: syntax.Unparen(fi.Value), file: s.file}
	}
	return stated{}
}

// spellable reports a number or a string: a Duration is printed in its canonical text (API.md
// M7), a bool or a member has one spelling.
func spellable(v value.Value) bool {
	switch v.(type) {
	case *value.Int, *value.Float, *value.Str:
		return true
	}
	return false
}

// spellCanon is the canonical text of v, a spellable value.
func spellCanon(v value.Value) string {
	if s, ok := v.(*value.Str); ok {
		return canonQuote(s.V)
	}
	return v.CanonText()
}

// spellToken reports e one number or string token on one line, which a Source may carry
// (API.md E26): no name, no interpolation, no multiline string.
func spellToken(e syntax.Expr, text string) bool {
	if strings.Contains(text, newline) {
		return false
	}
	switch x := e.(type) {
	case *syntax.IntLit, *syntax.FloatLit, *syntax.RawStringLit:
		return true
	case *syntax.StringLit:
		_, ok := constString(x)
		return ok
	}
	return false
}

// token is the token e typed as t by typed, its spelling kept for the write.
func (st *srcTyping) token(e syntax.Expr, t types.Type, typed func(syntax.Expr, types.Type) (value.Value, error)) (value.Value, error) {
	v, err := typed(e, t)
	if err == nil {
		st.spell(e, v)
	}
	return v, err
}

// spell records v, which the token e typed, with e's text when that is not v's canonical text:
// the write prints the token as the operation spelled it.
func (st *srcTyping) spell(e syntax.Expr, v value.Value) {
	if st.ty.spelled == nil || !spellable(v) {
		return
	}
	sp := st.file.Span(e)
	if text := string(st.file.Src.Content[sp.Start:sp.End]); text != spellCanon(v) && spellToken(e, text) {
		st.ty.spelled[v] = text
	}
}

// noteSpellings are the numbers and strings of v that at writes as tokens at v's own place,
// directly or as items or keys of its composite literals, as written: notes of this restore only,
// so a value a name shares with another literal never borrows its spelling (DECISIONS 337).
func (a *applier) noteSpellings(v value.Value, at stated) spellings {
	if at.node == nil {
		return nil
	}
	n := noter{a: a, notes: spellings{}}
	n.value(v, item{at.node, at.file}, at.entries)
	if len(n.notes) > 0 {
		n.unshared(v)
	}
	return n.notes
}

// noter collects one restore's spellings.
type noter struct {
	a     *applier
	notes spellings
}

// value notes v's tokens, it the item stating v and more a root table's entry declarations.
func (n noter) value(v value.Value, it item, more []item) {
	if spellable(v) {
		n.token(v, it)
		return
	}
	kids := childValues(v)
	if len(kids) == 0 {
		return
	}
	n.matched(kids, append(items(it.node, it.file), more...), n.value)
	if m, ok := v.(*value.Map); ok {
		n.matched(m.Keys, keyItems(it.node, it.file), func(k value.Value, c item, _ []item) { n.token(k, c) })
	}
}

// matched calls do with each of vs that one of its holder's items states, and that item.
func (n noter) matched(vs []value.Value, its []item, do func(value.Value, item, []item)) {
	bySpan := make(map[source.Span]item, len(its))
	for _, c := range its {
		bySpan[c.file.Span(c.node)] = c
	}
	for _, v := range vs {
		if p := provOf(v); p != nil {
			if c, ok := bySpan[p.Span]; ok {
				do(v, c, nil)
			}
		}
	}
}

// keyItems are the key expressions of a map literal's entries.
func keyItems(node syntax.Node, f *syntax.File) []item {
	lit := braceOf(node)
	if lit == nil {
		return nil
	}
	var out []item
	for _, it := range lit.Items {
		if m, ok := it.(*syntax.MapItem); ok {
			out = append(out, item{syntax.Unparen(m.Key), f})
		}
	}
	return out
}

// token notes v when the token it states is it, reads back as v, and is not canonical: a value
// an index, a branch or a name brings from another literal is not.
func (n noter) token(v value.Value, it item) {
	p := provOf(v)
	e, isExpr := it.node.(syntax.Expr)
	if p == nil || !isExpr || p.Kind != value.ProvLiteral && p.Kind != value.ProvLayer || p.Span != it.file.Span(e) {
		return
	}
	text := string(it.file.Src.Content[p.Span.Start:p.Span.End])
	if text == spellCanon(v) || !spellToken(e, text) {
		return
	}
	st := &srcTyping{typing: &typing{ctx: n.a.ctx}, file: it.file}
	if back, err := st.expr(e, v.Type()); err == nil && sameValue(back, v) {
		n.notes[v] = text
	}
}

// unshared drops each note whose value v holds more than once: noted at its own token, it is
// also somewhere a name or an expression shares it, which is printed canonically.
func (n noter) unshared(v value.Value) {
	seen := map[value.Value]int{}
	var count func(value.Value)
	count = func(x value.Value) {
		if _, ok := n.notes[x]; ok {
			seen[x]++
		}
		if m, ok := x.(*value.Map); ok {
			for _, k := range m.Keys {
				count(k)
			}
		}
		for _, c := range childValues(x) {
			count(c)
		}
	}
	count(v)
	for x, c := range seen { //canon:unordered deletes from a set
		if c > 1 {
			delete(n.notes, x)
		}
	}
}

// childValues are the values v holds that a literal writes as items: fields, elements, map
// values and table entries.
func childValues(v value.Value) []value.Value {
	switch x := v.(type) {
	case *value.Record:
		return x.Fields
	case *value.List:
		return x.Elems
	case *value.Map:
		return x.Vals
	case *value.Table:
		out := make([]value.Value, len(x.Entries))
		for i, e := range x.Entries {
			out[i] = e
		}
		return out
	}
	return nil
}
