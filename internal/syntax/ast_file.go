package syntax

// AmendBlock is "amend target { items }" in a layer file.
type AmendBlock struct {
	Bounds
	Doc    *DocComment
	Target *Ident
	Braces Delims
	Items  []*Amendment
}

func (*AmendBlock) Kind() NodeKind { return KindAmendBlock }

func (n *AmendBlock) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Target) && visit(yield, n.Items...)
}

// Amendment is "path: expr" in an amend block.
type Amendment struct {
	Bounds
	Doc   *DocComment
	Path  []*AmendSegment
	Value Expr
}

func (*Amendment) Kind() NodeKind { return KindAmendment }

func (n *Amendment) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Path...) && visit(yield, n.Value)
}

// AmendSegment is one segment of an amend path: a word, "[key]" or "[#position]"; one field is set.
type AmendSegment struct {
	Bounds
	Name     *Ident
	Key      Expr
	Position *IntLit
}

func (*AmendSegment) Kind() NodeKind { return KindAmendSegment }

func (n *AmendSegment) children(yield func(Node) bool) bool {
	return visit(yield, n.Name) && visit(yield, n.Key) && visit(yield, n.Position)
}

// TranslationEntry is `key.path "text"` in a translation file.
type TranslationEntry struct {
	Bounds
	Doc  *DocComment
	Key  *QualifiedName
	Text StrLit
}

func (*TranslationEntry) Kind() NodeKind { return KindTranslationEntry }

func (n *TranslationEntry) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Key) && visit(yield, n.Text)
}

// ProjectDecl is "project name { items }", the content of project.canon.
type ProjectDecl struct {
	Bounds
	Doc    *DocComment
	Name   *Ident
	Braces Delims
	Items  []*ProjectEntry
}

func (*ProjectDecl) Kind() NodeKind { return KindProjectDecl }

func (n *ProjectDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Name) && visit(yield, n.Items...)
}

// ProjectEntry is "key: value", or the sugar "key { … }" when Colon is NoTok.
type ProjectEntry struct {
	Bounds
	Doc   *DocComment
	Key   NameLit
	Colon Tok
	Value ProjectValue
}

func (*ProjectEntry) Kind() NodeKind { return KindProjectEntry }

func (n *ProjectEntry) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Key) && visit(yield, n.Value)
}

// ProjectList is a bracketed list of project values.
type ProjectList struct {
	Bounds
	Items []ProjectValue
}

func (*ProjectList) Kind() NodeKind    { return KindProjectList }
func (*ProjectList) projectValueNode() {}

func (n *ProjectList) children(yield func(Node) bool) bool { return visit(yield, n.Items...) }

// ProjectMap is a braced list of project entries.
type ProjectMap struct {
	Bounds
	Entries []*ProjectEntry
}

func (*ProjectMap) Kind() NodeKind    { return KindProjectMap }
func (*ProjectMap) projectValueNode() {}

func (n *ProjectMap) children(yield func(Node) bool) bool { return visit(yield, n.Entries...) }
