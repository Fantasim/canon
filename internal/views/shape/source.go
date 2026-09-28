package shape

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// Form is how an expression states a value (API.md W1).
type Form uint8

// SourceForm is how e states a source (API.md W1); at a let it is VIEWMODEL.md §12.6's `editable`.
func SourceForm(info *check.Info, e syntax.Expr) Form {
	e = unparen(e)
	if e == nil {
		return formComputed
	}
	if f, ok := forms[e.Kind()]; ok {
		return f(info, e)
	}
	return formComputed
}

func alwaysLiteral(*check.Info, syntax.Expr) Form { return FormLiteral }

// stringForm is a string without interpolation; an interpolated one is computed.
func stringForm(_ *check.Info, e syntax.Expr) Form {
	for _, p := range e.(*syntax.StringLit).Parts {
		if p.Interp != nil {
			return formComputed
		}
	}
	return FormLiteral
}

// braceForm is a record, table or map literal; a comprehension is computed.
func braceForm(_ *check.Info, e syntax.Expr) Form {
	if len(e.(*syntax.BraceLit).Clauses) > 0 {
		return formComputed
	}
	return FormLiteral
}

// identForm is a member, a case or a key written as a bare name; any other name is computed.
func identForm(info *check.Info, e syntax.Expr) Form {
	x := e.(*syntax.IdentExpr)
	if literalName(info.Uses[x]) || info.Symbols[x] || info.Keys[x] != nil {
		return FormLiteral
	}
	return formComputed
}

// selectorForm is a qualified member or case (`Element.FIRE`), which has no selection.
func selectorForm(info *check.Info, e syntax.Expr) Form {
	x := e.(*syntax.SelectorExpr)
	if info.Selections[x] == nil && literalName(info.NameUses[x.Name]) {
		return FormLiteral
	}
	return formComputed
}

func literalName(obj check.Object) bool {
	if obj == nil {
		return false
	}
	switch obj.Kind() {
	case check.ObjMember, check.ObjCase, check.ObjEntry:
		return true
	default:
		return false
	}
}

// loadForm is the format row for load.csv, load.text and load.defines, JSON for the rest.
func loadForm(info *check.Info, e syntax.Expr) Form {
	x := e.(*syntax.LoadExpr)
	f := plainFormat(info, x)
	if x.Method != nil {
		f = types.FormatNamed(x.Method.Name)
	}
	if f == types.FormatCSV || f == types.FormatText || readsDefines(info, x) {
		return FormFormat
	}
	return FormJSON
}

// plainFormat is `load(path)`'s format: its `format:` symbol, else its path's extension (WIRE.md §6.2).
func plainFormat(info *check.Info, x *syntax.LoadExpr) types.LoadFormat {
	for _, a := range x.Args {
		if id, ok := a.Value.(*syntax.IdentExpr); ok && a.Name != nil && info.Symbols[id] {
			return types.FormatNamed(id.Name)
		}
	}
	if len(x.Args) == 0 || x.Args[0].Name != nil {
		return types.FormatUnknown
	}
	s, ok := x.Args[0].Value.(*syntax.StringLit)
	if !ok || len(s.Parts) != 1 {
		return types.FormatUnknown
	}
	return types.FormatOfPath(s.Parts[0].Text)
}

// readsDefines reports load.defines, which the checker types as a table of Define (WIRE.md §6.8).
func readsDefines(info *check.Info, x *syntax.LoadExpr) bool {
	t, ok := info.Types[x].(*types.TableType)
	return ok && t.Elem == types.DefineType
}

// unparen is e without its enclosing parentheses.
func unparen(e syntax.Expr) syntax.Expr {
	for {
		p, ok := e.(*syntax.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}
