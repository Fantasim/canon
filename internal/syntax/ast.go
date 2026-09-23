package syntax

import "github.com/fantasim/canonlang/internal/source"

// NodeKind is the kind of a node; every node type has its own.
type NodeKind uint8

// String is the name of the node type of kind k.
func (k NodeKind) String() string {
	if k >= NodeKindCount {
		return kindNames[KindInvalid]
	}
	return kindNames[k]
}

// Node is a node of the syntax tree: its kind and its first and last tokens, so its exact text
// and its trivia are known.
type Node interface {
	First() Tok
	Last() Tok
	Kind() NodeKind
	children(yield func(Node) bool) bool
}

// Bounds are a node's first and last tokens, inclusive.
type Bounds struct {
	From, To Tok
}

func (b Bounds) First() Tok { return b.From }
func (b Bounds) Last() Tok  { return b.To }

// Decl is a top-level declaration of a source file.
type Decl interface {
	Node
	declNode()
}

// Type is a written type.
type Type interface {
	Node
	typeNode()
}

// Expr is an expression.
type Expr interface {
	Node
	exprNode()
}

// Stmt is a statement of a block.
type Stmt interface {
	Node
	stmtNode()
}

// ViewItem is an item of a view body.
type ViewItem interface {
	Node
	viewItemNode()
}

// RecordItem is an item of a record or case body: a field, a method or a check.
type RecordItem interface {
	Node
	recordItemNode()
}

// VariantItem is an item of a variant body: a case, a method or a check.
type VariantItem interface {
	Node
	variantItemNode()
}

// GroupMember is an item of a view group: a show line or a field.
type GroupMember interface {
	ViewItem
	groupMemberNode()
}

// BraceItem is an item of a brace literal (GRAMMAR.md §5.12).
type BraceItem interface {
	Node
	braceItemNode()
}

// StrLit is a string literal: a StringLit or a RawStringLit.
type StrLit interface {
	Expr
	strLitNode()
}

// NameLit is a name written as an identifier or as a string literal.
type NameLit interface {
	Node
	nameLitNode()
}

// EntryKey is the key of an entry declaration: a word or an integer.
type EntryKey interface {
	Node
	entryKeyNode()
}

// AnnValue is an annotation argument value (GRAMMAR.md §5.8).
type AnnValue interface {
	Node
	annValueNode()
}

// ProjectValue is a value of the project file (GRAMMAR.md §7).
type ProjectValue interface {
	Node
	projectValueNode()
}

// FileKind is the kind of a Canon file.
type FileKind uint8

// File is a parsed file, the root node: its token stream and its tree; the fields its FileKind
// does not use are empty.
type File struct {
	Bounds
	Src      *source.File
	Tokens   []Token
	FileKind FileKind
	Doc      *DocComment
	Package  *QualifiedName
	Imports  []*Import
	Decls    []Decl
	Layer    *Ident
	Amends   []*AmendBlock
	Lang     *Ident
	Entries  []*TranslationEntry
	Project  *ProjectDecl
}

func (*File) Kind() NodeKind { return KindFile }

func (f *File) children(yield func(Node) bool) bool {
	return visit(yield, f.Doc) && visit(yield, f.Package) && visit(yield, f.Imports...) &&
		visit(yield, f.Decls...) && visit(yield, f.Layer) && visit(yield, f.Amends...) &&
		visit(yield, f.Lang) && visit(yield, f.Entries...) && visit(yield, f.Project)
}

// Span is the byte range of n: its first token to its last, a DocComment's own lines, or all
// of the File (its last token is EOF).
func (f *File) Span(n Node) source.Span {
	switch n := n.(type) {
	case *DocComment:
		return source.Span{File: f.Src.ID, Start: n.Start, End: n.End}
	case *File:
		return source.Span{File: f.Src.ID, Start: 0, End: f.Tokens[f.Last()].End}
	}
	return source.Span{File: f.Src.ID, Start: f.Tokens[n.First()].Start, End: f.Tokens[n.Last()].End}
}

// Leading is the trivia before n's first token: its own-line comments and doc block (FORMATTER.md §8.1).
func (f *File) Leading(n Node) []Trivia { return f.Tokens[n.First()].Leading }

// Trailing is the trivia after n's last token on its line: its trailing comment.
func (f *File) Trailing(n Node) []Trivia { return f.Tokens[n.Last()].Trailing }
