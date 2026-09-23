package syntax

import "github.com/fantasim/canonlang/internal/source"

// Ident is a name: an IDENT, a reserved word where GRAMMAR.md §4.3 allows one, or "_" (Blank).
type Ident struct {
	Bounds
	Name string
}

func (*Ident) Kind() NodeKind { return KindIdent }

func (*Ident) children(func(Node) bool) bool { return true }
func (*Ident) nameLitNode()                  {}
func (*Ident) entryKeyNode()                 {}

// QualifiedName is a dotted name: a package path, a translation key, a symbol or a type name.
type QualifiedName struct {
	Bounds
	Parts []*Ident
}

func (*QualifiedName) Kind() NodeKind { return KindQualifiedName }

func (n *QualifiedName) children(yield func(Node) bool) bool { return visit(yield, n.Parts...) }
func (*QualifiedName) annValueNode()                         {}
func (*QualifiedName) projectValueNode()                     {}

// DocComment is a doc block: leading trivia of its item's first token, which both bounds name.
type DocComment struct {
	Bounds
	Start, End source.Pos
	Text       string // normalized, GRAMMAR.md §2.2
}

func (*DocComment) Kind() NodeKind { return KindDocComment }

func (*DocComment) children(func(Node) bool) bool { return true }

// Annotation is "@name" with its optional argument list (GRAMMAR.md §5.8).
type Annotation struct {
	Bounds
	Name   *Ident
	Parens Delims
	Args   []*AnnotationArg
}

func (*Annotation) Kind() NodeKind { return KindAnnotation }

func (n *Annotation) children(yield func(Node) bool) bool {
	return visit(yield, n.Name) && visit(yield, n.Args...)
}

// AnnotationArg is one argument of an annotation; Name is nil when it is positional.
type AnnotationArg struct {
	Bounds
	Name  *Ident
	Value AnnValue
}

func (*AnnotationArg) Kind() NodeKind { return KindAnnotationArg }

func (n *AnnotationArg) children(yield func(Node) bool) bool {
	return visit(yield, n.Name) && visit(yield, n.Value)
}

// AnnotationList is a bracketed list of annotation values.
type AnnotationList struct {
	Bounds
	Items []AnnValue
}

func (*AnnotationList) Kind() NodeKind { return KindAnnotationList }

func (n *AnnotationList) children(yield func(Node) bool) bool { return visit(yield, n.Items...) }
func (*AnnotationList) annValueNode()                         {}

// Modifiers are the "local", "export" and "retired" keywords before an item; E1133 is checked
// on them, so the parser accepts any of them anywhere one is allowed.
type Modifiers struct {
	Bounds
	Local, Export, Retired Tok
}

func (*Modifiers) Kind() NodeKind { return KindModifiers }

func (*Modifiers) children(func(Node) bool) bool { return true }
