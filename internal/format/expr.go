package format

import (
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
)

// binary is a run of operators of one precedence level, broken after each operator (§7.2).
func (b *builder) binary(n *syntax.BinaryExpr) *doc {
	operands, ops := b.chain(n)
	var rest []*doc
	for i, op := range ops {
		rest = append(rest, b.operator(operands[i].Last(), op, b.node(operands[i+1]))...)
	}
	return group(false, b.node(operands[0]), indent(rest...))
}

// operator is " op", a break and the right operand; a comment moves the break before op.
func (b *builder) operator(left, op syntax.Tok, right *doc) []*doc {
	if b.lineEnds(left) || len(b.notes[op].lead) > 0 {
		return []*doc{lineDoc, b.tok(op), spaceDoc, right}
	}
	return []*doc{spaceDoc, b.tok(op), lineDoc, right}
}

// lineEnds reports a trailing line comment after t, which ends its line.
func (b *builder) lineEnds(t syntax.Tok) bool {
	return !b.heldTrail[t] && slices.ContainsFunc(b.notes[t].trail, isLine)
}

// chain flattens the binary expressions of n's level: the left operand of a left-associative
// operator, the right one of "??".
func (b *builder) chain(n *syntax.BinaryExpr) (operands []syntax.Expr, ops []syntax.Tok) {
	level := binaryLevel[n.Op]
	if level == levelCompare {
		return []syntax.Expr{n.X, n.Y}, []syntax.Tok{n.OpTok}
	}
	if level == levelCoalesce {
		operands = append(operands, n.X)
		ops = append(ops, n.OpTok)
		if y, ok := n.Y.(*syntax.BinaryExpr); ok && binaryLevel[y.Op] == level {
			more, moreOps := b.chain(y)
			return append(operands, more...), append(ops, moreOps...)
		}
		return append(operands, n.Y), ops
	}
	if x, ok := n.X.(*syntax.BinaryExpr); ok && binaryLevel[x.Op] == level {
		operands, ops = b.chain(x)
	} else {
		operands = []syntax.Expr{n.X}
	}
	return append(operands, n.Y), append(ops, n.OpTok)
}

func (b *builder) isExpr(n *syntax.IsExpr) *doc {
	is := b.before(n.Target.First())
	return group(false, b.node(n.X), indent(b.operator(n.X.Last(), is, b.node(n.Target))...))
}

// unary is "-x" or "not x" (§5).
func (b *builder) unary(n *syntax.UnaryExpr) *doc {
	if n.Op == syntax.KwNot {
		return cat(b.tok(n.First()), spaceDoc, b.node(n.X))
	}
	return cat(b.tok(n.First()), b.node(n.X))
}

// rangeExpr has no spaces and no break point; a space keeps a shorthand upper bound
// from joining the operator.
func (b *builder) rangeExpr(n *syntax.RangeExpr) *doc {
	var ds []*doc
	if n.Lo != nil {
		ds = append(ds, b.node(n.Lo))
	}
	ds = append(ds, b.tok(n.OpTok))
	if n.Hi != nil {
		if b.f.Tokens[n.Hi.First()].Kind == syntax.TokDot {
			ds = append(ds, spaceDoc)
		}
		ds = append(ds, b.node(n.Hi))
	}
	return cat(ds...)
}

// postfix is a chain of postfix steps: with two calls or more it breaks before each "." or
// "?." step, otherwise it is a concatenation.
func (b *builder) postfix(n syntax.Expr) *doc {
	head, steps := n, []syntax.Expr(nil)
	calls := 0
	for {
		inner := postfixInner(head)
		if inner == nil {
			break
		}
		if _, ok := head.(*syntax.CallExpr); ok {
			calls++
		}
		steps = append(steps, head)
		head = inner
	}
	lead, ds := []*doc{b.node(head)}, []*doc(nil)
	for _, st := range slices.Backward(steps) {
		_, dot := st.(*syntax.SelectorExpr)
		switch {
		case calls < minChainCalls || !dot && ds == nil:
			lead = append(lead, b.step(st))
		case dot:
			ds = append(ds, softlineDoc, b.step(st))
		default:
			ds = append(ds, b.step(st))
		}
	}
	if ds == nil {
		return cat(lead...)
	}
	return group(false, cat(lead...), indent(ds...))
}

// postfixInner is the expression a postfix step applies to; nil for anything else, and for
// the root of a shorthand lambda.
func postfixInner(x syntax.Expr) syntax.Expr {
	switch x := x.(type) {
	case *syntax.SelectorExpr:
		if x.X == nil {
			return nil
		}
		return x.X
	case *syntax.CallExpr:
		return x.Fun
	case *syntax.IndexExpr:
		return x.X
	case *syntax.ForceExpr:
		return x.X
	default:
		return nil
	}
}

// step is one postfix step without the expression it applies to.
func (b *builder) step(x syntax.Expr) *doc {
	switch x := x.(type) {
	case *syntax.SelectorExpr:
		return cat(b.tok(b.before(x.Name.First())), b.node(x.Name))
	case *syntax.CallExpr:
		return b.args(x.Parens, x.Args)
	case *syntax.IndexExpr:
		return cat(b.tok(x.Brackets.Open), b.node(x.Index), b.closing(x.Brackets.Close, true), b.closer(x.Brackets.Close))
	default:
		return b.tok(x.Last())
	}
}

// selector is a shorthand lambda's root ".name"; any other postfix node is a chain.
func (b *builder) selector(n *syntax.SelectorExpr) *doc {
	if n.X == nil {
		return b.step(n)
	}
	return b.postfix(n)
}

// args is a call's argument list; a lone literal argument hugs the parentheses (§6.2).
func (b *builder) args(parens syntax.Delims, args []*syntax.Arg) *doc {
	if b.hugs(parens, args) {
		return cat(b.tok(parens.Open), b.node(args[0]), b.closer(parens.Close))
	}
	return b.parenList(parens.Open, parens.Close, partsOf(b, args))
}

func (b *builder) hugs(parens syntax.Delims, args []*syntax.Arg) bool {
	if len(args) != 1 || args[0].Name != nil {
		return false
	}
	a := args[0]
	if len(b.notes[parens.Open].trail)+len(b.notes[a.First()].lead)+len(b.notes[a.Last()].trail)+len(b.notes[parens.Close].lead) > 0 {
		return false
	}
	switch args[0].Value.(type) {
	case *syntax.BraceLit, *syntax.TypedLit, *syntax.ListLit:
		return true
	default:
		return false
	}
}

func (b *builder) arg(n *syntax.Arg) *doc {
	if n.Name == nil {
		return b.node(n.Value)
	}
	return cat(b.node(n.Name), b.tok(b.after(n.Name.Last())), spaceDoc, b.node(n.Value))
}

func (b *builder) loadExpr(n *syntax.LoadExpr) *doc {
	ds := []*doc{b.tok(n.First())}
	if n.Method != nil {
		ds = append(ds, b.tok(b.before(n.Method.First())), b.node(n.Method))
	}
	return cat(append(ds, b.args(n.Parens, n.Args))...)
}

// paren is "(x)": parentheses are kept and have no break point of their own (§7.2, §10).
func (b *builder) paren(n *syntax.ParenExpr) *doc {
	return cat(b.tok(n.First()), b.node(n.X), b.closing(n.Last(), true), b.closer(n.Last()))
}

// lambda is "params => body" by rule A (§7.2).
func (b *builder) lambda(n *syntax.LambdaExpr) *doc {
	params := b.commaList(nodes(n.Params))
	if n.Parens.Open != syntax.NoTok {
		params = b.parenList(n.Parens.Open, n.Parens.Close, partsOf(b, n.Params))
	}
	return cat(params, b.assign(b.before(n.Body.First()), n.Body))
}

func (b *builder) shorthand(n *syntax.ShorthandLambda) *doc { return b.node(n.Body) }

// isBracket reports a value that rule A step 2 prints after the operator with its own
// brackets broken.
func isBracket(n syntax.Node) bool {
	switch n := n.(type) {
	case *syntax.BraceLit, *syntax.TypedLit, *syntax.ListLit, *syntax.ListComp, *syntax.IfExpr,
		*syntax.MatchExpr, *syntax.MatchType, *syntax.Block, *syntax.ProjectList, *syntax.ProjectMap:
		return true
	case *syntax.StringLit:
		return n.Multiline
	case *syntax.RawStringLit:
		return n.Multiline
	default:
		return false
	}
}
