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
			g.failf(ErrUnsupported, "dependent type %s", t.QName())
		}
	}
}

// variant writes the variant, then one type per case with fields (CODEGEN.md §5.5).
func (g *gen) variant(v *ir.Variant) {
	name := g.goName(v)
	kind := kindName(name)
	g.declare(name, v.QName())
	g.body.WriteString(docFor(name, v.Doc))
	g.printf(variantFormat, name, kind)
	methods := newScope(v.QName())
	for _, n := range []string{kindStore, caseStore, kindSuffix} {
		g.fail(methods.add(n, v.QName()))
	}
	for _, c := range v.Cases {
		if len(c.Fields) == 0 {
			if len(c.Methods) > 0 {
				g.failf(ErrUnsupported, "export fns of %s.%s, a case without fields", v.QName(), c.Name)
			}
			continue
		}
		as := asPrefix + upperCamel(c.Name)
		g.fail(methods.add(as, v.QName()))
		g.printf(asCaseFormat, name, as, caseName(name, c))
	}
	for _, c := range v.Cases {
		if len(c.Fields) > 0 {
			g.caseType(v, c)
		}
	}
}

func (g *gen) caseType(v *ir.Variant, c *ir.Case) {
	origin := v.QName() + dot + c.Name
	name := caseName(g.goName(v), c)
	g.declare(name, origin)
	g.typeDecl(g.recordBody(origin, name, c.Fields, c.Methods), c.Doc)
}

// variantExpr is a variant value, *V holding its kind and its case, or a case value *VC.
func (g *gen) variantExpr(t ir.TypeRef, r *value.Record) string {
	v, ok := t.Named.(*ir.Variant)
	var ct *types.CaseType
	isCase := false
	if r.T != nil {
		ct, isCase = r.T.Base().(*types.CaseType)
	}
	if !ok || !isCase || ct.Index >= len(v.Cases) {
		g.failf(ErrMalformed, "a variant value of %s", qname(t.Named))
		return nilLit
	}
	if v.Pkg != g.p.Name {
		g.failf(ErrUnsupported, "a baked value of %s, a variant of another package", v.QName())
	}
	c := v.Cases[ct.Index]
	name := g.goName(v)
	var lit string
	if len(c.Fields) > 0 {
		b := g.recordBody(v.QName()+dot+c.Name, caseName(name, c), c.Fields, c.Methods)
		lit = ampersand + compositeLit(caseName(name, c), g.bodyLit(b, r))
	}
	if t.Kind == types.Case {
		return lit
	}
	parts := []pair{{kindStore, g.kindLit(v, ct.Index)}}
	if lit != "" {
		parts = append(parts, pair{caseStore, lit})
	}
	return ampersand + compositeLit(name, parts)
}
