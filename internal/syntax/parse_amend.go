package syntax

// amendBlock is { DOC } "amend" IDENT BraceList( amendItem ) (GRAMMAR.md §5.7).
func (p *parser) amendBlock() *AmendBlock {
	start := p.pos
	a := &AmendBlock{Doc: p.doc(start)}
	p.next()
	if a.Target = p.ref(); a.Target == nil {
		return nil
	}
	a.Braces = p.braceList(func() bool {
		m := p.amendment()
		if m != nil {
			a.Items = append(a.Items, m)
		}
		return m != nil
	}, nil)
	if a.Braces.Close == NoTok {
		return nil
	}
	a.Bounds = p.from(start)
	return a
}

// amendment is { DOC } amendPath ":" expr, amendPath being WORD { "." WORD | "[" expr "]" |
// "[" "#" INT "]" }.
func (p *parser) amendment() *Amendment {
	start := p.pos
	m := &Amendment{Doc: p.doc(start)}
	first := p.word()
	if first == nil {
		return nil
	}
	m.Path = append(m.Path, &AmendSegment{Bounds: first.Bounds, Name: first})
	for p.at(TokDot) || p.at(TokLBrack) {
		seg := p.amendSegment()
		if seg == nil {
			return nil
		}
		m.Path = append(m.Path, seg)
	}
	if p.expect(TokColon) == NoTok {
		return nil
	}
	m.Value = p.expr()
	m.Bounds = p.from(start)
	return m
}

func (p *parser) amendSegment() *AmendSegment {
	start := p.next()
	seg := &AmendSegment{}
	if p.toks[start].Kind == TokDot {
		if seg.Name = p.word(); seg.Name == nil {
			return nil
		}
		seg.Bounds = p.from(start)
		return seg
	}
	if p.accept(TokHash) != NoTok {
		if !p.at(TokInt) {
			p.fail(expected(TokInt))
			return nil
		}
		seg.Position = p.intLit(p.next())
	} else {
		seg.Key = p.inBrackets(p.expr)
	}
	if p.expect(TokRBrack) == NoTok {
		return nil
	}
	seg.Bounds = p.from(start)
	return seg
}

// inBrackets parses x with header mode off, as inside any bracket (GRAMMAR.md §6.1).
func (p *parser) inBrackets(x func() Expr) Expr {
	header := p.header
	p.header = false
	defer func() { p.header = header }()
	return x()
}

// translationEntry is { DOC } translationKey stringLit, the key WORD { "." WORD }.
func (p *parser) translationEntry() *TranslationEntry {
	start := p.pos
	e := &TranslationEntry{Doc: p.doc(start), Key: p.qualifiedWord()}
	if e.Key == nil {
		return nil
	}
	if e.Text = p.strLit(); e.Text == nil {
		return nil
	}
	e.Bounds = p.from(start)
	return e
}

// projectDecl is { DOC } "project" IDENT BraceList( projectItem ); in project.canon its doc
// block is the project's, File.Doc stays nil.
func (p *parser) projectDecl() *ProjectDecl {
	start := p.pos
	d := &ProjectDecl{Doc: p.doc(start)}
	p.next()
	if d.Name = p.ref(); d.Name == nil {
		return nil
	}
	d.Braces = p.braceList(func() bool {
		e := p.projectEntry(true)
		if e != nil {
			d.Items = append(d.Items, e)
		}
		return e != nil
	}, nil)
	if d.Braces.Close == NoTok {
		return nil
	}
	d.Bounds = p.from(start)
	return d
}

// projectEntry is a projectItem, { DOC } WORD ( ":" pValue | BraceList( pEntry ) ), or, below
// the top, a pEntry, { DOC } ( WORD | stringLit ) ":" pValue.
func (p *parser) projectEntry(top bool) *ProjectEntry {
	start := p.pos
	e := &ProjectEntry{Doc: p.doc(start), Colon: NoTok}
	if top || isWord(p.kind()) {
		if key := p.word(); key != nil {
			e.Key = key
		}
	} else if s := p.constString(); s != nil {
		e.Key = s
	}
	if e.Key == nil {
		return nil
	}
	if top && p.at(TokLBrace) {
		e.Value = p.projectMap()
	} else if e.Colon = p.expect(TokColon); e.Colon != NoTok {
		e.Value = p.projectValue()
	} else {
		return nil
	}
	e.Bounds = p.from(start)
	return e
}

// projectValue is stringLit | INT | qualifiedIdent | "[" [ pValue { "," pValue } [ "," ] ] "]"
// | BraceList( pEntry ); strings are constant (E1132).
func (p *parser) projectValue() ProjectValue {
	start := p.pos
	switch k := p.kind(); {
	case startsString[k]:
		if s := p.constString(); s != nil {
			return s
		}
	case k == TokInt:
		return p.intLit(p.next())
	case k == TokIdent:
		if q := p.qualifiedIdent(); q != nil {
			return q
		}
	case k == TokLBrack:
		return p.projectList()
	case k == TokLBrace:
		return p.projectMap()
	default:
		p.fail(projectValueName)
	}
	return p.badExpr(start)
}

func (p *parser) projectList() ProjectValue {
	start := p.pos
	l := &ProjectList{}
	d := p.parenList(TokLBrack, TokRBrack, func() { l.Items = append(l.Items, p.projectValue()) })
	if d.Close == NoTok {
		return p.badExpr(start)
	}
	l.Bounds = p.from(start)
	return l
}

func (p *parser) projectMap() ProjectValue {
	start := p.pos
	m := &ProjectMap{}
	d := p.braceList(func() bool {
		e := p.projectEntry(false)
		if e != nil {
			m.Entries = append(m.Entries, e)
		}
		return e != nil
	}, nil)
	if d.Close == NoTok {
		return p.badExpr(start)
	}
	m.Bounds = p.from(start)
	return m
}
