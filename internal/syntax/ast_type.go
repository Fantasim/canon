package syntax

// NamedType is "[past] Name[(args)]", Past NoTok when absent; check reads args (GRAMMAR.md §6.6).
type NamedType struct {
	Bounds
	Past Tok
	Name *QualifiedName
	Args *TypeArgs
}

func (*NamedType) Kind() NodeKind { return KindNamedType }
func (*NamedType) typeNode()      {}

func (n *NamedType) children(yield func(Node) bool) bool {
	return visit(yield, n.Name) && visit(yield, n.Args)
}

// TypeArgs is the parenthesized argument list of a type: a regex or expressions.
type TypeArgs struct {
	Bounds
	Args []Expr
}

func (*TypeArgs) Kind() NodeKind { return KindTypeArgs }

func (n *TypeArgs) children(yield func(Node) bool) bool { return visit(yield, n.Args...) }

// ListType is "[T][(args)]".
type ListType struct {
	Bounds
	Past Tok
	Elem Type
	Args *TypeArgs
}

func (*ListType) Kind() NodeKind { return KindListType }
func (*ListType) typeNode()      {}

func (n *ListType) children(yield func(Node) bool) bool {
	return visit(yield, n.Elem) && visit(yield, n.Args)
}

// KeyedType is "T keyed by field"; E1137 when T is not a ListType.
type KeyedType struct {
	Bounds
	List Type
	Key  *Ident
}

func (*KeyedType) Kind() NodeKind { return KindKeyedType }
func (*KeyedType) typeNode()      {}

func (n *KeyedType) children(yield func(Node) bool) bool {
	return visit(yield, n.List) && visit(yield, n.Key)
}

// MapType is "{K: V}[(args)]".
type MapType struct {
	Bounds
	Key   Type
	Value Type
	Args  *TypeArgs
}

func (*MapType) Kind() NodeKind { return KindMapType }
func (*MapType) typeNode()      {}

func (n *MapType) children(yield func(Node) bool) bool {
	return visit(yield, n.Key) && visit(yield, n.Value) && visit(yield, n.Args)
}

// DepMapType is "{v in domain: T}[(args)]".
type DepMapType struct {
	Bounds
	Var    *Ident
	Domain Expr
	Value  Type
	Args   *TypeArgs
}

func (*DepMapType) Kind() NodeKind { return KindDepMapType }
func (*DepMapType) typeNode()      {}

func (n *DepMapType) children(yield func(Node) bool) bool {
	return visit(yield, n.Var) && visit(yield, n.Domain) && visit(yield, n.Value) && visit(yield, n.Args)
}

// TableType is "[stable] table Name"; Stable is NoTok when absent.
type TableType struct {
	Bounds
	Past   Tok
	Stable Tok
	Name   *QualifiedName
}

func (*TableType) Kind() NodeKind { return KindTableType }
func (*TableType) typeNode()      {}

func (n *TableType) children(yield func(Node) bool) bool { return visit(yield, n.Name) }

// RefType is "[past] ref name"; Past is NoTok when absent (GRAMMAR.md §5.9 pastType).
type RefType struct {
	Bounds
	Past Tok
	Name *QualifiedName
}

func (*RefType) Kind() NodeKind { return KindRefType }
func (*RefType) typeNode()      {}

func (n *RefType) children(yield func(Node) bool) bool { return visit(yield, n.Name) }

// OptionalType is "T?".
type OptionalType struct {
	Bounds
	Elem Type
}

func (*OptionalType) Kind() NodeKind { return KindOptionalType }
func (*OptionalType) typeNode()      {}

func (n *OptionalType) children(yield func(Node) bool) bool { return visit(yield, n.Elem) }

// WhereType is "T where predicate".
type WhereType struct {
	Bounds
	Base Type
	Pred Expr
}

func (*WhereType) Kind() NodeKind { return KindWhereType }
func (*WhereType) typeNode()      {}

func (n *WhereType) children(yield func(Node) bool) bool {
	return visit(yield, n.Base) && visit(yield, n.Pred)
}

// UnionType is "A | B | …".
type UnionType struct {
	Bounds
	Alts []Type
}

func (*UnionType) Kind() NodeKind { return KindUnionType }
func (*UnionType) typeNode()      {}

func (n *UnionType) children(yield func(Node) bool) bool { return visit(yield, n.Alts...) }

// LiteralType is a string literal written as a type alternative.
type LiteralType struct {
	Bounds
	Past  Tok
	Value StrLit
}

func (*LiteralType) Kind() NodeKind { return KindLiteralType }
func (*LiteralType) typeNode()      {}

func (n *LiteralType) children(yield func(Node) bool) bool { return visit(yield, n.Value) }

// MatchType is "match header { arms }" in type position.
type MatchType struct {
	Bounds
	Past      Tok
	Scrutinee Expr
	Braces    Delims
	Arms      []*TypeArm
}

func (*MatchType) Kind() NodeKind { return KindMatchType }
func (*MatchType) typeNode()      {}

func (n *MatchType) children(yield func(Node) bool) bool {
	return visit(yield, n.Scrutinee) && visit(yield, n.Arms...)
}

// TypeArm is "patterns => type".
type TypeArm struct {
	Bounds
	Patterns []*Pattern
	Type     Type
}

func (*TypeArm) Kind() NodeKind { return KindTypeArm }

func (n *TypeArm) children(yield func(Node) bool) bool {
	return visit(yield, n.Patterns...) && visit(yield, n.Type)
}

// AssetType is `asset("dir"[, ext: [names]])`.
type AssetType struct {
	Bounds
	Past     Tok
	Parens   Delims
	Dir      StrLit
	Brackets Delims
	Exts     []NameLit
}

func (*AssetType) Kind() NodeKind { return KindAssetType }
func (*AssetType) typeNode()      {}

func (n *AssetType) children(yield func(Node) bool) bool {
	return visit(yield, n.Dir) && visit(yield, n.Exts...)
}

// AnyType is "_" in type position.
type AnyType struct {
	Bounds
	Past Tok
}

func (*AnyType) Kind() NodeKind                { return KindAnyType }
func (*AnyType) typeNode()                     {}
func (*AnyType) children(func(Node) bool) bool { return true }

// FnType is "fn(T, …) -> R".
type FnType struct {
	Bounds
	Past   Tok
	Parens Delims
	Params []Type
	Result Type
}

func (*FnType) Kind() NodeKind { return KindFnType }
func (*FnType) typeNode()      {}

func (n *FnType) children(yield func(Node) bool) bool {
	return visit(yield, n.Params...) && visit(yield, n.Result)
}

// ParenType is "(T)"; the formatter never adds or removes parentheses.
type ParenType struct {
	Bounds
	Type Type
}

func (*ParenType) Kind() NodeKind { return KindParenType }
func (*ParenType) typeNode()      {}

func (n *ParenType) children(yield func(Node) bool) bool { return visit(yield, n.Type) }
