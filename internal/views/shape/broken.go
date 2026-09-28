package shape

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// Errors are the spans of the errors among findings that the views do not report themselves:
// the ones that make a view broken (TYPES.md 1, VIEWMODEL.md J4).
func Errors(findings []diag.Finding) []source.Span {
	var out []source.Span
	for _, f := range findings {
		if f.Severity == diag.Error && !viewsCode(f.Code) {
			out = append(out, f.Span)
		}
	}
	return out
}

// viewsCode reports a code the views report (ERRORS.md, Package column).
func viewsCode(code diag.Code) bool {
	for i := range diag.Registry {
		if diag.Registry[i].Code == code {
			return diag.Registry[i].Package == viewsPackage
		}
	}
	return false
}

// ViewBroken reports a view left out of the model and of the view checks (TYPES.md 1,
// VIEWMODEL.md J4): its target is broken, or it holds one of errs, a recovery node or an
// expression of the error type.
func ViewBroken(info *check.Info, f *syntax.File, d *syntax.ViewDecl, errs []source.Span) bool {
	if o := info.NameUses[d.Type]; o != nil && info.Broken[o] {
		return true
	}
	within := f.Span(d)
	if slices.ContainsFunc(errs, func(s source.Span) bool {
		return s.File == within.File && s.Start >= within.Start && s.Start < within.End
	}) {
		return true
	}
	bad := false
	syntax.Inspect(d, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.BadExpr, *syntax.BadDecl:
			bad = true
		}
		if e, ok := n.(syntax.Expr); ok && IsError(info.Types[e]) {
			bad = true
		}
		return !bad
	})
	return bad
}

// IsError reports the error type (TYPES.md 1).
func IsError(t types.Type) bool { return t != nil && t.Kind() == types.Error }
