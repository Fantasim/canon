package syntax

import "math/big"

// IdentExpr is a name in value position; `it`, `fail` and `warn` are names too (GRAMMAR.md §4.2).
type IdentExpr struct {
	Bounds
	Name string
}

func (*IdentExpr) Kind() NodeKind                { return KindIdentExpr }
func (*IdentExpr) exprNode()                     {}
func (*IdentExpr) children(func(Node) bool) bool { return true }

// IntLit is an integer literal with its exact value; a folded unary "-" is part of it.
type IntLit struct {
	Bounds
	Value *big.Int
}

func (*IntLit) Kind() NodeKind                { return KindIntLit }
func (*IntLit) exprNode()                     {}
func (*IntLit) annValueNode()                 {}
func (*IntLit) projectValueNode()             {}
func (*IntLit) entryKeyNode()                 {}
func (*IntLit) children(func(Node) bool) bool { return true }

// FloatLit is a float literal with its exact decimal value, Coef × 10^Exp.
type FloatLit struct {
	Bounds
	Coef *big.Int
	Exp  int
}

func (*FloatLit) Kind() NodeKind                { return KindFloatLit }
func (*FloatLit) exprNode()                     {}
func (*FloatLit) annValueNode()                 {}
func (*FloatLit) children(func(Node) bool) bool { return true }

// DurationLit is a duration literal in milliseconds (GRAMMAR.md §2.5).
type DurationLit struct {
	Bounds
	Millis int64
}

func (*DurationLit) Kind() NodeKind                { return KindDurationLit }
func (*DurationLit) exprNode()                     {}
func (*DurationLit) annValueNode()                 {}
func (*DurationLit) children(func(Node) bool) bool { return true }

// StringLit is a plain or multiline string with its parts, escapes decoded (GRAMMAR.md §2.6).
type StringLit struct {
	Bounds
	Multiline bool
	Parts     []StringPart
}

// StringPart is a text part (Interp nil) or an interpolation.
type StringPart struct {
	Text   string
	Interp *Interp
}

func (*StringLit) Kind() NodeKind    { return KindStringLit }
func (*StringLit) exprNode()         {}
func (*StringLit) strLitNode()       {}
func (*StringLit) nameLitNode()      {}
func (*StringLit) annValueNode()     {}
func (*StringLit) projectValueNode() {}

func (n *StringLit) children(yield func(Node) bool) bool {
	for _, p := range n.Parts {
		if !visit(yield, p.Interp) {
			return false
		}
	}
	return true
}

// Interp is "{expr[:spec]}" inside a string.
type Interp struct {
	Bounds
	X    Expr
	Spec *FormatSpec
}

func (*Interp) Kind() NodeKind { return KindInterp }

func (n *Interp) children(yield func(Node) bool) bool { return visit(yield, n.X) }

// RawStringLit is a raw string, plain or multiline, with its value.
type RawStringLit struct {
	Bounds
	Multiline bool
	Value     string
}

func (*RawStringLit) Kind() NodeKind                { return KindRawStringLit }
func (*RawStringLit) exprNode()                     {}
func (*RawStringLit) strLitNode()                   {}
func (*RawStringLit) nameLitNode()                  {}
func (*RawStringLit) annValueNode()                 {}
func (*RawStringLit) projectValueNode()             {}
func (*RawStringLit) children(func(Node) bool) bool { return true }

// RegexLit is a regular expression; Pattern has each `\/` replaced by "/" (GRAMMAR.md §2.7).
type RegexLit struct {
	Bounds
	Pattern string
}

func (*RegexLit) Kind() NodeKind                { return KindRegexLit }
func (*RegexLit) exprNode()                     {}
func (*RegexLit) children(func(Node) bool) bool { return true }

// BoolLit is "true" or "false".
type BoolLit struct {
	Bounds
	Value bool
}

func (*BoolLit) Kind() NodeKind                { return KindBoolLit }
func (*BoolLit) exprNode()                     {}
func (*BoolLit) annValueNode()                 {}
func (*BoolLit) children(func(Node) bool) bool { return true }

// NoneLit is "none".
type NoneLit struct {
	Bounds
}

func (*NoneLit) Kind() NodeKind                { return KindNoneLit }
func (*NoneLit) exprNode()                     {}
func (*NoneLit) children(func(Node) bool) bool { return true }

// SelfExpr is "self".
type SelfExpr struct {
	Bounds
}

func (*SelfExpr) Kind() NodeKind                { return KindSelfExpr }
func (*SelfExpr) exprNode()                     {}
func (*SelfExpr) children(func(Node) bool) bool { return true }

// ParenExpr is "(expr)".
type ParenExpr struct {
	Bounds
	X Expr
}

func (*ParenExpr) Kind() NodeKind { return KindParenExpr }
func (*ParenExpr) exprNode()      {}

func (n *ParenExpr) children(yield func(Node) bool) bool { return visit(yield, n.X) }

// UnaryExpr is "-x" or "not x"; Op is the first token's kind.
type UnaryExpr struct {
	Bounds
	Op TokenKind
	X  Expr
}

func (*UnaryExpr) Kind() NodeKind { return KindUnaryExpr }
func (*UnaryExpr) exprNode()      {}

func (n *UnaryExpr) children(yield func(Node) bool) bool { return visit(yield, n.X) }

// BinaryExpr is "x op y" for the arithmetic, comparison, "in", "and", "or" and "??" operators.
type BinaryExpr struct {
	Bounds
	X     Expr
	Op    TokenKind
	OpTok Tok
	Y     Expr
}

func (*BinaryExpr) Kind() NodeKind { return KindBinaryExpr }
func (*BinaryExpr) exprNode()      {}

func (n *BinaryExpr) children(yield func(Node) bool) bool {
	return visit(yield, n.X) && visit(yield, n.Y)
}

// IsExpr is "x is case" (GRAMMAR.md §6.7).
type IsExpr struct {
	Bounds
	X      Expr
	Target *QualifiedName
}

func (*IsExpr) Kind() NodeKind { return KindIsExpr }
func (*IsExpr) exprNode()      {}

func (n *IsExpr) children(yield func(Node) bool) bool {
	return visit(yield, n.X) && visit(yield, n.Target)
}

// RangeExpr is "lo..hi" or "lo..=hi"; either bound may be nil.
type RangeExpr struct {
	Bounds
	Lo    Expr
	Op    TokenKind
	OpTok Tok
	Hi    Expr
}

func (*RangeExpr) Kind() NodeKind { return KindRangeExpr }
func (*RangeExpr) exprNode()      {}

func (n *RangeExpr) children(yield func(Node) bool) bool {
	return visit(yield, n.Lo) && visit(yield, n.Hi)
}

// SelectorExpr is "x.name", or "x?.name" when Optional; X is nil at the root of a shorthand lambda.
type SelectorExpr struct {
	Bounds
	X        Expr
	Optional bool
	Name     *Ident
}

func (*SelectorExpr) Kind() NodeKind { return KindSelectorExpr }
func (*SelectorExpr) exprNode()      {}

func (n *SelectorExpr) children(yield func(Node) bool) bool {
	return visit(yield, n.X) && visit(yield, n.Name)
}

// IndexExpr is "x[index]".
type IndexExpr struct {
	Bounds
	X        Expr
	Brackets Delims
	Index    Expr
}

func (*IndexExpr) Kind() NodeKind { return KindIndexExpr }
func (*IndexExpr) exprNode()      {}

func (n *IndexExpr) children(yield func(Node) bool) bool {
	return visit(yield, n.X) && visit(yield, n.Index)
}

// CallExpr is "fun(args)".
type CallExpr struct {
	Bounds
	Fun    Expr
	Parens Delims
	Args   []*Arg
}

func (*CallExpr) Kind() NodeKind { return KindCallExpr }
func (*CallExpr) exprNode()      {}

func (n *CallExpr) children(yield func(Node) bool) bool {
	return visit(yield, n.Fun) && visit(yield, n.Args...)
}

// Arg is "[name:] expr" in a call or load argument list.
type Arg struct {
	Bounds
	Name  *Ident
	Value Expr
}

func (*Arg) Kind() NodeKind { return KindArg }

func (n *Arg) children(yield func(Node) bool) bool {
	return visit(yield, n.Name) && visit(yield, n.Value)
}

// ForceExpr is the presence assertion "x!" (DECISIONS 17).
type ForceExpr struct {
	Bounds
	X Expr
}

func (*ForceExpr) Kind() NodeKind { return KindForceExpr }
func (*ForceExpr) exprNode()      {}

func (n *ForceExpr) children(yield func(Node) bool) bool { return visit(yield, n.X) }
