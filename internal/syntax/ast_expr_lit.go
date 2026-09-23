package syntax

// LambdaExpr is "x => body" or "(x, y) => body"; Parens are NoTok for a single bare binder.
type LambdaExpr struct {
	Bounds
	Parens Delims
	Params []*Ident
	Body   Expr
}

func (*LambdaExpr) Kind() NodeKind { return KindLambdaExpr }
func (*LambdaExpr) exprNode()      {}

func (n *LambdaExpr) children(yield func(Node) bool) bool {
	return visit(yield, n.Params...) && visit(yield, n.Body)
}

// ShorthandLambda is ".name{postfix}": Body is the chain, rooted at a SelectorExpr whose X is nil.
type ShorthandLambda struct {
	Bounds
	Body Expr
}

func (*ShorthandLambda) Kind() NodeKind { return KindShorthandLambda }
func (*ShorthandLambda) exprNode()      {}

func (n *ShorthandLambda) children(yield func(Node) bool) bool { return visit(yield, n.Body) }

// ListLit is "[a, b, …]".
type ListLit struct {
	Bounds
	Elems []Expr
}

func (*ListLit) Kind() NodeKind { return KindListLit }
func (*ListLit) exprNode()      {}

func (n *ListLit) children(yield func(Node) bool) bool { return visit(yield, n.Elems...) }

// ListComp is "[elem clauses]".
type ListComp struct {
	Bounds
	Elem    Expr
	Clauses []*CompClause
}

func (*ListComp) Kind() NodeKind { return KindListComp }
func (*ListComp) exprNode()      {}

func (n *ListComp) children(yield func(Node) bool) bool {
	return visit(yield, n.Elem) && visit(yield, n.Clauses...)
}

// BraceLit is every "{ … }" literal (GRAMMAR.md §5.12); with Clauses it is a comprehension.
type BraceLit struct {
	Bounds
	Items   []BraceItem
	Clauses []*CompClause
}

func (*BraceLit) Kind() NodeKind { return KindBraceLit }
func (*BraceLit) exprNode()      {}
func (*BraceLit) annValueNode()  {}

func (n *BraceLit) children(yield func(Node) bool) bool {
	return visit(yield, n.Items...) && visit(yield, n.Clauses...)
}

// FieldItem is "WORD: expr" in a brace literal.
type FieldItem struct {
	Bounds
	Name  *Ident
	Value Expr
}

func (*FieldItem) Kind() NodeKind { return KindFieldItem }
func (*FieldItem) braceItemNode() {}

func (n *FieldItem) children(yield func(Node) bool) bool {
	return visit(yield, n.Name) && visit(yield, n.Value)
}

// MapItem is "key: expr" in a brace literal, the key an expression.
type MapItem struct {
	Bounds
	Key   Expr
	Value Expr
}

func (*MapItem) Kind() NodeKind { return KindMapItem }
func (*MapItem) braceItemNode() {}

func (n *MapItem) children(yield func(Node) bool) bool {
	return visit(yield, n.Key) && visit(yield, n.Value)
}

// EntryItem is a table entry "[retired] WORD annotations { … }" in a brace literal.
type EntryItem struct {
	Bounds
	Doc         *DocComment
	Mods        *Modifiers
	Key         *Ident
	Annotations []*Annotation
	Value       *BraceLit
}

func (*EntryItem) Kind() NodeKind { return KindEntryItem }
func (*EntryItem) braceItemNode() {}

func (n *EntryItem) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Mods) && visit(yield, n.Key) &&
		visit(yield, n.Annotations...) && visit(yield, n.Value)
}

// SpreadItem is "...expr" in a brace literal.
type SpreadItem struct {
	Bounds
	X Expr
}

func (*SpreadItem) Kind() NodeKind { return KindSpreadItem }
func (*SpreadItem) braceItemNode() {}

func (n *SpreadItem) children(yield func(Node) bool) bool { return visit(yield, n.X) }

// CompClause is "for vars in X", "if X" or "let Vars[0] = X", by Keyword.
type CompClause struct {
	Bounds
	Keyword TokenKind
	Vars    []*Ident
	X       Expr
}

func (*CompClause) Kind() NodeKind { return KindCompClause }

func (n *CompClause) children(yield func(Node) bool) bool {
	return visit(yield, n.Vars...) && visit(yield, n.X)
}

// TypedLit is "Name { … }", the brace on the name's line.
type TypedLit struct {
	Bounds
	Type *QualifiedName
	Lit  *BraceLit
}

func (*TypedLit) Kind() NodeKind { return KindTypedLit }
func (*TypedLit) exprNode()      {}

func (n *TypedLit) children(yield func(Node) bool) bool {
	return visit(yield, n.Type) && visit(yield, n.Lit)
}

// IfExpr is "if c { a } else …": exactly one of ElseIf and Else is set.
type IfExpr struct {
	Bounds
	Cond   Expr
	Then   *ExprBody
	ElseIf *IfExpr
	Else   *ExprBody
}

func (*IfExpr) Kind() NodeKind { return KindIfExpr }
func (*IfExpr) exprNode()      {}

func (n *IfExpr) children(yield func(Node) bool) bool {
	return visit(yield, n.Cond) && visit(yield, n.Then) && visit(yield, n.ElseIf) && visit(yield, n.Else)
}

// ExprBody is "{ expr }", a branch of an if expression.
type ExprBody struct {
	Bounds
	X Expr
}

func (*ExprBody) Kind() NodeKind { return KindExprBody }

func (n *ExprBody) children(yield func(Node) bool) bool { return visit(yield, n.X) }

// MatchExpr is "match header { arms }" in value position.
type MatchExpr struct {
	Bounds
	Scrutinee Expr
	Braces    Delims
	Arms      []*MatchArm
}

func (*MatchExpr) Kind() NodeKind { return KindMatchExpr }
func (*MatchExpr) exprNode()      {}

func (n *MatchExpr) children(yield func(Node) bool) bool {
	return visit(yield, n.Scrutinee) && visit(yield, n.Arms...)
}

// MatchArm is "patterns => expr".
type MatchArm struct {
	Bounds
	Patterns []*Pattern
	Body     Expr
}

func (*MatchArm) Kind() NodeKind { return KindMatchArm }

func (n *MatchArm) children(yield func(Node) bool) bool {
	return visit(yield, n.Patterns...) && visit(yield, n.Body)
}

// Pattern is "_" or "none" (Keyword TokUnderscore, KwNone), else "Name[(binder)]" (TokInvalid).
type Pattern struct {
	Bounds
	Keyword TokenKind
	Name    *QualifiedName
	Parens  Delims
	Binder  *Ident
}

func (*Pattern) Kind() NodeKind { return KindPattern }

func (n *Pattern) children(yield func(Node) bool) bool {
	return visit(yield, n.Name) && visit(yield, n.Binder)
}

// LoadExpr is "load[.method](args)"; Method is nil for a plain load.
type LoadExpr struct {
	Bounds
	Method *Ident
	Parens Delims
	Args   []*Arg
}

func (*LoadExpr) Kind() NodeKind { return KindLoadExpr }
func (*LoadExpr) exprNode()      {}

func (n *LoadExpr) children(yield func(Node) bool) bool {
	return visit(yield, n.Method) && visit(yield, n.Args...)
}
