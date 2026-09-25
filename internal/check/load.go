package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// load types a `load` form (WIRE.md §6.1).
func (c *checker) load(env *env, e *syntax.LoadExpr, want types.Type) types.Type {
	c.loadArgs(env, e)
	form := loadForm
	if e.Method != nil {
		form = e.Method.Name
	}
	switch form {
	case loadDefines:
		return &types.TableType{Elem: types.DefineType}
	case loadText:
		return c.loadText(env, e, want)
	}
	if want == nil {
		c.report(env, diag.E7002.At(env.span(e)))
		return types.ErrorType
	}
	if holds(want, types.Error) {
		return want
	}
	if !c.loadFits(e, form, want) {
		label := loadName
		if form != loadForm {
			label += dot + form
		}
		c.report(env, diag.E7116.At(env.span(e), label, want))
		return types.ErrorType
	}
	return want
}

// loadFits reports whether a load form can build want, else E7116 (WIRE.md §6.1, §6.2, §6.5, §6.6, §5.9).
func (c *checker) loadFits(e *syntax.LoadExpr, form string, want types.Type) bool {
	switch form {
	case loadCSV:
		return types.CSVFits(hasHeader(e), want)
	case loadForm:
		k := want.Base().Kind()
		return k != types.Range && k != types.Func && types.FormatFits(c.loadFormat(e, true), hasHeader(e), want)
	default:
		return dirFits(c.loadFormat(e, false), want)
	}
}

// loadFormat is the format `format:` names, else when byPath its path literal's extension's (WIRE.md §6.2).
func (c *checker) loadFormat(e *syntax.LoadExpr, byPath bool) types.LoadFormat {
	for _, a := range e.Args {
		if id, ok := a.Value.(*syntax.IdentExpr); ok && a.Name != nil && a.Name.Name == optionFormat {
			return types.FormatNamed(id.Name)
		}
	}
	if !byPath || len(e.Args) == 0 || e.Args[0].Name != nil {
		return types.FormatUnknown
	}
	s, ok := e.Args[0].Value.(syntax.StrLit)
	if !ok || !interpolationFree(e.Args[0].Value) || c.lexError(e.Args[0].Value) {
		return types.FormatUnknown
	}
	return types.FormatOfPath(constText(s))
}

// dirFits is a list, keyed list or table, never an optional, of what one file of format builds with no header (WIRE.md §6.5).
func dirFits(f types.LoadFormat, t types.Type) bool {
	switch b := t.Base().(type) {
	case *types.ListType:
		return types.FormatFits(f, false, b.Elem)
	case *types.TableType:
		return types.FormatFits(f, false, b.Elem)
	default:
		return false
	}
}

// loadText is `load.text(path)`: a String, or an alias or refinement of it (WIRE.md §6.1).
func (c *checker) loadText(env *env, e *syntax.LoadExpr, want types.Type) types.Type {
	if want == nil || holds(want, types.Error) {
		return types.StringType
	}
	if want.Base().Kind() != types.String {
		c.report(env, diag.E7116.At(env.span(e), loadName+dot+loadText, want))
		return types.ErrorType
	}
	return want
}

// hasHeader reports `header: true` among a load's options.
func hasHeader(e *syntax.LoadExpr) bool {
	return slices.ContainsFunc(e.Args, func(a *syntax.Arg) bool {
		b, ok := a.Value.(*syntax.BoolLit)
		return a.Name != nil && a.Name.Name == optionHeader && ok && b.Value
	})
}

// loadArgs types the path and options of a load as constants: strings, Bool flags, and the
// format symbol, which is never resolved in scope.
func (c *checker) loadArgs(env *env, e *syntax.LoadExpr) {
	ce := env.constant(diag.KindConstValue)
	for _, a := range e.Args {
		name := ""
		if a.Name != nil {
			name = a.Name.Name
		}
		switch name {
		case optionFormat:
			if id, ok := a.Value.(*syntax.IdentExpr); ok {
				c.info.Symbols[id] = true
				c.info.Types[id] = types.StringType
				continue
			}
			c.expr(ce, a.Value, types.StringType)
		case optionPartial, optionHeader:
			c.expr(ce, a.Value, types.BoolType)
		default:
			c.expr(ce, a.Value, types.StringType)
		}
	}
}
