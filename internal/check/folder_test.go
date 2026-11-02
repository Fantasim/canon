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

// literalFolder is the fixture folder of check's tests: literals, `-n`, parentheses, consts, a field of a record literal (IMPLEMENTATION-PLAN §4.7).
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
	case *syntax.ListLit:
		return foldList(ctx, x, info, depth)
	case *syntax.SelectorExpr:
		if sel := info.Selections[x]; sel != nil && sel.Kind == check.SelField {
			f, ok := fieldOf(x, info, depth+1)
			return foldIf(ctx, f, ok, info, depth+1)
		}
		o := info.NameUses[x.Name]
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

// foldList folds a list literal whose elements all fold.
func foldList(ctx context.Context, x *syntax.ListLit, info *check.Info, depth int) (value.Value, bool) {
	l := &value.List{T: info.Types[x]}
	for _, e := range x.Elems {
		v, ok := foldLiteral(ctx, e, info, depth+1)
		if !ok {
			return nil, false
		}
		l.Elems = append(l.Elems, v)
	}
	return l, true
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

// foldIf folds e when ok.
func foldIf(ctx context.Context, e syntax.Expr, ok bool, info *check.Info, depth int) (value.Value, bool) {
	if !ok {
		return nil, false
	}
	return foldLiteral(ctx, e, info, depth)
}

// fieldOf is the expression giving the field x reads: the record literal's item, or the field's default (TYPES.md §7.4; DECISIONS 444).
func fieldOf(x *syntax.SelectorExpr, info *check.Info, depth int) (syntax.Expr, bool) {
	lit, ok := braceOf(x.X, info, depth)
	if !ok {
		return nil, false
	}
	for _, it := range lit.Items {
		f, isField := it.(*syntax.FieldItem)
		if !isField {
			return nil, false // a spread gives what this folder does not follow
		}
		if f.Name.Name == x.Name.Name {
			return f.Value, true
		}
	}
	obj := info.Selections[x].Obj
	if obj == nil {
		return nil, false
	}
	d, isDecl := obj.Decl().(*syntax.FieldDecl)
	if !isDecl {
		return nil, false
	}
	return d.Default, d.Default != nil
}

// braceOf is the record literal e builds, through parentheses, a typed literal, a const and a field read.
func braceOf(e syntax.Expr, info *check.Info, depth int) (*syntax.BraceLit, bool) {
	if depth > maxFold {
		return nil, false
	}
	switch x := e.(type) {
	case *syntax.ParenExpr:
		return braceOf(x.X, info, depth+1)
	case *syntax.TypedLit:
		return x.Lit, true
	case *syntax.BraceLit:
		return x, true
	case *syntax.IdentExpr:
		if o := info.Uses[x]; o != nil && o.Kind() == check.ObjConst {
			return braceOf(o.Decl().(*syntax.ConstDecl).Value, info, depth+1)
		}
	case *syntax.SelectorExpr:
		if sel := info.Selections[x]; sel != nil && sel.Kind == check.SelField {
			f, ok := fieldOf(x, info, depth+1)
			return braceIf(f, ok, info, depth+1)
		}
	}
	return nil, false
}

// braceIf is braceOf(e) when ok.
func braceIf(e syntax.Expr, ok bool, info *check.Info, depth int) (*syntax.BraceLit, bool) {
	if !ok {
		return nil, false
	}
	return braceOf(e, info, depth)
}
