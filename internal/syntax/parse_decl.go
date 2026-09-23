package syntax

import "github.com/fantasim/canonlang/internal/diag"

// declHead is what precedes a declaration's keyword: its first token, its doc block, its
// prefix annotations and its modifiers.
type declHead struct {
	start Tok
	doc   *DocComment
	anns  []*Annotation
	mods  *Modifiers
}

// declTable dispatches a top-level declaration on its keyword (GRAMMAR.md §5.3, DECISIONS 26).
var declTable [TokenKindCount]func(*parser, *declHead) Decl

func init() {
	declTable[KwConst], declTable[KwLet], declTable[KwType] = (*parser).constDecl, (*parser).letDecl, (*parser).typeDecl
	declTable[KwFn], declTable[KwEntry], declTable[KwTest] = (*parser).topFn, (*parser).entryDecl, (*parser).testDecl
	declTable[KwCheck], declTable[KwWarn] = (*parser).topCheck, (*parser).topCheck
	declTable[KwView], declTable[KwWidget], declTable[KwEmit] = (*parser).viewDecl, (*parser).widgetDecl, (*parser).emitDecl
	declTable[KwRecord], declTable[KwEnum] = (*parser).recordDecl, (*parser).enumDecl
	declTable[KwVariant] = (*parser).variantDecl
}

// topDecl is { DOC } { annotation { NL } } topDeclBody (GRAMMAR.md §5.3).
func (p *parser) topDecl() Decl {
	h := p.head()
	if f := declTable[p.kind()]; f != nil {
		return f(p, h)
	}
	p.fail(declName)
	return nil
}

// head reads the doc block, the prefix annotations and the modifiers of an item.
func (p *parser) head() *declHead {
	h := &declHead{start: p.pos, doc: p.doc(p.pos)}
	h.anns = p.prefixAnnotations()
	h.mods = p.modifiers()
	return h
}

// modifiers reads "local", "export" and "retired" before a WORD, each at most once, in any
// order; the declaration decides E1133. Before anything else such a word is a name.
func (p *parser) modifiers() *Modifiers {
	start := p.pos
	m := &Modifiers{}
	for slot := m.slot(p.kind()); slot != nil && *slot == NoTok && isWord(p.peek(1)); slot = m.slot(p.kind()) {
		*slot = p.next()
	}
	if p.pos == start {
		return nil
	}
	m.Bounds = p.from(start)
	return m
}

func (m *Modifiers) slot(k TokenKind) *Tok {
	switch k {
	case KwLocal:
		return &m.Local
	case KwExport:
		return &m.Export
	case KwRetired:
		return &m.Retired
	default:
	}
	return nil
}

// allowMods reports E1133 for each modifier outside allowed on a declaration of kind; nil
// modifiers stay nil.
func (p *parser) allowMods(m *Modifiers, allowed uint8, kind diag.Kind) *Modifiers {
	if m == nil {
		return nil
	}
	for i, t := range [...]Tok{m.Local, m.Export, m.Retired} {
		if t != NoTok && allowed&(1<<i) == 0 {
			diag.E1133.At(p.span(t, t), p.text(t), kind).Report(p.bag)
		}
	}
	return m
}

func (p *parser) constDecl(h *declHead) Decl {
	p.next()
	d := &ConstDecl{Doc: h.doc, Annotations: h.anns, Mods: p.allowMods(h.mods, modLocal, diag.KindConst)}
	if d.Name = p.ident(diag.KindConst); d.Name == nil || p.expect(TokAssign) == NoTok {
		return nil
	}
	d.Value = p.expr()
	d.Bounds = p.from(h.start)
	p.checkAnnotations(h.anns, siteConst)
	return d
}

func (p *parser) letDecl(h *declHead) Decl {
	p.next()
	d := &LetDecl{Doc: h.doc, Annotations: h.anns, Mods: p.allowMods(h.mods, modLocal, diag.KindLet)}
	if d.Name = p.ident(diag.KindLet); d.Name == nil {
		return nil
	}
	if p.accept(TokColon) != NoTok {
		d.Type = p.typ()
	}
	if p.expect(TokAssign) == NoTok {
		return nil
	}
	d.Value = p.expr()
	d.Bounds = p.from(h.start)
	p.checkAnnotations(h.anns, siteLet)
	return d
}

func (p *parser) typeDecl(h *declHead) Decl {
	p.next()
	d := &TypeDecl{Doc: h.doc, Annotations: h.anns, Mods: p.allowMods(h.mods, modLocal, diag.KindTypeAlias)}
	if d.Name = p.ident(diag.KindTypeAlias); d.Name == nil {
		return nil
	}
	if p.at(TokLParen) {
		if d.Parens, d.Params = p.typeParams(); d.Parens.Close == NoTok {
			return nil
		}
	}
	if p.expect(TokAssign) == NoTok {
		return nil
	}
	d.Type = p.typ()
	d.Bounds = p.from(h.start)
	p.checkAnnotations(h.anns, siteType)
	return d
}

// typeParams is ParenList( param ), with at least one parameter (GRAMMAR.md §5.3).
func (p *parser) typeParams() (Delims, []*Param) {
	var params []*Param
	d := p.parenList(TokLParen, TokRParen, func() {
		if prm := p.param(diag.KindTypeParameter); prm != nil {
			params = append(params, prm)
		}
	})
	if d.Close != NoTok && len(params) == 0 {
		p.failAt(d.Close, identName)
		return Delims{Open: d.Open}, nil
	}
	return d, params
}

// param is IDENT ":" type [ "=" expr ].
func (p *parser) param(kind diag.Kind) *Param {
	start := p.pos
	prm := &Param{Name: p.ident(kind)}
	if prm.Name == nil || p.expect(TokColon) == NoTok {
		return nil
	}
	prm.Type = p.typ()
	if p.accept(TokAssign) != NoTok {
		prm.Default = p.expr()
	}
	prm.Bounds = p.from(start)
	return prm
}

func (p *parser) topFn(h *declHead) Decl {
	if d := p.fnDecl(h, false); d != nil {
		return d
	}
	return nil
}

// fnDecl is [ "local" | "export" ] "fn" IDENT "(" [ fnParams ] ")" "->" type block; a method
// may be exported, not local.
func (p *parser) fnDecl(h *declHead, method bool) *FnDecl {
	p.next()
	allowed, nameKind, site := modLocal|modExport, diag.KindFunction, siteFn
	if method {
		allowed, nameKind, site = modExport, diag.KindMethod, siteMethod
	}
	d := &FnDecl{Doc: h.doc, Annotations: h.anns, Mods: p.allowMods(h.mods, allowed, diag.KindFunction), Self: NoTok}
	if d.Name = p.ident(nameKind); d.Name == nil {
		return nil
	}
	d.Parens = p.parenList(TokLParen, TokRParen, func() { p.fnParam(d) })
	if d.Parens.Close == NoTok || p.expect(TokArrow) == NoTok {
		return nil
	}
	d.Result = p.typ()
	outer := p.ctx
	p.ctx = bodyCtx{fn: true}
	d.Body = p.block()
	p.ctx = outer
	if d.Body == nil {
		return nil
	}
	d.Bounds = p.from(h.start)
	p.checkAnnotations(h.anns, site)
	return d
}

// fnParam is "self" as the first parameter, or a param.
func (p *parser) fnParam(d *FnDecl) {
	if p.at(KwSelf) && d.Self == NoTok && len(d.Params) == 0 {
		d.Self = p.next()
		return
	}
	if prm := p.param(diag.KindParameter); prm != nil {
		d.Params = append(d.Params, prm)
	}
}

// entryDecl is [ "retired" ] "entry" IDENT "." entryKey braceLit (GRAMMAR.md §5.3).
func (p *parser) entryDecl(h *declHead) Decl {
	p.next()
	d := &EntryDecl{Doc: h.doc, Annotations: h.anns, Mods: p.allowMods(h.mods, modRetired, diag.KindEntry)}
	if d.Table = p.ref(); d.Table == nil || p.expect(TokDot) == NoTok {
		return nil
	}
	if p.at(TokInt) {
		d.Key = p.intLit(p.next())
	} else if key := p.dataWord(); key != nil {
		d.Key = key
	} else {
		return nil
	}
	if d.Value = p.braceLit(); d.Value == nil {
		return nil
	}
	d.Bounds = p.from(h.start)
	p.checkAnnotations(h.anns, siteEntry)
	return d
}

// testDecl is "test" stringLit block, the name a constant string (E1132).
func (p *parser) testDecl(h *declHead) Decl {
	p.next()
	p.dropMods(h.mods, diag.KindTest)
	d := &TestDecl{Doc: h.doc, Annotations: h.anns, Name: p.constString()}
	if d.Name == nil {
		return nil
	}
	outer := p.ctx
	p.ctx = bodyCtx{test: true}
	d.Body = p.block()
	p.ctx = outer
	if d.Body == nil {
		return nil
	}
	d.Bounds = p.from(h.start)
	p.checkAnnotations(h.anns, siteOther)
	return d
}

// emitDecl is "emit" WORD braceLit.
func (p *parser) emitDecl(h *declHead) Decl {
	p.next()
	p.dropMods(h.mods, diag.KindEmit)
	d := &EmitDecl{Doc: h.doc, Annotations: h.anns, Target: p.word()}
	if d.Target == nil {
		return nil
	}
	if d.Options = p.braceLit(); d.Options == nil {
		return nil
	}
	d.Bounds = p.from(h.start)
	p.checkAnnotations(h.anns, siteOther)
	return d
}

// dropMods reports E1133 for every modifier of a declaration that takes none.
func (p *parser) dropMods(m *Modifiers, kind diag.Kind) { p.allowMods(m, 0, kind) }
