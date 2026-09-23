package syntax

// ViewTitle is `title "text"`.
type ViewTitle struct {
	Bounds
	Doc  *DocComment
	Text StrLit
}

func (*ViewTitle) Kind() NodeKind { return KindViewTitle }
func (*ViewTitle) viewItemNode()  {}

func (n *ViewTitle) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Text)
}

// ViewSubtitle is `subtitle "text"`.
type ViewSubtitle struct {
	Bounds
	Doc  *DocComment
	Text StrLit
}

func (*ViewSubtitle) Kind() NodeKind { return KindViewSubtitle }
func (*ViewSubtitle) viewItemNode()  {}

func (n *ViewSubtitle) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Text)
}

// ViewSingular is `singular "text"`.
type ViewSingular struct {
	Bounds
	Doc  *DocComment
	Text StrLit
}

func (*ViewSingular) Kind() NodeKind { return KindViewSingular }
func (*ViewSingular) viewItemNode()  {}

func (n *ViewSingular) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Text)
}

// ViewPlural is `plural "text"`.
type ViewPlural struct {
	Bounds
	Doc  *DocComment
	Text StrLit
}

func (*ViewPlural) Kind() NodeKind { return KindViewPlural }
func (*ViewPlural) viewItemNode()  {}

func (n *ViewPlural) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Text)
}

// ViewMenu is "menu WORD icon WORD".
type ViewMenu struct {
	Bounds
	Doc  *DocComment
	Menu *Ident
	Icon *Ident
}

func (*ViewMenu) Kind() NodeKind { return KindViewMenu }
func (*ViewMenu) viewItemNode()  {}

func (n *ViewMenu) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Menu) && visit(yield, n.Icon)
}

// ViewColumns is "columns { items }".
type ViewColumns struct {
	Bounds
	Doc    *DocComment
	Braces Delims
	Items  []*ViewColumn
}

func (*ViewColumns) Kind() NodeKind { return KindViewColumns }
func (*ViewColumns) viewItemNode()  {}

func (n *ViewColumns) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Items...)
}

// ViewColumn is "WORD [width]".
type ViewColumn struct {
	Bounds
	Name  *Ident
	Width *IntLit
}

func (*ViewColumn) Kind() NodeKind { return KindViewColumn }

func (n *ViewColumn) children(yield func(Node) bool) bool {
	return visit(yield, n.Name) && visit(yield, n.Width)
}

// ViewSearch is "search { exprs }".
type ViewSearch struct {
	Bounds
	Doc    *DocComment
	Braces Delims
	Items  []Expr
}

func (*ViewSearch) Kind() NodeKind { return KindViewSearch }
func (*ViewSearch) viewItemNode()  {}

func (n *ViewSearch) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Items...)
}

// ViewFilters is "filters { items }".
type ViewFilters struct {
	Bounds
	Doc    *DocComment
	Braces Delims
	Items  []*ViewFilter
}

func (*ViewFilters) Kind() NodeKind { return KindViewFilters }
func (*ViewFilters) viewItemNode()  {}

func (n *ViewFilters) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Items...)
}

// ViewFilter is "WORD [multi]"; Multi is NoTok when absent.
type ViewFilter struct {
	Bounds
	Name  *Ident
	Multi Tok
}

func (*ViewFilter) Kind() NodeKind { return KindViewFilter }

func (n *ViewFilter) children(yield func(Node) bool) bool { return visit(yield, n.Name) }

// ViewPreview is "preview expr".
type ViewPreview struct {
	Bounds
	Doc *DocComment
	X   Expr
}

func (*ViewPreview) Kind() NodeKind { return KindViewPreview }
func (*ViewPreview) viewItemNode()  {}

func (n *ViewPreview) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.X)
}

// ViewShow is `show [id] "label" "template"`.
type ViewShow struct {
	Bounds
	Doc      *DocComment
	ID       *Ident
	Label    StrLit
	Template StrLit
}

func (*ViewShow) Kind() NodeKind   { return KindViewShow }
func (*ViewShow) viewItemNode()    {}
func (*ViewShow) groupMemberNode() {}

func (n *ViewShow) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.ID) && visit(yield, n.Label) && visit(yield, n.Template)
}

// ViewGroup is `group id "label" ["help"] [advanced] [when header] { members }`.
type ViewGroup struct {
	Bounds
	Doc      *DocComment
	ID       *Ident
	Label    StrLit
	Help     StrLit
	Advanced Tok
	When     Expr
	Braces   Delims
	Members  []GroupMember
}

func (*ViewGroup) Kind() NodeKind { return KindViewGroup }
func (*ViewGroup) viewItemNode()  {}

func (n *ViewGroup) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.ID) && visit(yield, n.Label) &&
		visit(yield, n.Help) && visit(yield, n.When) && visit(yield, n.Members...)
}

// ViewField is `[field] WORD ["label"] [{ props }]`; Field is the "field" keyword or NoTok.
type ViewField struct {
	Bounds
	Doc   *DocComment
	Field Tok
	Name  *Ident
	Label StrLit
	Props *BraceLit
}

func (*ViewField) Kind() NodeKind   { return KindViewField }
func (*ViewField) viewItemNode()    {}
func (*ViewField) groupMemberNode() {}

func (n *ViewField) children(yield func(Node) bool) bool {
	return visit(yield, n.Doc) && visit(yield, n.Name) && visit(yield, n.Label) && visit(yield, n.Props)
}
