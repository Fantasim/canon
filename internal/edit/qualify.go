package edit

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// naming is how text names the type it qualifies a member or case through: as its .canon file
// does, by package and imports. The zero naming, an op's Source, names a type alone; a file
// without imports (a layer, a new entry file) has a package and no file.
type naming struct {
	pkg  string
	file *syntax.File
}

// typeRef is a named type a qualified name goes through: an enum or a variant.
type typeRef struct {
	pkg, name string
}

// fileNaming is the naming of the source file f.
func fileNaming(f *syntax.File) naming {
	if f == nil || f.Package == nil {
		return naming{}
	}
	return naming{pkg: syntax.Qualified(f.Package), file: f}
}

// bareNaming is the naming of a layer or new entry file: the root's package, no imports.
func (x *opCtx) bareNaming() naming {
	return naming{pkg: x.res.root.pkg.Path}
}

// valueWord reports a name read bare in value position: an IDENT or a nameable word (LEX-08).
func valueWord(name string) bool {
	k := syntax.LookupWord(name)
	return isWord(name) && (k == syntax.TokIdent || syntax.IsNameable(k))
}

// nameText is a member, or a dependent symbol the edit pinned or a bare name holds (pinned), as
// value position writes it; a symbol no bare name holds and no branch types has none (DECISIONS 366).
func (p *canonPrinter) nameText(v value.Value) string {
	switch x := v.(type) {
	case *value.Member:
		return p.in.qualified(typeRef{x.Enum.Pkg, x.Enum.Name}, x.CanonText())
	case *value.Symbol:
		if !valueWord(x.Name) && p.err == nil {
			p.err = &ValueError{Expected: x.T.String(), Got: x.Name, Detail: detailNoBranch}
		}
	}
	return v.CanonText()
}

// caseText is case c's name as value position writes it, alone or before its fields.
func (n naming) caseText(c *types.CaseType) string {
	if c.Variant == nil {
		return c.Name
	}
	return n.qualified(typeRef{c.Variant.Pkg, c.Variant.Name}, c.Name)
}

// qualified is name, of type t, bare where the grammar reads it, else `T.name` (LEX-08).
func (n naming) qualified(t typeRef, name string) string {
	if valueWord(name) {
		return name
	}
	return n.typeName(t) + dotSeg + name
}

// typeName is t as n's file names it: alone in its package or imported by name, else through the
// name its import binds; with no import, through the last segment, for the re-check's E2102.
func (n naming) typeName(t typeRef) string {
	if n.pkg == "" || t.pkg == "" || t.pkg == n.pkg {
		return t.name
	}
	if n.file != nil {
		for _, imp := range n.file.Imports {
			if name, ok := importedName(imp, t); ok {
				return name
			}
		}
	}
	return lastSegment(t.pkg) + dotSeg + t.name
}

// importedName is t as imp makes it nameable, false when imp does not.
func importedName(imp *syntax.Import, t typeRef) (string, bool) {
	if imp.Path == nil || len(imp.Path.Parts) == 0 || syntax.Qualified(imp.Path) != t.pkg {
		return "", false
	}
	if len(imp.Names) > 0 {
		return t.name, slices.ContainsFunc(imp.Names, func(id *syntax.Ident) bool { return id.Name == t.name })
	}
	bound := imp.Path.Parts[len(imp.Path.Parts)-1].Name
	if imp.Alias != nil {
		bound = imp.Alias.Name
	}
	return bound + dotSeg + t.name, true
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

// pinSymbols is v, of type t, typed for the value at the path, with each symbol a .canon text must
// qualify resolved in the branch its record's discriminant selects (DECISIONS 366).
func (x *opCtx) pinSymbols(v value.Value, t types.Type) (value.Value, error) {
	fr := x.frameAt(len(x.res.Steps))
	fr.kept, fr.pin = nil, true
	return fr.symbolsIn(v, nil, t)
}

// pinned is s, of type t, resolved when a Source qualified it or no bare name holds it: what it
// names in the branch fr computes, whose type its qualifier must name; else s as given. Where no
// branch can be computed, one no bare name holds is a *ValueError (DECISIONS 366).
func (fr depFrame) pinned(s *value.Symbol, t types.Type) (value.Value, error) {
	qual, named := fr.a.marks.qualifier(s)
	if s.P != nil || !named && (valueWord(s.Name) || !isWord(s.Name)) {
		return s, nil // data a JSON source wrote, or a name a .canon text holds as given
	}
	et, ok := fr.branchFor(t)
	switch {
	case !ok && valueWord(s.Name):
		return s, nil
	case !ok:
		return nil, &ValueError{Expected: t.String(), Got: s.Name, Detail: detailNoBranch}
	}
	if tr, isNamed := expectedType(et); named && (!isNamed || !namesType(qual, tr)) {
		return nil, &ValueError{Expected: et.String(), Got: strings.Join(qual, dotSeg) + dotSeg + s.Name, Detail: detailOtherType}
	}
	return fr.symbolIn(s, t)
}
