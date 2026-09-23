package syntax

// RecordDecl is "record Name[(params)] annotations { items }"; its annotations follow the name.
type RecordDecl struct {
	Bounds
	Doc         *DocComment
	Mods        *Modifiers
	Name        *Ident
	Parens      Delims
	Params      []*Param
	Annotations []*Annotation
	Body        *RecordBody
}

func (*RecordDecl) Kind() NodeKind { return KindRecordDecl }
func (*RecordDecl) declNode()      {}

func (n *RecordDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Mods) && visit(yield, n.Name) &&
		visit(yield, n.Params...) && visit(yield, n.Annotations...) && visit(yield, n.Body)
}

// RecordBody is the braced item list of a record or of a variant case.
type RecordBody struct {
	Bounds
	Items []RecordItem
}

func (*RecordBody) Kind() NodeKind { return KindRecordBody }

func (n *RecordBody) children(yield func(Node) bool) bool { return visit(yield, n.Items...) }

// FieldDecl is "name: type [= default] annotations"; Input and Env hold "input T from env S".
type FieldDecl struct {
	Bounds
	Doc         *DocComment
	Name        *Ident
	Input       Tok
	Type        Type
	Env         StrLit
	Default     Expr
	Annotations []*Annotation
}

func (*FieldDecl) Kind() NodeKind  { return KindFieldDecl }
func (*FieldDecl) recordItemNode() {}

func (n *FieldDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Name) && visit(yield, n.Type) &&
		visit(yield, n.Env) && visit(yield, n.Default) && visit(yield, n.Annotations...)
}

// EnumDecl is "enum Name [ordered] annotations { members }"; Ordered is NoTok when absent.
type EnumDecl struct {
	Bounds
	Doc         *DocComment
	Mods        *Modifiers
	Name        *Ident
	Ordered     Tok
	Annotations []*Annotation
	Braces      Delims
	Members     []*EnumMember
}

func (*EnumDecl) Kind() NodeKind { return KindEnumDecl }
func (*EnumDecl) declNode()      {}

func (n *EnumDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Mods) && visit(yield, n.Name) &&
		visit(yield, n.Annotations...) && visit(yield, n.Members...)
}

// EnumMember is "[retired] WORD [= value] annotations"; Value is a StrLit or an IntLit.
type EnumMember struct {
	Bounds
	Doc         *DocComment
	Mods        *Modifiers
	Name        *Ident
	Value       Expr
	Annotations []*Annotation
}

func (*EnumMember) Kind() NodeKind { return KindEnumMember }

func (n *EnumMember) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Mods) && visit(yield, n.Name) &&
		visit(yield, n.Value) && visit(yield, n.Annotations...)
}

// VariantDecl is "variant Name annotations { items }".
type VariantDecl struct {
	Bounds
	Doc         *DocComment
	Mods        *Modifiers
	Name        *Ident
	Annotations []*Annotation
	Braces      Delims
	Items       []VariantItem
}

func (*VariantDecl) Kind() NodeKind { return KindVariantDecl }
func (*VariantDecl) declNode()      {}

func (n *VariantDecl) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Mods) && visit(yield, n.Name) &&
		visit(yield, n.Annotations...) && visit(yield, n.Items...)
}

// VariantCase is "[retired] WORD annotations [{ items }]"; Body is nil for a case written alone.
type VariantCase struct {
	Bounds
	Doc         *DocComment
	Mods        *Modifiers
	Name        *Ident
	Annotations []*Annotation
	Body        *RecordBody
}

func (*VariantCase) Kind() NodeKind   { return KindVariantCase }
func (*VariantCase) variantItemNode() {}

func (n *VariantCase) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Mods) && visit(yield, n.Name) &&
		visit(yield, n.Annotations...) && visit(yield, n.Body)
}
