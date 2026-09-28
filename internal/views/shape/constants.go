package shape

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// The forms a source can take (API.md W1, §7.2 format).
const (
	formComputed Form = iota
	FormLiteral
	FormJSON
	FormFormat
)

// forms classifies the expressions that can state a source (API.md W1); any other is computed.
var forms = map[syntax.NodeKind]func(*check.Info, syntax.Expr) Form{
	syntax.KindIntLit:       alwaysLiteral,
	syntax.KindFloatLit:     alwaysLiteral,
	syntax.KindDurationLit:  alwaysLiteral,
	syntax.KindRawStringLit: alwaysLiteral,
	syntax.KindRegexLit:     alwaysLiteral,
	syntax.KindBoolLit:      alwaysLiteral,
	syntax.KindNoneLit:      alwaysLiteral,
	syntax.KindListLit:      alwaysLiteral,
	syntax.KindTypedLit:     alwaysLiteral,
	syntax.KindStringLit:    stringForm,
	syntax.KindBraceLit:     braceForm,
	syntax.KindIdentExpr:    identForm,
	syntax.KindSelectorExpr: selectorForm,
	syntax.KindLoadExpr:     loadForm,
}

// viewsPackage is the views in ERRORS.md's Package column: their own codes break no view.
const viewsPackage = "views"
