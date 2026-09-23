package syntax

// Import is "import path [as alias] [{ names }]"; a doc block before it is W1001.
type Import struct {
	Bounds
	Path   *QualifiedName
	Alias  *Ident
	Braces Delims
	Names  []*Ident
}

func (*Import) Kind() NodeKind { return KindImport }

func (n *Import) children(yield func(Node) bool) bool {
	return visit(yield, n.Path) && visit(yield, n.Alias) && visit(yield, n.Names...)
}

// ConstDecl is "const NAME = expr".
type ConstDecl struct {
	Bounds
	Doc         *DocComment
	Annotations []*Annotation
	Mods        *Modifiers
	Name        *Ident
	Value       Expr
}

func (*ConstDecl) Kind() NodeKind { return KindConstDecl }
func (*ConstDecl) declNode()      {}

func (n *ConstDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Annotations...) && visit(yield, n.Mods) &&
		visit(yield, n.Name) && visit(yield, n.Value)
}

// LetDecl is "let name [: type] = expr".
type LetDecl struct {
	Bounds
	Doc         *DocComment
	Annotations []*Annotation
	Mods        *Modifiers
	Name        *Ident
	Type        Type
	Value       Expr
}

func (*LetDecl) Kind() NodeKind { return KindLetDecl }
func (*LetDecl) declNode()      {}

func (n *LetDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Annotations...) && visit(yield, n.Mods) &&
		visit(yield, n.Name) && visit(yield, n.Type) && visit(yield, n.Value)
}

// TypeDecl is "type Name[(params)] = type".
type TypeDecl struct {
	Bounds
	Doc         *DocComment
	Annotations []*Annotation
	Mods        *Modifiers
	Name        *Ident
	Parens      Delims
	Params      []*Param
	Type        Type
}

func (*TypeDecl) Kind() NodeKind { return KindTypeDecl }
func (*TypeDecl) declNode()      {}

func (n *TypeDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Annotations...) && visit(yield, n.Mods) &&
		visit(yield, n.Name) && visit(yield, n.Params...) && visit(yield, n.Type)
}

// Param is "name: type [= default]": a type, function or widget parameter.
type Param struct {
	Bounds
	Name    *Ident
	Type    Type
	Default Expr
}

func (*Param) Kind() NodeKind { return KindParam }

func (n *Param) children(yield func(Node) bool) bool {
	return visit(yield, n.Name) && visit(yield, n.Type) && visit(yield, n.Default)
}

// FnDecl is a function or method; Self is the "self" receiver token, or NoTok.
type FnDecl struct {
	Bounds
	Doc         *DocComment
	Annotations []*Annotation
	Mods        *Modifiers
	Name        *Ident
	Parens      Delims
	Self        Tok
	Params      []*Param
	Result      Type
	Body        *Block
}

func (*FnDecl) Kind() NodeKind   { return KindFnDecl }
func (*FnDecl) declNode()        {}
func (*FnDecl) recordItemNode()  {}
func (*FnDecl) variantItemNode() {}

func (n *FnDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Annotations...) && visit(yield, n.Mods) &&
		visit(yield, n.Name) && visit(yield, n.Params...) && visit(yield, n.Result) &&
		visit(yield, n.Body)
}

// EntryDecl is "[retired] entry table.KEY { … }".
type EntryDecl struct {
	Bounds
	Doc         *DocComment
	Annotations []*Annotation
	Mods        *Modifiers
	Table       *Ident
	Key         EntryKey
	Value       *BraceLit
}

func (*EntryDecl) Kind() NodeKind { return KindEntryDecl }
func (*EntryDecl) declNode()      {}

func (n *EntryDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Annotations...) && visit(yield, n.Mods) &&
		visit(yield, n.Table) && visit(yield, n.Key) && visit(yield, n.Value)
}

// CheckDecl is a check or warn (Keyword): a Body block, or "[name:] cond [at field] else message".
type CheckDecl struct {
	Bounds
	Doc         *DocComment
	Annotations []*Annotation
	Keyword     TokenKind
	Body        *Block
	Name        *Ident
	Cond        Expr
	At          *Ident
	Message     StrLit
}

func (*CheckDecl) Kind() NodeKind   { return KindCheckDecl }
func (*CheckDecl) declNode()        {}
func (*CheckDecl) recordItemNode()  {}
func (*CheckDecl) variantItemNode() {}

func (n *CheckDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Annotations...) && visit(yield, n.Body) &&
		visit(yield, n.Name) && visit(yield, n.Cond) && visit(yield, n.At) && visit(yield, n.Message)
}

// ViewDecl is "view Type[.case] { items }".
type ViewDecl struct {
	Bounds
	Doc         *DocComment
	Annotations []*Annotation
	Type        *Ident
	Case        *Ident
	Braces      Delims
	Items       []ViewItem
}

func (*ViewDecl) Kind() NodeKind { return KindViewDecl }
func (*ViewDecl) declNode()      {}

func (n *ViewDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Annotations...) && visit(yield, n.Type) &&
		visit(yield, n.Case) && visit(yield, n.Items...)
}

// WidgetDecl is "widget name(value: T[, siblings: U]) [default]"; Default is NoTok when absent.
type WidgetDecl struct {
	Bounds
	Doc         *DocComment
	Annotations []*Annotation
	Name        *Ident
	Parens      Delims
	Params      []*Param
	Default     Tok
}

func (*WidgetDecl) Kind() NodeKind { return KindWidgetDecl }
func (*WidgetDecl) declNode()      {}

func (n *WidgetDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Annotations...) && visit(yield, n.Name) &&
		visit(yield, n.Params...)
}

// TestDecl is `test "name" { … }`.
type TestDecl struct {
	Bounds
	Doc         *DocComment
	Annotations []*Annotation
	Name        StrLit
	Body        *Block
}

func (*TestDecl) Kind() NodeKind { return KindTestDecl }
func (*TestDecl) declNode()      {}

func (n *TestDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Annotations...) && visit(yield, n.Name) &&
		visit(yield, n.Body)
}

// EmitDecl is "emit target { options }".
type EmitDecl struct {
	Bounds
	Doc         *DocComment
	Annotations []*Annotation
	Target      *Ident
	Options     *BraceLit
}

func (*EmitDecl) Kind() NodeKind { return KindEmitDecl }
func (*EmitDecl) declNode()      {}

func (n *EmitDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Annotations...) && visit(yield, n.Target) &&
		visit(yield, n.Options)
}
