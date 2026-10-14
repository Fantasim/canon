package edit

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// typeRef is a named type a qualified name goes through: an enum or a variant.
type typeRef struct {
	pkg, name string
}

// hasText reports a name with a value-position text, bare or quoted (GRAMMAR.md §2.3, §4.3).
func hasText(name string) bool {
	_, ok := syntax.ValueText(name)
	return ok
}

// nameText is a member, case kind, or dependent symbol the edit pinned or a name holds (pinned),
// bare or quoted, never qualified; a symbol with no such text is a *ValueError (API.md M7).
func (p *canonPrinter) nameText(v value.Value) string {
	if text, ok := syntax.ValueText(v.CanonText()); ok {
		return text
	}
	if s, isSymbol := v.(*value.Symbol); isSymbol && p.err == nil {
		p.err = &ValueError{Expected: s.T.String(), Got: s.Name, Detail: detailNoBranch}
	}
	return v.CanonText()
}

// caseText is case c's name as value position writes it, alone or before its fields (API.md M7).
func caseText(c *types.CaseType) string {
	if text, ok := syntax.ValueText(c.Name); ok {
		return text
	}
	return c.Name
}

// lastSegment is the last segment of a dotted package path: the name `import` binds.
func lastSegment(pkg string) string {
	return pkg[strings.LastIndexByte(pkg, fieldMark)+1:]
}

// selected is `T.n` or `p.T.n`, a member or case named through its type (LEX-08).
func (st *srcTyping) selected(e syntax.Expr, t types.Type) (value.Value, error) {
	words, ok := selectorWords(e)
	if !ok {
		return nil, st.wrong(e, t, detailNotLiteral)
	}
	return st.qualifiedName(e, words, t)
}

// qualifiedName is words, `T.n` or `p.T.n`, as member or case n of the type T that t expects;
// p is T's package, by its path or the last segment its import binds.
func (st *srcTyping) qualifiedName(e syntax.Expr, words []string, t types.Type) (value.Value, error) {
	tr, ok := expectedType(t)
	last := len(words) - 1
	switch {
	case !ok:
		return nil, st.wrong(e, t, detailNotLiteral)
	case !namesType(words[:last], tr):
		return nil, st.wrong(e, t, detailOtherType)
	}
	v, ok := nameValue(words[last], t)
	return st.fits(e, t, v, ok)
}

// branchSymbol is `T.n` or `p.T.n` given to dependent type t: the symbol n, as a bare n gives,
// noted qualified through T for pinned to check, when T is a branch's enum or variant (DECISIONS 366).
func (st *srcTyping) branchSymbol(e syntax.Expr, t types.Type) (value.Value, bool) {
	words, ok := selectorWords(e)
	if !ok {
		return nil, false
	}
	last := len(words) - 1
	for _, b := range branchesOf(t) {
		if tr, ok := expectedType(present(b)); ok && namesType(words[:last], tr) {
			s := &value.Symbol{Name: words[last], T: t}
			if st.ty.marks != nil {
				st.ty.marks.through[s] = words[:last]
			}
			return s, true
		}
	}
	return nil, false
}

// branchesOf are the types dependent type t computes (TYPES.md 11.2).
func branchesOf(t types.Type) []types.Type {
	switch b := t.Base().(type) {
	case *types.TypeAppType:
		return types.Branches(b.Fn)
	case *types.DepUnionType:
		return types.Branches(b.Fn)
	}
	return nil
}

// selectorWords are the words of `a.b.c`, a chain of plain selectors on a name; false for any
// other expression.
func selectorWords(e syntax.Expr) ([]string, bool) {
	var words []string
	for {
		s, ok := e.(*syntax.SelectorExpr)
		if !ok {
			break
		}
		if s.Optional || s.Name == nil {
			return nil, false
		}
		words = append(words, s.Name.Name)
		e = s.X
	}
	id, ok := e.(*syntax.IdentExpr)
	if !ok {
		return nil, false
	}
	words = append(words, id.Name)
	slices.Reverse(words)
	return words, true
}

// partNames are the words of a typed literal's qualified name.
func partNames(parts []*syntax.Ident) []string {
	words := make([]string, len(parts))
	for i, p := range parts {
		words[i] = p.Name
	}
	return words
}

// expectedType is the enum or variant whose member or case t takes: an enum, a literal union's
// base, a variant or its case; false for any other type, a ref included, which takes a key.
func expectedType(t types.Type) (typeRef, bool) {
	switch b := t.Base().(type) {
	case *types.EnumType:
		return typeRef{b.Pkg, b.Name}, true
	case *types.LitUnionType:
		return expectedType(b.Of)
	case *types.VariantType:
		return typeRef{b.Pkg, b.Name}, true
	case *types.CaseType:
		if b.Variant != nil {
			return typeRef{b.Variant.Pkg, b.Variant.Name}, true
		}
	}
	return typeRef{}, false
}

// namesType reports qual, the words before a member or case, naming t: `T`, or `p.T` with p t's
// package path or its last segment.
func namesType(qual []string, t typeRef) bool {
	last := len(qual) - 1
	if last < 0 || qual[last] != t.name {
		return false
	}
	pkg := strings.Join(qual[:last], dotSeg)
	return pkg == "" || pkg == t.pkg || pkg == lastSegment(t.pkg)
}

// pinSymbols is v, of type t, typed for the value at the path, with each symbol a Source qualified,
// or with no value text, resolved in the branch its record's discriminant selects (DECISIONS 378).
func (x *opCtx) pinSymbols(v value.Value, t types.Type) (value.Value, error) {
	fr := x.frameAt(len(x.res.Steps))
	fr.kept, fr.pin = nil, true
	return fr.symbolsIn(v, nil, t)
}

// pinned is s, of type t, resolved when a Source qualified it or it has no value text: what it
// names in the branch fr computes, whose type its qualifier must name; else s as given. Where no
// branch can be computed, it is kept when it has a text, else a *ValueError (DECISIONS 378).
func (fr depFrame) pinned(s *value.Symbol, t types.Type) (value.Value, error) {
	qual, named := fr.a.marks.qualifier(s)
	if s.P != nil || !named && (hasText(s.Name) || !isWord(s.Name)) {
		return s, nil // data a JSON source wrote, or a name a .canon text holds as given
	}
	et, ok := fr.branchFor(t)
	switch {
	case !ok && hasText(s.Name):
		return s, nil
	case !ok:
		return nil, &ValueError{Expected: t.String(), Got: s.Name, Detail: detailNoBranch}
	}
	if tr, isNamed := expectedType(et); named && (!isNamed || !namesType(qual, tr)) {
		return nil, &ValueError{Expected: et.String(), Got: strings.Join(qual, dotSeg) + dotSeg + s.Name, Detail: detailOtherType}
	}
	return fr.symbolIn(s, t)
}
