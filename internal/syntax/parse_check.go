package syntax

import "github.com/fantasim/canonlang/internal/diag"

func (p *parser) topCheck(h *declHead) Decl {
	p.dropMods(h.mods, diag.KindCheck)
	if d := p.checkDecl(h, siteOther); d != nil {
		return d
	}
	return nil
}

// checkDecl is ( "check" | "warn" ) ( block | [ IDENT ":" ] expr [ "at" IDENT ] "else"
// stringLit ); "check {" always starts the block form (GRM-11).
func (p *parser) checkDecl(h *declHead, site annSite) *CheckDecl {
	d := &CheckDecl{Doc: h.doc, Annotations: h.anns, Keyword: p.kind()}
	p.next()
	if p.at(TokLBrace) {
		outer := p.ctx
		p.ctx = bodyCtx{check: true}
		d.Body = p.block()
		p.ctx = outer
		if d.Body == nil {
			return nil
		}
	} else if !p.inlineCheck(d) {
		return nil
	}
	d.Bounds = p.from(h.start)
	p.checkAnnotations(h.anns, site)
	return d
}

func (p *parser) inlineCheck(d *CheckDecl) bool {
	if p.at(TokIdent) && p.peek(1) == TokColon {
		d.Name = p.ref()
		p.next()
	}
	d.Cond = p.expr()
	if p.atWord(wordAt) {
		p.next()
		if d.At = p.ref(); d.At == nil {
			return false
		}
	}
	if p.expect(KwElse) == NoTok {
		return false
	}
	d.Message = p.strLit()
	return d.Message != nil
}

// block is BraceList( statement ) (GRAMMAR.md §5.10); nil when its braces are missing.
func (p *parser) block() *Block {
	start := p.pos
	b := &Block{}
	d := p.braceList(func() bool {
		s := p.stmt()
		if s != nil {
			b.Stmts = append(b.Stmts, s)
		}
		return s != nil
	}, func(bd Bounds) { b.Stmts = append(b.Stmts, &BadStmt{Bounds: bd}) })
	if d.Close == NoTok {
		return nil
	}
	b.Bounds = p.from(start)
	return b
}

// widgetDecl is "widget" IDENT "(" "value" ":" type [ "," "siblings" ":" type ] [ "," ] ")"
// [ "default" ].
func (p *parser) widgetDecl(h *declHead) Decl {
	p.next()
	p.dropMods(h.mods, diag.KindWidget)
	d := &WidgetDecl{Doc: h.doc, Annotations: h.anns, Name: p.ident(diag.KindWidget), Default: NoTok}
	if d.Name == nil {
		return nil
	}
	d.Parens = p.parenList(TokLParen, TokRParen, func() {
		i := len(d.Params)
		if i >= len(widgetParams) || !p.atWord(widgetParams[i]) {
			p.fail(quoted(widgetParams[min(i, len(widgetParams)-1)]))
			return
		}
		if prm := p.param(diag.KindParameter); prm != nil {
			d.Params = append(d.Params, prm)
		}
	})
	if d.Parens.Close == NoTok || len(d.Params) == 0 {
		if d.Parens.Close != NoTok {
			p.failAt(d.Parens.Close, quoted(widgetParams[0]))
		}
		return nil
	}
	if p.atWord(wordDefault) {
		d.Default = p.next()
	}
	d.Bounds = p.from(h.start)
	p.checkAnnotations(h.anns, siteOther)
	return d
}
