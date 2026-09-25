package gogen

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// types writes records and variants in declaration order (CODEGEN.md §2.7).
func (g *gen) types() {
	for _, t := range g.p.Types {
		switch t := t.(type) {
		case *ir.Record:
			g.record(t)
		case *ir.Variant:
			g.variant(t)
		case *ir.Dependent:
			// E8019 DependentType already refuses this at stage E: unreachable.
			g.failf(ErrMalformed, "dependent type %s", t.QName())
		}
	}
}

// variant writes the variant, then one type per case with fields (CODEGEN.md §5.5).
func (g *gen) variant(v *ir.Variant) {
	name := g.goName(v)
	g.body.WriteString(docFor(name, v.Doc))
	g.printf(variantFormat, name, g.names.KindName(v), ir.GoKindStore, ir.GoCaseStore, ir.GoKind)
	for _, c := range v.Cases {
		if len(c.Fields) == 0 {
			if len(c.Methods) > 0 {
				// E8019 FieldlessCaseExportFn already refuses this at stage E: unreachable.
				g.failf(ErrMalformed, "export fns of %s.%s, a case without fields", v.QName(), c.Name)
			}
			continue
		}
		g.printf(asCaseFormat, name, g.names.AsName(c), g.names.CaseName(v, c), ir.GoCaseStore)
	}
	for _, c := range v.Cases {
		if len(c.Fields) > 0 {
			g.caseType(v, c)
		}
	}
}

func (g *gen) caseType(v *ir.Variant, c *ir.Case) {
	g.typeDecl(g.caseBody(v, c), c.Doc)
}

// variantExpr is a variant value, *V holding its kind and its case, or a case value *VC.
func (g *gen) variantExpr(t ir.TypeRef, r *value.Record) string {
	v, ok := t.Named.(*ir.Variant)
	var ct *types.CaseType
	isCase := false
	if r.T != nil {
		ct, isCase = r.T.Base().(*types.CaseType)
	}
	if !ok || !isCase || ct.Index < 0 || ct.Index >= len(v.Cases) {
		g.failf(ErrMalformed, "a variant value of %s", qname(t.Named))
		return nilLit
	}
	if v.Pkg != g.p.Name {
		// E8019 CrossPackageBakedValue already refuses this at stage E: unreachable.
		g.failf(ErrMalformed, "a baked value of %s, a variant of another package", v.QName())
	}
	c := v.Cases[ct.Index]
	name := g.goName(v)
	var lit string
	if len(c.Fields) > 0 {
		b := g.caseBody(v, c)
		lit = ampersand + compositeLit(b.goName, g.bodyLit(b, r))
	}
	if t.Kind == types.Case {
		return lit
	}
	parts := []pair{{ir.GoKindStore, g.kindLit(v, ct.Index)}}
	if lit != "" {
		parts = append(parts, pair{ir.GoCaseStore, lit})
	}
	return ampersand + compositeLit(name, parts)
}
