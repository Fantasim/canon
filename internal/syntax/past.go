package syntax

// pastable is a type node a pastType may hold: its Past field is the `past` token (GRAMMAR.md §5.9).
type pastable interface {
	pastSlot() *Tok
}

// PastOf is the `past` written before t, NoTok when none is (TYPES.md §8.4).
func PastOf(t Node) Tok {
	if p, ok := t.(pastable); ok {
		return *p.pastSlot()
	}
	return NoTok
}

func (n *NamedType) pastSlot() *Tok   { return &n.Past }
func (n *RefType) pastSlot() *Tok     { return &n.Past }
func (n *ListType) pastSlot() *Tok    { return &n.Past }
func (n *TableType) pastSlot() *Tok   { return &n.Past }
func (n *FnType) pastSlot() *Tok      { return &n.Past }
func (n *AssetType) pastSlot() *Tok   { return &n.Past }
func (n *MatchType) pastSlot() *Tok   { return &n.Past }
func (n *LiteralType) pastSlot() *Tok { return &n.Past }
func (n *AnyType) pastSlot() *Tok     { return &n.Past }
