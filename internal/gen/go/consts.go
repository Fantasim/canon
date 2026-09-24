package gogen

import (
	"math"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// constants writes the public constants: Int and Float typed, lists and maps as functions (§5.1).
func (g *gen) constants() {
	for _, c := range g.p.Consts {
		g.constant(c)
	}
}

func (g *gen) constant(c *ir.Const) {
	defer g.enter(c.Name)()
	name := g.names.ConstName(c)
	g.body.WriteString(docFor(name, c.Doc))
	switch c.Type.Kind {
	case types.List, types.Map:
		g.printf("func %s() %s { return %s }\n\n", name, g.goType(c.Type), g.expr(c.Type, c.V))
	case types.Int, types.Float:
		g.printf("const %s %s = %s\n\n", name, g.goType(c.Type), g.constExpr(c))
	case types.Bool, types.String, types.LitUnion, types.Duration, types.Enum:
		g.printf(constDeclFormat, name, g.constExpr(c))
	default:
		g.failKind(c.Type.Kind)
	}
}

// constExpr is a constant's value; -0.0 has no Go constant.
func (g *gen) constExpr(c *ir.Const) string {
	if f, ok := c.V.(*value.Float); ok && f.V == 0 && math.Signbit(f.V) {
		g.failf(ErrUnsupported, "constant %s is -0.0, which Go constants cannot hold", c.Name)
	}
	return g.expr(c.Type, c.V)
}
