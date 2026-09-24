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
	if form == loadCSV && !hasHeader(e) {
		if !stringRows(want) {
			c.report(env, diag.E7116.At(env.span(e), loadName+dot+form, want))
			return types.ErrorType
		}
		return want
	}
	if form != loadForm && !collectionTarget(want) {
		c.report(env, diag.E7116.At(env.span(e), loadName+dot+form, want))
		return types.ErrorType
	}
	if want.Base().Kind() == types.Range || want.Base().Kind() == types.Func {
		c.report(env, diag.E7116.At(env.span(e), loadName, want))
		return types.ErrorType
	}
	return want
}

// loadText is `load.text(path)`: a String, or an alias or refinement of it (WIRE.md §6.1).
func (c *checker) loadText(env *env, e *syntax.LoadExpr, want types.Type) types.Type {
	if want == nil || want.Base().Kind() == types.Error {
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

// stringRows reports `[[String]]`, what load.csv builds without a header (WIRE.md §6.1).
func stringRows(t types.Type) bool {
	rows, ok := t.Base().(*types.ListType)
	if !ok || rows.KeyedBy != nil {
		return false
	}
	row, ok := rows.Elem.Base().(*types.ListType)
	return ok && row.KeyedBy == nil && types.Identical(row.Elem, types.StringType)
}

// collectionTarget reports what load.dir and load.csv (with a header) build: a list, a keyed
// list or a table.
func collectionTarget(t types.Type) bool {
	switch unwrap(t).Kind() {
	case types.List, types.Table:
		return true
	default:
		return false
	}
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
