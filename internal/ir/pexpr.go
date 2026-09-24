package ir

import "github.com/fantasim/canonlang/internal/value"

// PExpr is a node of a translated body: an expression of the portable subset, typed. The
// statements of the subset are folded in: `let` is Let, `if … return` is If.
type PExpr interface {
	Type() TypeRef
}

// Op is an operator of the portable subset.
type Op uint8

// Builtin is a built-in function of the portable subset.
type Builtin uint8

// Lit is a literal of an allowed type; an enum member is a *value.Member.
type Lit struct {
	T TypeRef
	V value.Value
}

func (n *Lit) Type() TypeRef { return n.T }

// ParamRef reads the declared parameter Index.
type ParamRef struct {
	T     TypeRef
	Index int
}

func (n *ParamRef) Type() TypeRef { return n.T }

// ReadRef reads the path of self ExportFn.Reads[Index].
type ReadRef struct {
	T     TypeRef
	Index int
}

func (n *ReadRef) Type() TypeRef { return n.T }

// LocalRef reads the name a Let bound.
type LocalRef struct {
	T    TypeRef
	Name string
}

func (n *LocalRef) Type() TypeRef { return n.T }

// Unary is `-x` or `not x`.
type Unary struct {
	T  TypeRef
	Op Op
	X  PExpr
}

func (n *Unary) Type() TypeRef { return n.T }

// Binary is `x op y`; `and` and `or` short-circuit.
type Binary struct {
	T    TypeRef
	Op   Op
	X, Y PExpr
}

func (n *Binary) Type() TypeRef { return n.T }

// Call calls a built-in function.
type Call struct {
	T    TypeRef
	Fn   Builtin
	Args []PExpr
}

func (n *Call) Type() TypeRef { return n.T }

// CallFn calls a package-level export fn of the package: a translated one or a lookup function;
// a precomputed method of self is a Read instead.
type CallFn struct {
	T    TypeRef
	Fn   *ExportFn
	Args []PExpr
}

func (n *CallFn) Type() TypeRef { return n.T }

// If evaluates Then or Else as Cond says.
type If struct {
	T                TypeRef
	Cond, Then, Else PExpr
}

func (n *If) Type() TypeRef { return n.T }

// Let binds Name to Value in Body.
type Let struct {
	T           TypeRef
	Name        string
	Value, Body PExpr
}

func (n *Let) Type() TypeRef { return n.T }

// Template is a string template; each part is its text, then an optional interpolation.
type Template struct {
	T     TypeRef
	Parts []TemplatePart
}

func (n *Template) Type() TypeRef { return n.T }

// TemplatePart is literal text followed by an interpolated String, integer or enum, or nil.
type TemplatePart struct {
	Text string
	X    PExpr
}

// Coalesce is `x ?? y` on an optional read of self.
type Coalesce struct {
	T    TypeRef
	X, Y PExpr
}

func (n *Coalesce) Type() TypeRef { return n.T }

// IsCase is `x is c` on a variant-typed read of self: Case indexes the variant's cases.
type IsCase struct {
	T    TypeRef
	X    PExpr
	Case int
}

func (n *IsCase) Type() TypeRef { return n.T }

// Block is a translated body as statements, run in order in a scope of their own: a `let` binds
// for the rest of its block, an `if` whose branch does not return falls through to the next
// statement, and every path of a fn's Body ends in a ReturnStmt. It keeps the IR linear.
type Block struct {
	T     TypeRef
	Stmts []Stmt
}

func (n *Block) Type() TypeRef { return n.T }

// Stmt is a statement of a Block: *LetStmt, *IfStmt or *ReturnStmt.
type Stmt interface {
	stmtNode()
}

// LetStmt binds Name to Value for the rest of its Block.
type LetStmt struct {
	Name  string
	Value PExpr
}

func (*LetStmt) stmtNode() {}

// IfStmt runs Then when Cond holds, else Else (nil: nothing); each is a Block of its own.
type IfStmt struct {
	Cond       PExpr
	Then, Else *Block
}

func (*IfStmt) stmtNode() {}

// ReturnStmt returns X from the fn.
type ReturnStmt struct {
	X PExpr
}

func (*ReturnStmt) stmtNode() {}
