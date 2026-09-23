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

// CallFn calls an export fn of the package: a translated one, a precomputed method of self
// (no Args) or a lookup function.
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
