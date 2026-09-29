package edit

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// source types text, parsed as a `let` value, as a literal with contextual names only (SPEC §6.2).
func (tc *typing) source(text Source, t types.Type) (value.Value, error) {
	e, f := parseLiteral(string(text))
	if e == nil {
		return nil, tc.mismatch(text, t, detailNotLiteral)
	}
	st := &srcTyping{typing: tc, file: f}
	return st.expr(e, t)
}

// parseLiteral is text's expression, nil when text is not exactly one expression.
func parseLiteral(text string) (syntax.Expr, *syntax.File) {
	var fs source.FileSet
	src, err := fs.Add(sourceName, sourceName, []byte(sourcePrefix+text))
	if err != nil {
		return nil, nil
	}
	bag := diag.NewBag(&fs, "")
	f := syntax.Parse(src, syntax.FileSource, bag)
	if bag.Summary().Errors > 0 || len(f.Decls) != 1 {
		return nil, nil
	}
	d, ok := f.Decls[0].(*syntax.LetDecl)
	if !ok || d.Value == nil {
		return nil, nil
	}
	return d.Value, f
}

// srcTyping types the expression of a Source.
type srcTyping struct {
	*typing
	file *syntax.File
}

// expr is e as a value of t; a T is accepted where a T? is expected (SPEC §5.6).
func (st *srcTyping) expr(e syntax.Expr, t types.Type) (value.Value, error) {
	if len(st.at) > maxLitDepth {
		return nil, st.wrong(e, t, detailDeep)
	}
	e = syntax.Unparen(e)
	if _, ok := e.(*syntax.NoneLit); ok {
		if isOptional(t) {
			return &value.None{T: t}, nil
		}
		return nil, st.wrong(e, t, "")
	}
	et := present(t)
	if _, isName := e.(*syntax.IdentExpr); dependent(et) && !isName {
		return nil, st.wrong(e, et, detailDependent)
	}
	switch e.(type) {
	case *syntax.IntLit:
		return st.intLit(e, et)
	case *syntax.FloatLit:
		return st.floatLit(e, et)
	case *syntax.DurationLit:
		return st.durLit(e, et)
	case *syntax.StringLit, *syntax.RawStringLit:
		return st.strLit(e, et)
	case *syntax.BoolLit:
		return st.boolLit(e, et)
	case *syntax.IdentExpr:
		return st.name(e, et)
	case *syntax.ListLit:
		return st.listLit(e, et)
	case *syntax.BraceLit:
		return st.brace(e, et)
	case *syntax.TypedLit:
		return st.typedLit(e, et)
	}
	return nil, st.wrong(e, t, detailNotLiteral)
}

// wrong is the refusal of e where t is expected: what was given is e's text.
func (st *srcTyping) wrong(e syntax.Node, t types.Type, detail string) error {
	sp := st.file.Span(e)
	return st.refuse(t, string(st.file.Src.Content[sp.Start:sp.End]), detail)
}

// fits is v, or the refusal of e when it does not fit t.
func (st *srcTyping) fits(e syntax.Expr, t types.Type, v value.Value, ok bool) (value.Value, error) {
	if !ok {
		return nil, st.wrong(e, t, "")
	}
	return v, nil
}

// intLit is an integer, or the key of a ref into an integer-keyed collection (TYPES.md §4.1).
func (st *srcTyping) intLit(e syntax.Expr, t types.Type) (value.Value, error) {
	x := e.(*syntax.IntLit)
	if !x.Value.IsInt64() {
		return nil, st.wrong(e, t, detailRange)
	}
	if v, ok := intValue(x.Value.Int64(), t); ok {
		return v, nil
	}
	v, ok := refValue(t, func(kt types.Type) value.Value { return intKeyValue(x.Value.Int64(), kt) })
	return st.fits(e, t, v, ok)
}

// floatLit is a float, its exact decimal value rounded once (TYP-11).
func (st *srcTyping) floatLit(e syntax.Expr, t types.Type) (value.Value, error) {
	x := e.(*syntax.FloatLit)
	text := x.Coef.String() + exponentMark + strconv.FormatInt(x.Exp, decimalBase)
	if x.Neg {
		text = string(minus) + text
	}
	f, _ := strconv.ParseFloat(text, int64Bits) // out of range is ±Inf, which floatValue refuses
	v, ok := floatValue(f, t)
	return st.fits(e, t, v, ok)
}

func (st *srcTyping) durLit(e syntax.Expr, t types.Type) (value.Value, error) {
	v := &value.Dur{Ms: e.(*syntax.DurationLit).Millis}
	return st.fits(e, t, v, t.Base().Kind() == types.Duration)
}

func (st *srcTyping) boolLit(e syntax.Expr, t types.Type) (value.Value, error) {
	v := &value.Bool{V: e.(*syntax.BoolLit).Value}
	return st.fits(e, t, v, t.Base().Kind() == types.Bool)
}

// strLit is a string without interpolation: a String, a union's literal, or a String key (TYPES.md §4.1).
func (st *srcTyping) strLit(e syntax.Expr, t types.Type) (value.Value, error) {
	s, ok := constString(e)
	if !ok {
		return nil, st.wrong(e, t, detailNotLiteral)
	}
	if v, ok := strValue(s, t); ok {
		return v, nil
	}
	v, ok := refValue(t, func(kt types.Type) value.Value { return textKeyValue(s, kt, false) })
	return st.fits(e, t, v, ok)
}

// constString is a string literal's value, false when it interpolates.
func constString(e syntax.Expr) (string, bool) {
	switch x := e.(type) {
	case *syntax.RawStringLit:
		return x.Value, true
	case *syntax.StringLit:
		var b strings.Builder
		for _, p := range x.Parts {
			if p.Interp != nil {
				return "", false
			}
			b.WriteString(p.Text)
		}
		return b.String(), true
	}
	return "", false
}

// name is a contextual name (SPEC §6.2): a member, a case, a ref's key, a dependent symbol.
func (st *srcTyping) name(e syntax.Expr, t types.Type) (value.Value, error) {
	id := e.(*syntax.IdentExpr)
	v, ok := nameValue(id.Name, t)
	return st.fits(e, t, v, ok)
}

// nameValue is the value a bare name has where t is expected, by step 1 of SPEC §6.2.
func nameValue(name string, t types.Type) (value.Value, bool) {
	if dependent(t) {
		return &value.Symbol{Name: name, T: t}, true
	}
	switch b := t.Base().(type) {
	case *types.EnumType, *types.RefType:
		return memberOrKey(name, t)
	case *types.LitUnionType:
		return nameValue(name, b.Of)
	case *types.VariantKindType:
		i := slices.IndexFunc(b.Variant.Cases, func(c *types.CaseType) bool { return c.Name == name })
		return &value.CaseKind{T: b, Index: i}, i >= 0
	case *types.VariantType:
		i := slices.IndexFunc(b.Cases, func(c *types.CaseType) bool { return c.Name == name })
		if i >= 0 {
			return bareCase(b.Cases[i]), true
		}
	case *types.CaseType:
		return bareCase(b), b.Name == name
	}
	return nil, false
}

// memberOrKey is an enum member, or a ref's key: a name of a String- or enum-keyed target.
func memberOrKey(name string, t types.Type) (value.Value, bool) {
	if v, ok := memberValue(name, t); ok {
		return v, true
	}
	return refValue(t, func(kt types.Type) value.Value { return textKeyValue(name, kt, false) })
}

// bareCase is a case written without fields; a required one is the re-check's E3302 (V3).
func bareCase(c *types.CaseType) *value.Record {
	return &value.Record{T: c, Fields: make([]value.Value, len(c.Fields)), Set: make([]bool, len(c.Fields))}
}
