package syntax

import (
	"github.com/fantasim/canonlang/internal/diag"
)

// valueOK checks an argument value of each kind; expectedKind is the word E1119 prints for it.
var (
	valueOK      [valKindCount]func(AnnValue) bool
	expectedKind [valKindCount]diag.Kind
)

func init() {
	valueOK[valString], expectedKind[valString] = isConstString, diag.KindConstantString
	valueOK[valTemplate], expectedKind[valTemplate] = isTemplate, diag.KindTemplateString
	valueOK[valInteger], expectedKind[valInteger] = isInteger, diag.KindInteger
	valueOK[valSince], expectedKind[valSince] = isPositive, diag.KindInteger
	valueOK[valLiteral], expectedKind[valLiteral] = isLiteral, diag.KindLiteral
	valueOK[valPairs], expectedKind[valPairs] = isPairs, diag.KindTemplateString
}

// isConstString is a string without interpolation.
func isConstString(v AnnValue) bool {
	switch v := v.(type) {
	case *RawStringLit:
		return true
	case *StringLit:
		return !hasInterp(v)
	}
	return false
}

// isTemplate is a string whose interpolations are names or "."-paths, with no format spec.
func isTemplate(v AnnValue) bool {
	return templateVars(v, func(string) bool { return true })
}

// templateVars reports a template whose every variable's root name passes ok; an interpolation holding a syntax error is not judged (DECISIONS 214).
func templateVars(v AnnValue, ok func(root string) bool) bool {
	switch v := v.(type) {
	case *RawStringLit:
		return true
	case *StringLit:
		for _, part := range v.Parts {
			if part.Interp == nil || holdsBad(part.Interp) {
				continue
			}
			root, path := pathRoot(part.Interp.X)
			if !path || part.Interp.Spec != nil || !ok(root) {
				return false
			}
		}
		return true
	}
	return false
}

// pathRoot is the root name of a name or a "."-path, and whether x is one.
func pathRoot(x Expr) (string, bool) {
	switch x := x.(type) {
	case *IdentExpr:
		return x.Name, true
	case *SelectorExpr:
		if x.X == nil || x.Optional {
			return "", false
		}
		return pathRoot(x.X)
	}
	return "", false
}

// holdsBad reports a node holding a BadExpr: a syntax error the lexer or parser has reported.
func holdsBad(n Node) bool {
	bad := false
	Inspect(n, func(c Node) bool {
		_, isBad := c.(*BadExpr)
		bad = bad || isBad
		return !bad
	})
	return bad
}

func isInteger(v AnnValue) bool {
	_, ok := v.(*IntLit)
	return ok
}

// isPositive is an integer of at least 1 (@since, GRAMMAR.md §8.3).
func isPositive(v AnnValue) bool {
	i, ok := v.(*IntLit)
	return ok && i.Value.Sign() > 0
}

// isLiteral is a string, a number, a duration, true, false, "{}" or "[]" (GRAMMAR.md §8.2).
func isLiteral(v AnnValue) bool {
	switch v := v.(type) {
	case *IntLit, *FloatLit, *DurationLit, *BoolLit:
		return true
	case *BraceLit:
		return len(v.Items) == 0
	case *AnnotationList:
		return len(v.Items) == 0
	}
	return isConstString(v)
}

// isPairs is a list of exactly two templates whose only variable is {i} (@json(pairs:)).
func isPairs(v AnnValue) bool {
	l, ok := v.(*AnnotationList)
	if !ok || len(l.Items) != pairWidth {
		return false
	}
	for _, it := range l.Items {
		if !templateVars(it, func(root string) bool { return root == pairIndex }) {
			return false
		}
	}
	return true
}
