package ir

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// exprFn translates one kind of expression; nil when it is refused or broken.
type exprFn func(t *translator, e syntax.Expr) PExpr

// exprTable dispatches on the node kind, a kind without handler refused (DECISIONS 26, CONFORMANCE.md §2.2); init fills it, as handlers recurse through it.
var exprTable [syntax.NodeKindCount]exprFn

func init() {
	exprTable = [syntax.NodeKindCount]exprFn{
		syntax.KindIdentExpr: (*translator).ident, syntax.KindIntLit: (*translator).literal,
		syntax.KindFloatLit: (*translator).literal, syntax.KindDurationLit: (*translator).literal,
		syntax.KindBoolLit: (*translator).literal, syntax.KindRawStringLit: (*translator).literal,
		syntax.KindStringLit: (*translator).stringLit, syntax.KindUnaryExpr: (*translator).unary,
		syntax.KindBinaryExpr: (*translator).binary, syntax.KindIsExpr: (*translator).isExpr,
		syntax.KindSelectorExpr: (*translator).selector, syntax.KindCallExpr: (*translator).call,
		syntax.KindIfExpr: (*translator).ifExpr, syntax.KindBadExpr: (*translator).bad,
	}
}

// expr translates e once, then its conversion; the result is kept for fold.
func (t *translator) expr(e syntax.Expr) PExpr {
	e = unparen(e)
	if n, done := t.exprs[e]; done {
		return n
	}
	var n PExpr
	if fn := exprTable[e.Kind()]; fn != nil {
		n = fn(t, e)
	} else {
		t.refuse(e)
		t.scanChildren(e)
	}
	if n != nil {
		n = t.convert(e, n)
	}
	t.exprs[e] = n
	return n
}

// convert applies e's conversion: only an integer literal read as a Float is portable; a refused one names its `let`; one to a refused result type (E9004) is not judged.
func (t *translator) convert(e syntax.Expr, n PExpr) PExpr {
	conv := t.s.info.Conv[e]
	if conv == nil || (t.badRes && types.Identical(conv.To, t.site.sig.Result)) {
		return n
	}
	lit, isLit := n.(*Lit)
	if conv.Kind != check.ConvIntLitToFloat || !isLit {
		if l := t.letOf[e]; l != nil {
			t.refuse(l)
		} else {
			t.refuse(e)
		}
		return nil
	}
	if i, isInt := lit.V.(*value.Int); isInt {
		return &Lit{T: t.s.ref(conv.To), V: &value.Float{V: float64(i.V), T: types.FloatType}}
	}
	return &Lit{T: t.s.ref(conv.To), V: lit.V}
}

func (t *translator) bad(syntax.Expr) PExpr {
	t.broken = true
	return nil
}

// checked is e's type after its conversion; nil, marking the fn broken, when the checker left none.
func (t *translator) checked(e syntax.Expr) types.Type {
	e = unparen(e)
	var ty types.Type
	if conv := t.s.info.Conv[e]; conv != nil {
		ty = conv.To
	} else {
		ty = t.s.info.Types[e]
	}
	if ty == nil || ty.Kind() == types.Error {
		t.broken = true
		return nil
	}
	return ty
}

// kind is the base kind of e's type; Error when the checker left none.
func (t *translator) kind(e syntax.Expr) types.Kind {
	if ty := t.checked(e); ty != nil {
		return ty.Base().Kind()
	}
	return types.Error
}

// typ is e's type as IR, before its conversion (convert retypes a literal).
func (t *translator) typ(e syntax.Expr) TypeRef {
	ty := t.s.info.Types[unparen(e)]
	if ty == nil || ty.Kind() == types.Error {
		t.broken = true
		return TypeRef{Kind: types.Error}
	}
	return t.s.ref(ty)
}

// computed is the type an operation computes in: every integer is Int, every float Float (TYP-03, CONFORMANCE.md §2.3).
func (t *translator) computed(tr TypeRef) TypeRef {
	switch tr.Kind {
	case types.Int:
		return t.s.ref(types.IntType)
	case types.Float:
		return t.s.ref(types.FloatType)
	default:
		return tr
	}
}

// literal is a constant leaf, folded by the one evaluator; a literal naming a key its ref checks only when evaluated is refused.
func (t *translator) literal(e syntax.Expr) PExpr {
	if t.s.info.Keys[e] != nil {
		t.refuse(e)
		return nil
	}
	if t.s.in.Fold == nil {
		t.broken = true
		return nil
	}
	v, ok := t.s.in.Fold.Fold(t.s.ctx, t.site.obj, e, t.s.info)
	if !ok {
		t.broken = true
		return nil
	}
	return &Lit{T: t.typ(e), V: v}
}

// ident is a name: a parameter, a `let` local, a field of self, an enum member or a table entry
// as a ref; any other name (a constant, a value, a function) is outside the subset.
func (t *translator) ident(e syntax.Expr) PExpr {
	x := e.(*syntax.IdentExpr)
	info := t.s.info
	if info.Keys[x] != nil || info.Symbols[x] {
		t.refuse(x)
		return nil
	}
	o := info.Uses[x]
	if o == nil {
		t.broken = true
		return nil
	}
	switch o.Kind() {
	case check.ObjParam:
		return t.param(x, o)
	case check.ObjLocal:
		return &LocalRef{T: t.typ(x), Name: o.Name()}
	case check.ObjField:
		n, _ := t.selfRead(x, ctxValue)
		return n
	case check.ObjMember:
		return t.member(o)
	case check.ObjEntry:
		return t.entry(x, o)
	default:
		t.refuse(x)
		return nil
	}
}

// param is a declared parameter of the fn.
func (t *translator) param(x *syntax.IdentExpr, o check.Object) PExpr {
	for i, p := range t.site.decl.Params {
		if p.Name != nil && t.s.info.Defs[p.Name] == o && i < len(t.site.fn.Params) {
			return &ParamRef{T: t.site.fn.Params[i].Type, Index: i}
		}
	}
	t.refuse(x)
	return nil
}

// member is an enum member, by its index in declaration order.
func (t *translator) member(o check.Object) PExpr {
	en, ok := o.Type().Base().(*types.EnumType)
	if !ok {
		t.broken = true
		return nil
	}
	for i, m := range en.Members {
		if m.Name == o.Name() {
			return &Lit{T: t.s.ref(en), V: &value.Member{Enum: en, Index: i}}
		}
	}
	t.broken = true
	return nil
}

// entry is a table entry named where a ref is expected: its key, the value a translation returns for a `ref T` (CONFORMANCE.md §2.1).
func (t *translator) entry(e syntax.Expr, o check.Object) PExpr {
	ty := t.checked(e)
	if ty == nil || ty.Base().Kind() != types.Ref {
		t.refuse(e)
		return nil
	}
	return &Lit{T: t.s.ref(ty), V: &value.Ref{T: ty, Key: value.Key{S: o.Name()}}}
}

// selector is an enum member `E.m` or a path of self; anything else (a package member, a ref
// dereference, a built-in member) is outside the subset.
func (t *translator) selector(e syntax.Expr) PExpr {
	x := e.(*syntax.SelectorExpr)
	if o := t.s.info.NameUses[x.Name]; o != nil && o.Kind() == check.ObjMember {
		return t.member(o)
	}
	if n, isRead := t.selfRead(x, ctxValue); isRead {
		return n
	}
	t.refuseScan(x, x.X)
	return nil
}

// ifExpr is `if c { a } else if … else { b }`, each branch evaluated only when taken.
func (t *translator) ifExpr(e syntax.Expr) PExpr {
	return t.ifChain(e.(*syntax.IfExpr), t.typ(e))
}

func (t *translator) ifChain(x *syntax.IfExpr, ty TypeRef) PExpr {
	cond := t.expr(x.Cond)
	var then, els PExpr
	if x.Then != nil {
		then = t.expr(x.Then.X)
	}
	switch {
	case x.ElseIf != nil:
		els = t.ifChain(x.ElseIf, ty)
	case x.Else != nil:
		els = t.expr(x.Else.X)
	}
	if !present(cond, then, els) {
		return nil
	}
	return &If{T: ty, Cond: cond, Then: then, Else: els}
}

// scanChildren translates the operands and match-arm values of a refused construct (decision 194); a lambda's or comprehension's names are not the fn's.
func (t *translator) scanChildren(e syntax.Expr) {
	switch e.(type) {
	case *syntax.LambdaExpr, *syntax.ShorthandLambda, *syntax.ListComp:
		return
	}
	t.within(func() {
		for c := range syntax.Children(e) {
			switch x := c.(type) {
			case syntax.Expr:
				t.expr(x)
			case *syntax.MatchArm:
				t.expr(x.Body)
			}
		}
	})
}

// refuseScan refuses n, then still translates its operands, a path of self excepted: it is part of n (decision 194).
func (t *translator) refuseScan(n syntax.Node, operands ...syntax.Expr) {
	t.refuse(n)
	t.within(func() {
		for _, x := range operands {
			if x != nil && !t.selfRooted(x) {
				t.expr(x)
			}
		}
	})
}

// selfRooted reports a chain of `.f` rooted at self or at a field named bare.
func (t *translator) selfRooted(e syntax.Expr) bool {
	for {
		switch x := unparen(e).(type) {
		case *syntax.SelfExpr:
			return true
		case *syntax.IdentExpr:
			o := t.s.info.Uses[x]
			return o != nil && o.Kind() == check.ObjField
		case *syntax.SelectorExpr:
			if x.X == nil {
				return false
			}
			e = x.X
		default:
			return false
		}
	}
}
