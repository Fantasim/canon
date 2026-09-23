package syntax

// annotation is "@" WORD [ "(" [ annArg { "," annArg } [ "," ] ] ")" ]: the name directly
// after "@", the "(" directly after the name.
func (p *parser) annotation() *Annotation {
	start := p.next()
	if !isWord(p.kind()) || !p.adjacent(start, p.pos) {
		p.fail(wordName)
		return nil
	}
	a := &Annotation{Name: p.word()}
	if p.at(TokLParen) && p.adjacent(p.pos-1, p.pos) {
		a.Parens = p.parenList(TokLParen, TokRParen, func() {
			if arg := p.annArg(); arg != nil {
				a.Args = append(a.Args, arg)
			}
		})
		if a.Parens.Close == NoTok {
			return nil
		}
	}
	a.Bounds = p.from(start)
	return a
}

// annotations reads the trailing annotations of a header, a field, a member, a case or an entry.
func (p *parser) annotations() []*Annotation {
	var out []*Annotation
	for p.at(TokAt) {
		a := p.annotation()
		if a == nil {
			break
		}
		out = append(out, a)
	}
	return out
}

// prefixAnnotations reads { annotation { NL } } before a declaration (GRAMMAR.md §5.3).
func (p *parser) prefixAnnotations() []*Annotation {
	var out []*Annotation
	for p.at(TokAt) {
		a := p.annotation()
		if a == nil {
			break
		}
		out = append(out, a)
		p.skipNL()
	}
	return out
}

// annArg is [ WORD ":" ] annValue.
func (p *parser) annArg() *AnnotationArg {
	start := p.pos
	arg := &AnnotationArg{}
	if isWord(p.kind()) && p.peek(1) == TokColon {
		arg.Name = p.word()
		p.next()
	}
	arg.Value = p.annValue()
	arg.Bounds = p.from(start)
	return arg
}

// annValue is stringLit, [ "-" ] a number or duration, true, false, a qualifiedWord (a symbol,
// never resolved), a bracketed list of values or "{}".
func (p *parser) annValue() AnnValue {
	start := p.pos
	switch k := p.kind(); {
	case startsString[k]:
		if s := p.strLit(); s != nil {
			return s
		}
		return p.badExpr(start)
	case k == TokMinus || isNumber[k]:
		return p.signedLiteral()
	case k == KwTrue || k == KwFalse:
		t := p.next()
		return &BoolLit{Bounds: Bounds{From: t, To: t}, Value: k == KwTrue}
	case isWord(k):
		return p.qualifiedWord()
	case k == TokLBrack:
		return p.annList()
	case k == TokLBrace && p.peek(1) == TokRBrace:
		p.next()
		return &BraceLit{Bounds: Bounds{From: start, To: p.next()}}
	}
	p.fail(annValueName)
	return p.badExpr(start)
}

// annList is "[" [ annValue { "," annValue } [ "," ] ] "]".
func (p *parser) annList() AnnValue {
	start := p.pos
	l := &AnnotationList{}
	d := p.parenList(TokLBrack, TokRBrack, func() { l.Items = append(l.Items, p.annValue()) })
	if d.Close == NoTok {
		return p.badExpr(start)
	}
	l.Bounds = p.from(start)
	return l
}

// signedLiteral is [ "-" ] ( INT | FLOAT | DURATION ), the sign folded (GRAMMAR.md §2.4).
func (p *parser) signedLiteral() annExpr {
	start := p.pos
	neg := p.accept(TokMinus)
	if !isNumber[p.kind()] {
		p.fail(numberName)
		return p.badExpr(start)
	}
	lit := p.numLit(p.next(), neg != NoTok)
	if neg != NoTok {
		return negate(neg, lit)
	}
	return lit
}

// annExpr is a node that is both an expression and an annotation value: a literal or a BadExpr.
type annExpr interface {
	Expr
	AnnValue
}
