package eval

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// exprFn evaluates one kind of expression node; at is the value path of what it builds.
type exprFn func(r *run, e syntax.Expr, at *vpath) value.Value

// exprTable dispatches on the node kind (DECISIONS 26); filled in init, since its handlers
// evaluate subexpressions through it.
var exprTable [syntax.NodeKindCount]exprFn

func init() {
	exprTable = [syntax.NodeKindCount]exprFn{
		syntax.KindIdentExpr: evalIdent, syntax.KindIntLit: evalInt, syntax.KindFloatLit: evalFloat,
		syntax.KindDurationLit: evalDuration, syntax.KindStringLit: evalString,
		syntax.KindRawStringLit: evalRaw, syntax.KindRegexLit: evalRegex, syntax.KindBoolLit: evalBool,
		syntax.KindNoneLit: evalNone, syntax.KindSelfExpr: evalSelf, syntax.KindUnaryExpr: evalUnary,
		syntax.KindBinaryExpr: evalBinary, syntax.KindIsExpr: evalIs, syntax.KindRangeExpr: evalRange,
		syntax.KindSelectorExpr: evalLink, syntax.KindIndexExpr: evalLink, syntax.KindCallExpr: evalLink,
		syntax.KindForceExpr: evalLink, syntax.KindLambdaExpr: evalLambda,
		syntax.KindShorthandLambda: evalLambda, syntax.KindListLit: evalList,
		syntax.KindListComp: evalListComp, syntax.KindBraceLit: evalBrace, syntax.KindTypedLit: evalTyped,
		syntax.KindIfExpr: evalIf, syntax.KindMatchExpr: evalMatch, syntax.KindLoadExpr: evalLoad,
	}
}

// eval evaluates e, then its conversion (EVALUATION.md §4.3, §12.1); nil: the root aborted.
func (r *run) eval(e syntax.Expr) value.Value {
	e = syntax.Unparen(e)
	at := r.at
	r.at = nil
	if r.failed || e == nil {
		return nil
	}
	v := r.node(e, at)
	if r.chainNone {
		r.chainNone = false
		v = &value.None{T: r.typeOf(e), P: r.prov(e, value.ProvLiteral)}
	}
	if v == nil || r.failed {
		return nil
	}
	if conv := r.ev.info.Conv[e]; conv != nil {
		return r.convert(v, conv, e, at)
	}
	return v
}

// evalAt evaluates e as the value at path at, for the literal that builds it.
func (r *run) evalAt(e syntax.Expr, at *vpath) value.Value {
	r.at = at
	return r.eval(e)
}

// node spends the node's step and dispatches it.
func (r *run) node(e syntax.Expr, at *vpath) value.Value {
	if read, ok := r.fr.reads[e]; ok {
		r.chainNone = read.none // a TS entry read, reused for free
		return read.v
	}
	fn := exprTable[e.Kind()]
	if fn == nil {
		r.bug(e)
		return nil
	}
	if !r.step(e) {
		return nil
	}
	if r.ev.info.Keys[e] != nil {
		if k, ok := r.literalKey(e); ok {
			return r.keyValue(e, k)
		}
	}
	if at == nil {
		return fn(r, e, at)
	}
	outer := r.cur
	r.cur = at
	v := fn(r, e, at)
	r.cur = outer
	return v
}

// typeOf is the checker's type of e.
func (r *run) typeOf(e syntax.Expr) types.Type {
	if t := r.ev.info.Types[e]; t != nil {
		return t
	}
	return types.ErrorType
}

func evalInt(r *run, e syntax.Expr, _ *vpath) value.Value {
	x := e.(*syntax.IntLit)
	if !x.Value.IsInt64() {
		r.bug(e)
		return nil
	}
	return r.literal(&value.Int{V: x.Value.Int64(), T: types.IntType, P: r.prov(e, value.ProvLiteral)})
}

// evalFloat rounds the literal's exact decimal value to the nearest binary64.
func evalFloat(r *run, e syntax.Expr, _ *vpath) value.Value {
	x := e.(*syntax.FloatLit)
	f, err := strconv.ParseFloat(x.Coef.String()+exponent+strconv.FormatInt(x.Exp, decimalBase), floatBits)
	if err != nil {
		r.bug(e)
		return nil
	}
	if x.Neg {
		f = -f
	}
	return &value.Float{V: f, T: types.FloatType, P: r.prov(e, value.ProvLiteral)}
}

func evalDuration(r *run, e syntax.Expr, _ *vpath) value.Value {
	return &value.Dur{Ms: e.(*syntax.DurationLit).Millis, P: r.prov(e, value.ProvLiteral)}
}

func evalRaw(r *run, e syntax.Expr, _ *vpath) value.Value {
	return r.literal(&value.Str{V: e.(*syntax.RawStringLit).Value, T: types.StringType, P: r.prov(e, value.ProvLiteral)})
}

// evalRegex is the pattern of a regex literal, the argument of matches (STDLIB.md §8).
func evalRegex(r *run, e syntax.Expr, _ *vpath) value.Value {
	return &value.Str{V: e.(*syntax.RegexLit).Pattern, T: types.StringType, P: r.prov(e, value.ProvLiteral)}
}

func evalBool(r *run, e syntax.Expr, _ *vpath) value.Value {
	return &value.Bool{V: e.(*syntax.BoolLit).Value, P: r.prov(e, value.ProvLiteral)}
}

func evalNone(r *run, e syntax.Expr, _ *vpath) value.Value {
	return &value.None{T: r.typeOf(e), P: r.prov(e, value.ProvLiteral)}
}

func evalSelf(r *run, e syntax.Expr, _ *vpath) value.Value {
	if r.fr.self == nil {
		r.bug(e)
	}
	return r.fr.self
}

// evalLoad forces a load, decoded in this run; false poisons silently (EVALUATION.md §7.1).
func evalLoad(r *run, e syntax.Expr, at *vpath) value.Value {
	if r.nonConstant() {
		return nil
	}
	if r.ev.host == nil {
		r.bug(e)
		return nil
	}
	r.voidTrace() // what a load reads is not a value an entry's memo can compare
	site := &loadSite{r: r, at: e, coll: r.hint(at), cx: r.depAt(e)}
	return r.load(e.(*syntax.LoadExpr), r.typeOf(e), site)
}

// truth is the Bool a condition evaluates to; false with the root aborted.
func (r *run) truth(e syntax.Expr) (bool, bool) {
	v := r.eval(e)
	b, ok := v.(*value.Bool)
	if !ok {
		r.bug(e)
		return false, false
	}
	return b.V, true
}
