package syntax

// Block is "{ statements }": a function, check or test body, or a statement block.
type Block struct {
	Bounds
	Stmts []Stmt
}

func (*Block) Kind() NodeKind { return KindBlock }

func (n *Block) children(yield func(Node) bool) bool { return visit(yield, n.Stmts...) }

// LetStmt is "let name [: type] = expr" in a block.
type LetStmt struct {
	Bounds
	Name  *Ident
	Type  Type
	Value Expr
}

func (*LetStmt) Kind() NodeKind { return KindLetStmt }
func (*LetStmt) stmtNode()      {}

func (n *LetStmt) children(yield func(Node) bool) bool {
	return visit(yield, n.Name) && visit(yield, n.Type) && visit(yield, n.Value)
}

// VarStmt is "var name [: type] = expr".
type VarStmt struct {
	Bounds
	Name  *Ident
	Type  Type
	Value Expr
}

func (*VarStmt) Kind() NodeKind { return KindVarStmt }
func (*VarStmt) stmtNode()      {}

func (n *VarStmt) children(yield func(Node) bool) bool {
	return visit(yield, n.Name) && visit(yield, n.Type) && visit(yield, n.Value)
}

// AssignStmt is "target op value" for "=", "+=", "-=", "*=" and "/=".
type AssignStmt struct {
	Bounds
	Target Expr
	Op     TokenKind
	OpTok  Tok
	Value  Expr
}

func (*AssignStmt) Kind() NodeKind { return KindAssignStmt }
func (*AssignStmt) stmtNode()      {}

func (n *AssignStmt) children(yield func(Node) bool) bool {
	return visit(yield, n.Target) && visit(yield, n.Value)
}

// IfStmt is "if c { … } [else …]": at most one of ElseIf and Else is set.
type IfStmt struct {
	Bounds
	Cond   Expr
	Then   *Block
	ElseIf *IfStmt
	Else   *Block
}

func (*IfStmt) Kind() NodeKind { return KindIfStmt }
func (*IfStmt) stmtNode()      {}

func (n *IfStmt) children(yield func(Node) bool) bool {
	return visit(yield, n.Cond) && visit(yield, n.Then) && visit(yield, n.ElseIf) && visit(yield, n.Else)
}

// ForStmt is "for a[, b] in iter { … }".
type ForStmt struct {
	Bounds
	Vars []*Ident
	Iter Expr
	Body *Block
}

func (*ForStmt) Kind() NodeKind { return KindForStmt }
func (*ForStmt) stmtNode()      {}

func (n *ForStmt) children(yield func(Node) bool) bool {
	return visit(yield, n.Vars...) && visit(yield, n.Iter) && visit(yield, n.Body)
}

// WhileStmt is "while c { … }".
type WhileStmt struct {
	Bounds
	Cond Expr
	Body *Block
}

func (*WhileStmt) Kind() NodeKind { return KindWhileStmt }
func (*WhileStmt) stmtNode()      {}

func (n *WhileStmt) children(yield func(Node) bool) bool {
	return visit(yield, n.Cond) && visit(yield, n.Body)
}

// BreakStmt is "break".
type BreakStmt struct {
	Bounds
}

func (*BreakStmt) Kind() NodeKind                { return KindBreakStmt }
func (*BreakStmt) stmtNode()                     {}
func (*BreakStmt) children(func(Node) bool) bool { return true }

// ContinueStmt is "continue".
type ContinueStmt struct {
	Bounds
}

func (*ContinueStmt) Kind() NodeKind                { return KindContinueStmt }
func (*ContinueStmt) stmtNode()                     {}
func (*ContinueStmt) children(func(Node) bool) bool { return true }

// ReturnStmt is "return [expr]".
type ReturnStmt struct {
	Bounds
	Value Expr
}

func (*ReturnStmt) Kind() NodeKind { return KindReturnStmt }
func (*ReturnStmt) stmtNode()      {}

func (n *ReturnStmt) children(yield func(Node) bool) bool { return visit(yield, n.Value) }

// ExpectStmt is "expect x [fails|warns message|passes]"; Outcome is nil for a bare expect.
type ExpectStmt struct {
	Bounds
	X       Expr
	Outcome *Ident
	Message NameLit
}

func (*ExpectStmt) Kind() NodeKind { return KindExpectStmt }
func (*ExpectStmt) stmtNode()      {}

func (n *ExpectStmt) children(yield func(Node) bool) bool {
	return visit(yield, n.X) && visit(yield, n.Outcome) && visit(yield, n.Message)
}

// MatchStmt is "match header { arms }" at the start of a statement.
type MatchStmt struct {
	Bounds
	Scrutinee Expr
	Braces    Delims
	Arms      []*StmtArm
}

func (*MatchStmt) Kind() NodeKind { return KindMatchStmt }
func (*MatchStmt) stmtNode()      {}

func (n *MatchStmt) children(yield func(Node) bool) bool {
	return visit(yield, n.Scrutinee) && visit(yield, n.Arms...)
}

// StmtArm is "patterns => block" or "patterns => expr": exactly one of Block and X is set.
type StmtArm struct {
	Bounds
	Patterns []*Pattern
	Block    *Block
	X        Expr
}

func (*StmtArm) Kind() NodeKind { return KindStmtArm }

func (n *StmtArm) children(yield func(Node) bool) bool {
	return visit(yield, n.Patterns...) && visit(yield, n.Block) && visit(yield, n.X)
}

// ExprStmt is an expression used as a statement.
type ExprStmt struct {
	Bounds
	X Expr
}

func (*ExprStmt) Kind() NodeKind { return KindExprStmt }
func (*ExprStmt) stmtNode()      {}

func (n *ExprStmt) children(yield func(Node) bool) bool { return visit(yield, n.X) }
