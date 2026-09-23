package syntax

import "github.com/fantasim/canonlang/internal/diag"

// recordDecl is [ "local" ] "record" IDENT [ typeParams ] { annotation } recordBody; its
// annotations follow the name, a prefix one is E1118.
func (p *parser) recordDecl(h *declHead) Decl {
	p.next()
	p.misplaced(h.anns, diag.KindTopLevelDeclaration)
	d := &RecordDecl{Doc: h.doc, Mods: p.allowMods(h.mods, modLocal, diag.KindRecord)}
	if d.Name = p.ident(diag.KindRecord); d.Name == nil {
		return nil
	}
	if p.at(TokLParen) {
		if d.Parens, d.Params = p.typeParams(); d.Parens.Close == NoTok {
			return nil
		}
	}
	d.Annotations = p.annotations()
	p.checkAnnotations(d.Annotations, siteRecordHeader)
	if d.Body = p.recordBody(); d.Body == nil {
		return nil
	}
	d.Bounds = p.from(h.start)
	return d
}

// recordBody is BraceList( recordItem ), the body of a record or of a variant case.
func (p *parser) recordBody() *RecordBody {
	start := p.pos
	b := &RecordBody{}
	d := p.braceList(func() bool {
		it := p.recordItem()
		if it != nil {
			b.Items = append(b.Items, it)
		}
		return it != nil
	}, func(bd Bounds) { b.Items = append(b.Items, &BadDecl{Bounds: bd}) })
	if d.Close == NoTok {
		return nil
	}
	b.Bounds = p.from(start)
	return b
}

// recordItem is { DOC } ( fieldDecl | { annotation { NL } } ( fnDecl | checkDecl ) ).
func (p *parser) recordItem() RecordItem {
	h := p.head()
	if isMember[p.kind()] {
		if m := p.member(h); m != nil {
			return m
		}
		return nil
	}
	p.misplaced(h.anns, diag.KindField)
	p.dropMods(h.mods, diag.KindField)
	if f := p.fieldDecl(h); f != nil {
		return f
	}
	return nil
}

// member is a method or a check of a record, variant or case body; nil when it did not parse.
func (p *parser) member(h *declHead) memberItem {
	switch p.kind() {
	case KwFn:
		if f := p.fnDecl(h, true); f != nil {
			return f
		}
	case KwCheck, KwWarn:
		p.dropMods(h.mods, diag.KindCheck)
		if c := p.checkDecl(h, siteMethod); c != nil {
			return c
		}
	default:
	}
	return nil
}

// fieldDecl is IDENT ":" fieldType [ "=" expr ] { annotation }, fieldType being "input" type
// "from" "env" stringLit or type.
func (p *parser) fieldDecl(h *declHead) *FieldDecl {
	f := &FieldDecl{Doc: h.doc, Input: NoTok}
	if f.Name = p.ident(diag.KindField); f.Name == nil || p.expect(TokColon) == NoTok {
		return nil
	}
	f.Input = p.accept(KwInput)
	f.Type = p.typ()
	if p.atWord(wordFrom) && p.wordAt(p.pos+1, wordEnv) {
		from := p.next()
		p.next()
		if f.Env = p.constString(); f.Env == nil {
			return nil
		}
		if f.Input == NoTok {
			diag.E1131.AtNoInput(p.span(from, from+1)).Report(p.bag)
		}
	} else if f.Input != NoTok {
		diag.E1131.AtMissing(p.span(f.Input, f.Input)).Report(p.bag)
	}
	if p.accept(TokAssign) != NoTok {
		f.Default = p.expr()
	}
	f.Annotations = p.annotations()
	p.checkAnnotations(f.Annotations, siteField)
	f.Bounds = p.from(h.start)
	return f
}

// enumDecl is [ "local" ] "enum" IDENT [ "ordered" ] { annotation } BraceList( enumMember ).
func (p *parser) enumDecl(h *declHead) Decl {
	p.next()
	p.misplaced(h.anns, diag.KindTopLevelDeclaration)
	d := &EnumDecl{Doc: h.doc, Mods: p.allowMods(h.mods, modLocal, diag.KindEnum), Ordered: NoTok}
	if d.Name = p.ident(diag.KindEnum); d.Name == nil {
		return nil
	}
	if p.atWord(wordOrdered) {
		d.Ordered = p.next()
	}
	d.Annotations = p.annotations()
	codes := p.checkAnnotations(d.Annotations, siteEnumHeader)
	site := siteMember
	if codes {
		site = siteCodedMember
	}
	d.Braces = p.braceList(func() bool {
		m := p.enumMember(site)
		if m != nil {
			d.Members = append(d.Members, m)
		}
		return m != nil
	}, nil)
	if d.Braces.Close == NoTok {
		return nil
	}
	d.Bounds = p.from(h.start)
	return d
}

// enumMember is { DOC } [ "retired" ] WORD [ "=" ( stringLit | [ "-" ] INT ) ] { annotation }.
func (p *parser) enumMember(site annSite) *EnumMember {
	h := p.head()
	p.misplaced(h.anns, diag.KindEnumMember)
	m := &EnumMember{Doc: h.doc, Mods: p.allowMods(h.mods, modRetired, diag.KindMember)}
	if m.Name = p.dataWord(); m.Name == nil {
		return nil
	}
	if p.accept(TokAssign) != NoTok {
		if m.Value = p.memberValue(); m.Value == nil {
			return nil
		}
	}
	m.Annotations = p.annotations()
	p.checkAnnotations(m.Annotations, site)
	m.Bounds = p.from(h.start)
	return m
}

// memberValue is stringLit or [ "-" ] INT.
func (p *parser) memberValue() Expr {
	if startsString[p.kind()] {
		if s := p.strLit(); s != nil {
			return s
		}
		return nil
	}
	neg := p.accept(TokMinus)
	if !p.at(TokInt) {
		p.fail(stringName, expected(TokInt))
		return nil
	}
	lit := p.intLit(p.next())
	if neg != NoTok {
		return negate(neg, lit)
	}
	return lit
}

// variantDecl is [ "local" ] "variant" IDENT { annotation } BraceList( variantItem ).
func (p *parser) variantDecl(h *declHead) Decl {
	p.next()
	p.misplaced(h.anns, diag.KindTopLevelDeclaration)
	d := &VariantDecl{Doc: h.doc, Mods: p.allowMods(h.mods, modLocal, diag.KindVariant)}
	if d.Name = p.ident(diag.KindVariant); d.Name == nil {
		return nil
	}
	d.Annotations = p.annotations()
	p.checkAnnotations(d.Annotations, siteVariantHeader)
	d.Braces = p.braceList(func() bool {
		it := p.variantItem()
		if it != nil {
			d.Items = append(d.Items, it)
		}
		return it != nil
	}, func(b Bounds) { d.Items = append(d.Items, &BadDecl{Bounds: b}) })
	if d.Braces.Close == NoTok {
		return nil
	}
	d.Bounds = p.from(h.start)
	return d
}

// variantItem is { DOC } ( { annotation { NL } } ( fnDecl | checkDecl ) | variantCase ).
func (p *parser) variantItem() VariantItem {
	h := p.head()
	if isMember[p.kind()] {
		if m := p.member(h); m != nil {
			return m
		}
		return nil
	}
	p.misplaced(h.anns, diag.KindVariantCase)
	if c := p.variantCase(h); c != nil {
		return c
	}
	return nil
}

// variantCase is [ "retired" ] WORD { annotation } [ recordBody ].
func (p *parser) variantCase(h *declHead) *VariantCase {
	c := &VariantCase{Doc: h.doc, Mods: p.allowMods(h.mods, modRetired, diag.KindCase)}
	if c.Name = p.dataWord(); c.Name == nil {
		return nil
	}
	c.Annotations = p.annotations()
	p.checkAnnotations(c.Annotations, siteCase)
	if p.at(TokLBrace) {
		if c.Body = p.recordBody(); c.Body == nil {
			return nil
		}
	}
	c.Bounds = p.from(h.start)
	return c
}

// misplaced reports E1118 for prefix annotations where only trailing ones may stand.
func (p *parser) misplaced(anns []*Annotation, position diag.Kind) {
	for _, a := range anns {
		diag.E1118.At(p.nodeSpan(a), a.Name.Name, position).Report(p.bag)
	}
}

// memberItem is what a method or a check is in every body: a record and a variant item.
type memberItem interface {
	RecordItem
	VariantItem
}
