package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// arithRow is a row of the operator table of TYPES.md §7.1 on scalars.
type arithRow struct {
	ka, kb types.Kind
	ops    map[syntax.TokenKind]bool
	result types.Type
}

// unary is `-e` (Int, Float, Duration; the expected type passes through) and `not e` (Bool).
func (c *checker) unary(env *env, e *syntax.UnaryExpr, want types.Type) types.Type {
	if e.Op == syntax.KwNot {
		c.cond(env, e.X)
		return types.BoolType
	}
	var t types.Type
	if literalOf(e.X) != nil || c.contextOperation(env, e.X) {
		t = c.expr(env, e.X, numericWant(want))
	} else {
		t = c.synth(env, e.X)
	}
	switch {
	case t.Kind() == types.Error:
		return t
	case t.Base().Kind() == types.Optional:
		c.report(env, diag.E3402.At(env.span(e.X), env.span(e.X)))
		return types.ErrorType
	case t.Base().Kind() == types.DepUnion:
		c.report(env, diag.E3804.At(env.span(e), e.Op.String(), depName(t)))
		return types.ErrorType
	case isNumD(t):
		return t.Base()
	}
	c.report(env, diag.E3007.AtUnary(env.span(e), e.Op.String(), t))
	return types.ErrorType
}

// numericWant passes an expected Int, Float or Duration to the operand of `-`.
func numericWant(want types.Type) types.Type {
	if want != nil && isNumD(unwrap(want)) {
		return unwrap(want)
	}
	return nil
}

func isNumD(t types.Type) bool {
	switch t.Base().Kind() {
	case types.Int, types.Float, types.Duration:
		return true
	default:
		return false
	}
}

// binary types a binary operator (TYPES.md §7.1).
func (c *checker) binary(env *env, e *syntax.BinaryExpr, want types.Type) types.Type {
	switch e.Op {
	case syntax.KwAnd, syntax.KwOr:
		return c.logical(env, e)
	case syntax.TokCoalesce:
		return c.coalesce(env, e, want)
	case syntax.KwIn:
		return c.inExpr(env, e)
	default:
	}
	both := c.contextDependent(env, e.X) && c.contextDependent(env, e.Y)
	if both && unknownContext(want) {
		return types.ErrorType
	}
	if w := exprType(want); both && w != nil && allArith[e.Op] {
		return c.contextArith(env, e, w)
	}
	tx, ty, ok := c.operands(env, e)
	if !ok {
		return types.ErrorType
	}
	switch e.Op {
	case syntax.TokEq, syntax.TokNe:
		return c.equality(env, e, tx, ty)
	case syntax.TokLt, syntax.TokLe, syntax.TokGt, syntax.TokGe:
		return c.ordering(env, e, tx, ty)
	default:
	}
	return c.arithmetic(env, e, tx, ty)
}

// logical is `C1 and C2` (C2 under T(C1)) and `C1 or C2` (C2 under F(C1)), TYPES.md §6.6.
func (c *checker) logical(env *env, e *syntax.BinaryExpr) types.Type {
	t1, f1 := c.cond(env, e.X)
	under := t1
	if e.Op == syntax.KwOr {
		under = f1
	}
	c.cond(env.withFacts(under), e.Y)
	return types.BoolType
}

// cond checks a condition against Bool and returns its facts (TYPES.md §6.6).
func (c *checker) cond(env *env, e syntax.Expr) (tf, ff facts) {
	c.expr(env, e, types.BoolType)
	return c.factsOf(env, e)
}

// contextDependent reports an operand read from the other one's type (TYPES.md §5.1).
func (c *checker) contextDependent(env *env, e syntax.Expr) bool {
	switch x := e.(type) {
	case *syntax.IdentExpr:
		return c.lookup(env, x.Name) == nil
	case *syntax.NoneLit, *syntax.IntLit, *syntax.FloatLit:
		return true
	case *syntax.ListLit:
		return len(x.Elems) == 0
	case *syntax.BraceLit:
		return len(x.Items) == 0 && len(x.Clauses) == 0
	case *syntax.ParenExpr:
		return c.contextDependent(env, x.X)
	case *syntax.UnaryExpr:
		return x.Op == syntax.TokMinus && c.contextDependent(env, x.X)
	case *syntax.BinaryExpr:
		return allArith[x.Op] && c.contextDependent(env, x.X) && c.contextDependent(env, x.Y)
	}
	return false
}

// contextOperation reports `-e` or arithmetic of context-dependent operands, typed like a literal (TYPES.md §5.1).
func (c *checker) contextOperation(env *env, e syntax.Expr) bool {
	switch inner(e).(type) {
	case *syntax.UnaryExpr, *syntax.BinaryExpr:
		return c.contextDependent(env, e)
	default:
		return false
	}
}

// contextArith checks both context-dependent operands against w, then applies the operator table (TYPES.md §5.1, §7.1).
func (c *checker) contextArith(env *env, e *syntax.BinaryExpr, w types.Type) types.Type {
	tx, ty := c.contextOperand(env, e.X, w), c.contextOperand(env, e.Y, w)
	if tx.Kind() == types.Error || ty.Kind() == types.Error {
		return types.ErrorType
	}
	return c.arithmetic(env, e, tx, ty)
}

// contextOperand checks one operand against w; an integer literal stands for an expected Float (TYPES.md §5.3).
func (c *checker) contextOperand(env *env, e syntax.Expr, w types.Type) types.Type {
	t := c.expr(env, e, w)
	switch {
	case t.Kind() == types.Error:
		return t
	case isIntLiteral(e) && floatTarget(w):
		return w
	case !c.assignable(t, w):
		return types.ErrorType // reported by expr's accept
	}
	return t
}

// exprType is what a context-dependent operand takes from t: Int, Float, or t's list; nil otherwise (TYPES.md §7.2, §7.3).
func exprType(t types.Type) types.Type {
	u := unwrap(t)
	if u == nil {
		return nil
	}
	switch u.Kind() {
	case types.Int:
		return types.IntType
	case types.Float:
		return types.FloatType
	case types.List:
		return u
	default:
		return nil
	}
}

// operands types both operands of a comparison or arithmetic: the context-dependent one is
// checked against the other's type, else the left is synthesized and the right checked; both
// context-dependent is E3008.
func (c *checker) operands(env *env, e *syntax.BinaryExpr) (tx, ty types.Type, ok bool) {
	dx, dy := c.contextDependent(env, e.X), c.contextDependent(env, e.Y)
	switch {
	case dx && dy:
		c.bothDependent(env, e)
		return nil, nil, false
	case dx:
		ty = c.synth(env, e.Y)
		tx = c.operand(env, e.X, ty)
	default:
		tx = c.synth(env, e.X)
		ty = c.operand(env, e.Y, tx)
	}
	ok = tx.Kind() != types.Error && ty.Kind() != types.Error
	return tx, ty, ok
}

// operand types the checked operand from the other's type: a name, `[]` or `{}` against it,
// `none` as None, a numeric literal as a Float where the other is one.
func (c *checker) operand(env *env, e syntax.Expr, other types.Type) types.Type {
	switch inner(e).(type) {
	case *syntax.NoneLit:
		for x := e; ; x = x.(*syntax.ParenExpr).X {
			c.info.Types[x] = types.NoneType
			if _, isParen := x.(*syntax.ParenExpr); !isParen {
				break
			}
		}
		return types.NoneType
	case *syntax.IntLit:
		if unwrap(other).Kind() == types.Float {
			c.expr(env, e, unwrap(other))
			return unwrap(other)
		}
		return c.synth(env, e)
	case *syntax.IdentExpr, *syntax.ListLit, *syntax.BraceLit, *syntax.LambdaExpr:
		if other.Kind() == types.Error {
			return c.synth(env, e)
		}
		return c.checkedOperand(env, e, other)
	case *syntax.StringLit, *syntax.RawStringLit:
		if _, isUnion := unwrap(other).(*types.LitUnionType); isUnion {
			return c.checkedOperand(env, e, other)
		}
	case *syntax.UnaryExpr, *syntax.BinaryExpr:
		if w := exprType(other); w != nil && c.contextOperation(env, e) {
			return c.expr(env, e, w)
		}
	}
	return c.synth(env, e)
}

// checkedOperand checks e against the other operand's type: T? for a name, else T (TYPES.md §7.5).
func (c *checker) checkedOperand(env *env, e syntax.Expr, other types.Type) types.Type {
	want := optElem(other)
	if _, isName := inner(e).(*syntax.IdentExpr); isName {
		want = &types.OptionalType{Elem: want}
	}
	t := c.exprNode(env, inner(e), want)
	if t == nil {
		t = types.ErrorType
	}
	for x := e; ; x = x.(*syntax.ParenExpr).X { // a paren has its operand's type (TYPES.md §5.1)
		c.info.Types[x] = t
		if _, isParen := x.(*syntax.ParenExpr); !isParen {
			return t
		}
	}
}

func inner(e syntax.Expr) syntax.Expr {
	for {
		p, ok := e.(*syntax.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// equality is `==` and `!=` (TYPES.md §7.5).
func (c *checker) equality(env *env, e *syntax.BinaryExpr, tx, ty types.Type) types.Type {
	if tx.Kind() == types.None || ty.Kind() == types.None {
		c.noneTest(env, e, tx, ty)
		return types.BoolType
	}
	a, b := optElem(tx), optElem(ty)
	if c.inError(a) || c.inError(b) {
		return types.BoolType
	}
	if c.unionValues(env, e, tx, ty) || c.unionEquality(env, e, a, b) || c.dependent(env, e, a, b) {
		return types.BoolType
	}
	switch {
	case c.brokenRef(a) || c.brokenRef(b):
	case a.Base().Kind() == types.Func || b.Base().Kind() == types.Func:
		c.report(env, diag.E3007.AtBinary(env.span(e), e.Op.String(), tx, ty))
	case a.Base().Kind() == types.Ref && b.Base().Kind() == types.Ref && !types.Identical(a, b):
		c.report(env, diag.E3309.At(env.span(e), tx, ty))
	case types.Identical(a, b):
	case c.derefOperand(e.X, a, b) || c.derefOperand(e.Y, b, a):
	case caseOf(a, b), c.unionOf(a, b) || c.unionOf(b, a):
	default:
		c.report(env, diag.E3002.At(env.span(e.Y), tx, ty))
	}
	return types.BoolType
}

// dependent is E3804 for an operator on a dependent value but == and != alike (TYPES.md §11.4).
func (c *checker) dependent(env *env, e *syntax.BinaryExpr, a, b types.Type) bool {
	d := a
	if d.Base().Kind() != types.DepUnion {
		d = b
	}
	if d.Base().Kind() != types.DepUnion || (isEquality(e.Op) && types.Identical(a, b)) {
		return false
	}
	c.report(env, diag.E3804.At(env.span(e), e.Op.String(), depName(d)))
	return true
}

func isEquality(op syntax.TokenKind) bool { return op == syntax.TokEq || op == syntax.TokNe }

// unionOf reports a literal union compared with a value of its alternative (TYPES.md §13.2).
func (c *checker) unionOf(a, b types.Type) bool {
	u, ok := a.Base().(*types.LitUnionType)
	return ok && c.assignable(b, u.Of)
}

// optElem is T for T? and T itself.
func optElem(t types.Type) types.Type {
	if o, ok := t.Base().(*types.OptionalType); ok {
		return o.Elem
	}
	return t
}

// derefOperand dereferences a ref compared with its entry type (TYPES.md §7.5).
func (c *checker) derefOperand(e syntax.Expr, a, b types.Type) bool {
	r, ok := a.Base().(*types.RefType)
	if !ok || !types.Identical(c.coll(r).Elem, b) {
		return false
	}
	e = inner(e)
	d := &Conversion{Kind: ConvDeref, From: a, To: c.coll(r).Elem}
	if t := c.info.Types[e]; t != nil && t.Base().Kind() == types.Optional {
		d = &Conversion{Kind: ConvPresent, From: t, To: &types.OptionalType{Elem: d.To}, Inner: d}
	}
	c.info.Conv[e] = d
	return true
}

// caseOf reports a case compared with its variant, or two cases of one variant.
func caseOf(a, b types.Type) bool {
	va, vb := variantOf(a), variantOf(b)
	return va != nil && va == vb
}

func variantOf(t types.Type) *types.VariantType {
	if t == nil {
		return nil
	}
	switch x := t.Base().(type) {
	case *types.VariantType:
		return x
	case *types.CaseType:
		return x.Variant
	}
	return nil
}

// noneTest is `x == none` or `x != none`: W3401 when x is never none (TYPES.md §6.5).
func (c *checker) noneTest(env *env, e *syntax.BinaryExpr, tx, ty types.Type) {
	x, t := e.X, tx
	if tx.Kind() == types.None {
		x, t = e.Y, ty
	}
	switch t.Base().Kind() {
	case types.Optional, types.None, types.Error, types.DepUnion:
	default:
		c.warn(env, diag.W3401.At(env.span(x), env.span(x), e.Op.String()))
	}
}

// ordering is `<`, `<=`, `>`, `>=` (TYPES.md §7.5).
func (c *checker) ordering(env *env, e *syntax.BinaryExpr, tx, ty types.Type) types.Type {
	for _, side := range []struct {
		x syntax.Expr
		t types.Type
	}{{e.X, tx}, {e.Y, ty}} {
		if k := side.t.Base().Kind(); k == types.Optional || k == types.None {
			elem := optElem(side.t)
			c.report(env, diag.E3403.At(env.span(side.x), elem, foundOptional(side.t, elem)))
			return types.BoolType
		}
	}
	if c.dependent(env, e, tx, ty) || c.inError(tx) || c.inError(ty) {
		return types.BoolType
	}
	if !orderable(tx) {
		c.report(env, diag.E3310.At(env.span(e), e.Op.String(), tx))
		return types.BoolType
	}
	if !types.Identical(tx, ty) {
		c.report(env, diag.E3310.At(env.span(e), e.Op.String(), ty))
	}
	return types.BoolType
}

// orderable reports Int, Float, Duration, String and ordered enums (STDLIB.md `Ord`).
func orderable(t types.Type) bool {
	switch x := t.Base().(type) {
	case types.Basic:
		return x.K != types.Bool
	case *types.EnumType:
		return x.Ordered
	}
	return false
}

// arithmetic is the rows of the operator table for + - * / % (TYPES.md §7.1).
func (c *checker) arithmetic(env *env, e *syntax.BinaryExpr, tx, ty types.Type) types.Type {
	for _, side := range []struct {
		x syntax.Expr
		t types.Type
	}{{e.X, tx}, {e.Y, ty}} {
		if k := side.t.Base().Kind(); k == types.Optional || k == types.None {
			c.report(env, diag.E3402.At(env.span(side.x), env.span(side.x)))
			return types.ErrorType
		}
	}
	if c.dependent(env, e, tx, ty) || c.inError(tx) || c.inError(ty) {
		return types.ErrorType
	}
	if t := arithResult(e.Op, tx.Base(), ty.Base()); t != nil {
		return t
	}
	if e.Op == syntax.TokPlus {
		if t, ok := c.listConcat(tx, ty); ok {
			c.joined(e.X, tx, t)
			c.joined(e.Y, ty, t)
			return t
		}
	}
	c.report(env, diag.E3007.AtBinary(env.span(e), e.Op.String(), tx, ty))
	return types.ErrorType
}

// arithResult is the result of op on two scalar operands, nil when the table has no row.
func arithResult(op syntax.TokenKind, a, b types.Type) types.Type {
	ka, kb := a.Kind(), b.Kind()
	for _, row := range arithRows {
		if row.ka == ka && row.kb == kb && row.ops[op] {
			return row.result
		}
	}
	return nil
}

// joined records how an operand of `[S] + [T]` becomes the joined list (TYPES.md §6.4).
func (c *checker) joined(e syntax.Expr, s, t types.Type) {
	e = inner(e)
	if _, done := c.info.Conv[e]; done {
		return
	}
	if conv, ok := c.convert(s, t); ok && conv != nil {
		c.info.Conv[e] = conv
	}
}

// listConcat is `[S] + [T]`, a plain list of S ⊔ T; a ref in error joins as the error type (TYPES.md §1).
func (c *checker) listConcat(a, b types.Type) (types.Type, bool) {
	a, b = c.unbroken(a), c.unbroken(b)
	la, okA := a.Base().(*types.ListType)
	lb, okB := b.Base().(*types.ListType)
	if !okA || !okB {
		return nil, false
	}
	j, ok := types.Join(la.Elem, lb.Elem)
	if !ok {
		return nil, false
	}
	return &types.ListType{Elem: j}, true
}
