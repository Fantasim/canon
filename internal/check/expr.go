package check

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// expr checks e in env, against want when it is not nil (check mode) or alone (synthesize mode, TYPES.md §1).
func (c *checker) expr(env *env, e syntax.Expr, want types.Type) types.Type {
	if p, ok := e.(*syntax.ParenExpr); ok {
		t := c.expr(env, p.X, want)
		c.info.Types[e] = t
		return t
	}
	if env.joining() && !joinable(e) {
		env = env.noJoin()
	}
	t := c.exprNode(env, e, want)
	if t == nil {
		t = types.ErrorType
	}
	c.info.Types[e] = t
	if want != nil {
		c.accept(env, e, t, want)
	}
	return t
}

// sourceText is an expression as written, printed where a message wants a type it has none of.
type sourceText string

func (s sourceText) String() string { return string(s) }

// literalText is the source of a literal for a message: its text on one short line, else `{ … }`.
func (env *env) literalText(n syntax.Node) sourceText {
	sp := env.span(n)
	text := string(env.file.Src.Content[sp.Start:sp.End])
	if len(text) > maxLiteralText || strings.Contains(text, newline) {
		return elidedLiteral
	}
	return sourceText(text)
}

// joinable is a branch a join types: `none`, `[]`, `{}`, an if or a match (TYPES.md §6.4).
func joinable(e syntax.Expr) bool {
	switch x := e.(type) {
	case *syntax.NoneLit, *syntax.IfExpr, *syntax.MatchExpr:
		return true
	case *syntax.ListLit:
		return len(x.Elems) == 0
	case *syntax.BraceLit:
		return len(x.Items) == 0 && len(x.Clauses) == 0
	}
	return false
}

// synth synthesizes e: expr without an expected type.
func (c *checker) synth(env *env, e syntax.Expr) types.Type { return c.expr(env, e, nil) }

func (c *checker) exprNode(env *env, e syntax.Expr, want types.Type) types.Type {
	if env.what != noConstant {
		if bad := c.notConstant(env, e); bad {
			return types.ErrorType
		}
	}
	switch e := e.(type) {
	case *syntax.IdentExpr:
		return c.ident(env, e, want)
	case *syntax.SelectorExpr, *syntax.IndexExpr, *syntax.CallExpr, *syntax.ForceExpr:
		return c.chainTop(env, e, want)
	case *syntax.UnaryExpr:
		return c.unary(env, e, want)
	case *syntax.BinaryExpr:
		return c.binary(env, e, want)
	case *syntax.IsExpr:
		return c.isExpr(env, e)
	case *syntax.RangeExpr:
		return c.rangeExpr(env, e)
	case *syntax.LambdaExpr:
		return c.lambda(env, e, want)
	case *syntax.ShorthandLambda:
		return c.shorthand(env, e, want)
	}
	return c.literalNode(env, e, want)
}

// literalNode types literals, collection literals and the other primaries.
func (c *checker) literalNode(env *env, e syntax.Expr, want types.Type) types.Type {
	if leafLiteral(e) && c.lexError(env, e) {
		if s, ok := e.(*syntax.StringLit); ok {
			c.interpolations(env, s)
		}
		return types.ErrorType
	}
	switch e := e.(type) {
	case *syntax.IntLit:
		return c.intLit(e, want)
	case *syntax.FloatLit:
		return types.FloatType
	case *syntax.DurationLit:
		return types.DurationType
	case *syntax.StringLit:
		return c.stringLit(env, e, want)
	case *syntax.RawStringLit:
		return c.rawString(env, e, want)
	case *syntax.BoolLit:
		return types.BoolType
	case *syntax.NoneLit:
		return c.noneLit(env, e, want)
	case *syntax.SelfExpr:
		return c.self(env, e)
	case *syntax.RegexLit, *syntax.BadExpr:
		c.breakObj(env.owner)
		return types.ErrorType
	}
	return c.compositeNode(env, e, want)
}

func (c *checker) compositeNode(env *env, e syntax.Expr, want types.Type) types.Type {
	switch e := e.(type) {
	case *syntax.ListLit:
		return c.listLit(env, e, want)
	case *syntax.ListComp:
		return c.listComp(env, e, want)
	case *syntax.BraceLit:
		return c.braceLit(env, e, want)
	case *syntax.TypedLit:
		return c.typedLit(env, e, want)
	case *syntax.IfExpr:
		return c.ifExpr(env, e, want)
	case *syntax.MatchExpr:
		return c.matchExpr(env, e, want)
	case *syntax.LoadExpr:
		return c.load(env, e, want)
	}
	return types.ErrorType
}

// cmpType is want, or Error when there is none, for a message that needs an expected type.
func cmpType(want types.Type) types.Type {
	if want == nil {
		return types.ErrorType
	}
	return want
}

// intLit is an integer literal: Int, or a key when the expected type is a ref (TYPES.md §4.1).
func (c *checker) intLit(e *syntax.IntLit, want types.Type) types.Type {
	if coll := c.keyTarget(want); coll != nil && keyedByInt(coll) {
		c.info.Keys[e] = coll
		return refTo(want)
	}
	return types.IntType
}

// stringLit is a string, a key of a ref or a union's literal; interpolations checked (STDLIB.md §9).
func (c *checker) stringLit(env *env, e *syntax.StringLit, want types.Type) types.Type {
	c.interpolations(env, e)
	return c.stringValue(e, want)
}

func (c *checker) rawString(_ *env, e *syntax.RawStringLit, want types.Type) types.Type {
	return c.stringValue(e, want)
}

// stringValue types a string literal against want: a union's literal first (TYPES.md §13.2), then a key.
func (c *checker) stringValue(e syntax.Expr, want types.Type) types.Type {
	if !interpolationFree(e) {
		return types.StringType
	}
	if u, ok := unwrap(want).(*types.LitUnionType); ok && slices.Contains(u.Literals, constText(e.(syntax.StrLit))) {
		return want
	}
	if coll := c.keyTarget(want); coll != nil && !keyedByInt(coll) {
		c.info.Keys[e] = coll
		return refTo(want)
	}
	return types.StringType
}

func interpolationFree(e syntax.Expr) bool {
	s, ok := e.(*syntax.StringLit)
	if !ok {
		return true
	}
	for _, p := range s.Parts {
		if p.Interp != nil {
			return false
		}
	}
	return true
}

// noneLit is `none` (TYPES.md §5.3): None, and E3008 when synthesized alone.
func (c *checker) noneLit(env *env, e *syntax.NoneLit, want types.Type) types.Type {
	if want == nil && !env.joining() {
		c.report(env, diag.E3008.At(env.span(e)))
		return types.ErrorType
	}
	return types.NoneType
}

// self is the record or case value of the body (TYPES.md §8.1); E2108 elsewhere.
func (c *checker) self(env *env, e *syntax.SelfExpr) types.Type {
	if env.rec == nil {
		c.report(env, diag.E2108.At(env.span(e)))
		return types.ErrorType
	}
	return c.narrowed(env, e, env.rec.self)
}

// unwrap is an expected type without aliases, refinements and one optional (TYPES.md §4.1, §5.2).
func unwrap(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	b := t.Base()
	if o, ok := b.(*types.OptionalType); ok {
		return o.Elem.Base()
	}
	return b
}

// keyTarget is the collection of an expected ref (or a union over one), nil otherwise.
func (c *checker) keyTarget(want types.Type) *types.Collection {
	switch x := unwrap(want).(type) {
	case *types.RefType:
		return c.coll(x)
	case *types.LitUnionType:
		return c.keyTarget(x.Of)
	}
	return nil
}

// refTo is the ref type within an expected type.
func refTo(want types.Type) types.Type {
	switch x := unwrap(want).(type) {
	case *types.RefType:
		return x
	case *types.LitUnionType:
		return refTo(x.Of)
	}
	return want
}

// keyedByInt reports a keyed list whose key field is an integer type.
func keyedByInt(coll *types.Collection) bool {
	return coll.KeyedBy != nil && coll.KeyedBy.Type.Base().Kind() == types.Int
}
