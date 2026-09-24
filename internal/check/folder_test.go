package check_test

import (
	"context"
	"math/big"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// maxFold bounds the constants a fixture fold follows.
const maxFold = 64

// literalFolder is the fixture folder of check's tests: literals, `-n`, parentheses, consts (IMPLEMENTATION-PLAN §4.7).
type literalFolder struct{}

func (literalFolder) Fold(ctx context.Context, _ check.Object, e syntax.Expr, info *check.Info) (value.Value, bool) {
	return foldLiteral(ctx, e, info, 0)
}

func foldLiteral(ctx context.Context, e syntax.Expr, info *check.Info, depth int) (value.Value, bool) {
	if depth > maxFold || ctx.Err() != nil {
		return nil, false
	}
	switch x := e.(type) {
	case *syntax.ParenExpr:
		return foldLiteral(ctx, x.X, info, depth+1)
	case *syntax.IdentExpr:
		o := info.Uses[x]
		if o == nil || o.Kind() != check.ObjConst {
			return nil, false
		}
		return foldLiteral(ctx, o.Decl().(*syntax.ConstDecl).Value, info, depth+1)
	case *syntax.UnaryExpr:
		return negate(x, info)
	default:
	}
	return literal(e, info)
}

// negate folds `-n` for a numeric literal n.
func negate(x *syntax.UnaryExpr, info *check.Info) (value.Value, bool) {
	if x.Op != syntax.TokMinus {
		return nil, false
	}
	switch v := x.X.(type) {
	case *syntax.IntLit:
		return literal(&syntax.IntLit{Value: new(big.Int).Neg(v.Value)}, info)
	case *syntax.FloatLit:
		return literal(&syntax.FloatLit{Neg: !v.Neg, Coef: v.Coef, Exp: v.Exp}, info)
	default:
	}
	return nil, false
}

func literal(e syntax.Expr, info *check.Info) (value.Value, bool) {
	t := info.Types[e]
	switch x := e.(type) {
	case *syntax.IntLit:
		return &value.Int{V: x.Value.Int64(), T: t}, x.Value.IsInt64()
	case *syntax.FloatLit:
		f, err := strconv.ParseFloat(x.Coef.String()+"e"+strconv.FormatInt(x.Exp, 10), 64)
		if x.Neg {
			f = -f
		}
		return &value.Float{V: f, T: t}, err == nil
	case *syntax.DurationLit:
		return &value.Dur{Ms: x.Millis}, true
	case *syntax.BoolLit:
		return &value.Bool{V: x.Value}, true
	case *syntax.RawStringLit:
		return &value.Str{V: x.Value, T: t}, true
	case *syntax.StringLit:
		var b strings.Builder
		for _, p := range x.Parts {
			if p.Interp != nil {
				return nil, false
			}
			b.WriteString(p.Text)
		}
		return &value.Str{V: b.String(), T: t}, true
	default:
	}
	return nil, false
}
