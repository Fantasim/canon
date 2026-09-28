package syntax

import "github.com/fantasim/canonlang/internal/diag"

// viewTable dispatches a view item on its vocabulary word (GRAMMAR.md §5.6, GRM-17).
var viewTable map[string]func(*parser, Tok, *DocComment) ViewItem

func init() {
	viewTable = map[string]func(*parser, Tok, *DocComment) ViewItem{
		WordTitle: (*parser).viewTitle, WordSubtitle: (*parser).viewSubtitle,
		WordSingular: (*parser).viewSingular, WordPlural: (*parser).viewPlural,
		wordMenu: (*parser).viewMenu, wordPreview: (*parser).viewPreview,
		wordSearch: (*parser).viewSearch, wordFilters: (*parser).viewFilters,
		wordColumns: (*parser).viewColumns, WordGroup: (*parser).viewGroup,
		WordShow: (*parser).viewShowItem,
	}
}

// viewDecl is "view" IDENT [ "." WORD ] BraceList( viewItem ).
func (p *parser) viewDecl(h *declHead) Decl {
	p.next()
	p.dropMods(h.mods, diag.KindView)
	d := &ViewDecl{Doc: h.doc, Annotations: h.anns, Type: p.ref()}
	if d.Type == nil {
		return nil
	}
	if p.accept(TokDot) != NoTok {
		if d.Case = p.word(); d.Case == nil {
			return nil
		}
	}
	d.Braces = p.braceList(func() bool {
		it := p.viewItem()
		if it != nil {
			d.Items = append(d.Items, it)
		}
		return it != nil
	}, func(b Bounds) { d.Items = append(d.Items, &BadDecl{Bounds: b}) })
	if d.Braces.Close == NoTok {
		return nil
	}
	d.Bounds = p.from(h.start)
	p.checkAnnotations(h.anns, siteOther)
	return d
}

// viewItem is { DOC } and a vocabulary item, or a field item (GRAMMAR.md §5.6).
func (p *parser) viewItem() ViewItem {
	start := p.pos
	doc := p.doc(start)
	if p.at(TokIdent) {
		if f := viewTable[p.text(p.pos)]; f != nil {
			return f(p, start, doc)
		}
	}
	if it := p.viewField(start, doc); it != nil {
		return it
	}
	return nil
}

// textItem is a vocabulary word and a string, built by build (GRAMMAR.md §5.6).
func (p *parser) textItem(start Tok, doc *DocComment, build func(Bounds, *DocComment, StrLit) ViewItem) ViewItem {
	p.next()
	if s := p.strLit(); s != nil {
		return build(p.from(start), doc, s)
	}
	return nil
}

func (p *parser) viewTitle(start Tok, doc *DocComment) ViewItem {
	return p.textItem(start, doc, func(b Bounds, d *DocComment, s StrLit) ViewItem {
		return &ViewTitle{Bounds: b, Doc: d, Text: s}
	})
}

func (p *parser) viewSubtitle(start Tok, doc *DocComment) ViewItem {
	return p.textItem(start, doc, func(b Bounds, d *DocComment, s StrLit) ViewItem {
		return &ViewSubtitle{Bounds: b, Doc: d, Text: s}
	})
}

func (p *parser) viewSingular(start Tok, doc *DocComment) ViewItem {
	return p.textItem(start, doc, func(b Bounds, d *DocComment, s StrLit) ViewItem {
		return &ViewSingular{Bounds: b, Doc: d, Text: s}
	})
}

func (p *parser) viewPlural(start Tok, doc *DocComment) ViewItem {
	return p.textItem(start, doc, func(b Bounds, d *DocComment, s StrLit) ViewItem {
		return &ViewPlural{Bounds: b, Doc: d, Text: s}
	})
}

// viewMenu is "menu" WORD "icon" WORD.
func (p *parser) viewMenu(start Tok, doc *DocComment) ViewItem {
	p.next()
	m := &ViewMenu{Doc: doc, Menu: p.word()}
	if m.Menu == nil {
		return nil
	}
	if !p.atWord(wordIcon) {
		p.fail(quoted(wordIcon))
		return nil
	}
	p.next()
	if m.Icon = p.word(); m.Icon == nil {
		return nil
	}
	m.Bounds = p.from(start)
	return m
}

func (p *parser) viewPreview(start Tok, doc *DocComment) ViewItem {
	p.next()
	x := p.expr()
	return &ViewPreview{Bounds: p.from(start), Doc: doc, X: x}
}

// viewSearch is "search" BraceList( expr ).
func (p *parser) viewSearch(start Tok, doc *DocComment) ViewItem {
	p.next()
	s := &ViewSearch{Doc: doc}
	s.Braces = p.braceList(func() bool {
		x := p.expr()
		if _, bad := x.(*BadExpr); bad && p.bail {
			return false
		}
		s.Items = append(s.Items, x)
		return true
	}, func(b Bounds) { s.Items = append(s.Items, &BadExpr{Bounds: b}) })
	if s.Braces.Close == NoTok {
		return nil
	}
	s.Bounds = p.from(start)
	return s
}

// viewFilters is "filters" BraceList( WORD [ "multi" ] ).
func (p *parser) viewFilters(start Tok, doc *DocComment) ViewItem {
	p.next()
	s := &ViewFilters{Doc: doc}
	s.Braces = p.braceList(func() bool {
		at := p.pos
		f := &ViewFilter{Name: p.word(), Multi: NoTok}
		if f.Name == nil {
			return false
		}
		if p.atWord(wordMulti) {
			f.Multi = p.next()
		}
		f.Bounds = p.from(at)
		s.Items = append(s.Items, f)
		return true
	}, nil)
	if s.Braces.Close == NoTok {
		return nil
	}
	s.Bounds = p.from(start)
	return s
}

// viewColumns is "columns" BraceList( WORD [ INT ] ).
func (p *parser) viewColumns(start Tok, doc *DocComment) ViewItem {
	p.next()
	s := &ViewColumns{Doc: doc}
	s.Braces = p.braceList(func() bool {
		at := p.pos
		c := &ViewColumn{Name: p.word()}
		if c.Name == nil {
			return false
		}
		if p.at(TokInt) {
			c.Width = p.intLit(p.next())
		}
		c.Bounds = p.from(at)
		s.Items = append(s.Items, c)
		return true
	}, nil)
	if s.Braces.Close == NoTok {
		return nil
	}
	s.Bounds = p.from(start)
	return s
}

// viewGroup is "group" WORD stringLit [ stringLit ] [ "advanced" ] [ "when" headerExpr ]
// BraceList( groupMember ).
func (p *parser) viewGroup(start Tok, doc *DocComment) ViewItem {
	p.next()
	g := &ViewGroup{Doc: doc, ID: p.word(), Advanced: NoTok}
	if g.ID == nil {
		return nil
	}
	if g.Label = p.strLit(); g.Label == nil {
		return nil
	}
	if startsString[p.kind()] {
		g.Help = p.strLit()
	}
	if p.atWord(wordAdvanced) {
		g.Advanced = p.next()
	}
	if p.atWord(wordWhen) {
		p.next()
		g.When = p.headerExpr()
	}
	g.Braces = p.braceList(func() bool {
		m := p.groupMember()
		if m != nil {
			g.Members = append(g.Members, m)
		}
		return m != nil
	}, func(b Bounds) { g.Members = append(g.Members, &BadDecl{Bounds: b}) })
	if g.Braces.Close == NoTok {
		return nil
	}
	g.Bounds = p.from(start)
	return g
}

// groupMember is { DOC } ( showItem | fieldItem ).
func (p *parser) groupMember() GroupMember {
	start := p.pos
	doc := p.doc(start)
	if p.atWord(WordShow) {
		if s := p.viewShow(start, doc); s != nil {
			return s
		}
		return nil
	}
	if f := p.viewField(start, doc); f != nil {
		return f
	}
	return nil
}

func (p *parser) viewShowItem(start Tok, doc *DocComment) ViewItem {
	if s := p.viewShow(start, doc); s != nil {
		return s
	}
	return nil
}

// viewShow is "show" [ WORD ] stringLit stringLit.
func (p *parser) viewShow(start Tok, doc *DocComment) *ViewShow {
	p.next()
	s := &ViewShow{Doc: doc}
	if isWord(p.kind()) {
		s.ID = p.word()
	}
	if s.Label = p.strLit(); s.Label == nil {
		return nil
	}
	if s.Template = p.strLit(); s.Template == nil {
		return nil
	}
	s.Bounds = p.from(start)
	return s
}

// viewField is [ "field" ] WORD [ stringLit ] [ braceLit ]; "field" before a string, a "{" or
// a separator is itself the name.
func (p *parser) viewField(start Tok, doc *DocComment) *ViewField {
	f := &ViewField{Doc: doc, Field: NoTok}
	if p.atWord(WordField) && isWord(p.peek(1)) {
		f.Field = p.next()
	}
	if f.Name = p.word(); f.Name == nil {
		return nil
	}
	if startsString[p.kind()] {
		f.Label = p.strLit()
	}
	if p.at(TokLBrace) {
		if f.Props = p.braceLit(); f.Props == nil {
			return nil
		}
	}
	f.Bounds = p.from(start)
	return f
}
