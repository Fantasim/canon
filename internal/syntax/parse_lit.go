package syntax

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
)

// numLit is the literal of a numeric token with its exact value. A duration of 2^63 ms fits
// only negated, so it is E1111 unless negated is set.
func (p *parser) numLit(t Tok, negated bool) annExpr {
	text := p.text(t)
	b := Bounds{From: t, To: t}
	switch p.toks[t].Kind {
	case TokFloat:
		coef, exp, _ := floatParts(text)
		return &FloatLit{Bounds: b, Coef: coef, Exp: exp}
	case TokDuration:
		ms := durationMillis(text)
		if ms <= math.MaxInt64 {
			return &DurationLit{Bounds: b, Millis: int64(ms)}
		}
		if !negated {
			diag.E1111.AtOverflow(p.span(t, t), text).Report(p.bag)
		}
		return &DurationLit{Bounds: b, Millis: math.MinInt64}
	default:
	}
	return p.intLit(t)
}

func (p *parser) intLit(t Tok) *IntLit {
	return &IntLit{Bounds: Bounds{From: t, To: t}, Value: intValue(p.text(t))}
}

// negate folds a unary "-" at neg into a numeric literal (GRAMMAR.md §2.4, DECISIONS 76).
func negate(neg Tok, lit annExpr) annExpr {
	switch l := lit.(type) {
	case *IntLit:
		l.Value.Neg(l.Value)
		l.From = neg
	case *FloatLit:
		l.Neg = true
		l.From = neg
	case *DurationLit:
		l.Millis = -l.Millis
		l.From = neg
	}
	return lit
}

// braceLit is BraceList( braceItem ), or one named or keyed item followed by comprehension
// clauses; nil when its braces are missing.
func (p *parser) braceLit() *BraceLit {
	start := p.pos
	b := &BraceLit{}
	d := p.braceList(func() bool {
		late := len(b.Clauses) > 0
		if late {
			diag.E1136.At(p.span(p.pos, p.pos)).Report(p.bag)
		}
		it := p.braceItem()
		if it == nil || late {
			return it != nil
		}
		b.Items = append(b.Items, it)
		if p.clauseAhead(KwFor) {
			p.comprehension(b)
		}
		return true
	}, func(bd Bounds) {
		if len(b.Clauses) == 0 {
			b.Items = append(b.Items, &BadDecl{Bounds: bd})
		}
	})
	if d.Close == NoTok {
		return nil
	}
	b.Bounds = p.from(start)
	return b
}

// comprehension reads the clauses after the item of a comprehension brace literal, which must
// be its only item, named or keyed (E1136); a named item's key is its name as an expression.
func (p *parser) comprehension(b *BraceLit) {
	last := len(b.Items) - 1
	switch it := b.Items[last].(type) {
	case *FieldItem:
		key := &IdentExpr{Bounds: it.Name.Bounds, Name: it.Name.Name}
		b.Items[last] = &MapItem{Bounds: it.Bounds, Key: key, Value: it.Value}
	case *MapItem:
	default:
		diag.E1136.At(p.nodeSpan(it)).Report(p.bag)
	}
	if last > 0 {
		diag.E1136.At(p.nodeSpan(b.Items[last])).Report(p.bag)
	}
	for p.clauseAhead(KwFor) || p.clauseAhead(KwIf) || p.clauseAhead(KwLet) {
		p.skipNL()
		b.Clauses = append(b.Clauses, p.compClause())
	}
}

// clauseAhead reports a clause keyword k, after line breaks.
func (p *parser) clauseAhead(k TokenKind) bool {
	i := 0
	for p.peek(i) == TokNL {
		i++
	}
	return p.peek(i) == k
}

// compClause is "for" binder [ "," binder ] "in" expr | "if" expr | "let" IDENT "=" expr.
func (p *parser) compClause() *CompClause {
	start := p.pos
	c := &CompClause{Keyword: p.kind()}
	p.next()
	switch c.Keyword {
	case KwFor:
		c.Vars = p.binders()
		p.expect(KwIn)
	case KwLet:
		if v := p.ident(diag.KindLocal); v != nil {
			c.Vars = []*Ident{v}
		}
		p.expect(TokAssign)
	default:
	}
	c.X = p.expr()
	c.Bounds = p.from(start)
	return c
}

// binders is binder [ "," binder ], a binder being IDENT or "_".
func (p *parser) binders() []*Ident {
	vars := []*Ident{p.binder()}
	if p.accept(TokComma) != NoTok {
		vars = append(vars, p.binder())
	}
	out := vars[:0]
	for _, v := range vars {
		if v != nil {
			out = append(out, v)
		}
	}
	return out
}

func (p *parser) binder() *Ident {
	if p.at(TokUnderscore) {
		t := p.next()
		return &Ident{Bounds: Bounds{From: t, To: t}, Name: Blank}
	}
	return p.ident(diag.KindVariable)
}

// braceItem is { DOC } "..." expr, [ "retired" ] WORD { annotation } braceLit (a table entry),
// WORD ":" expr, or expr ":" expr.
func (p *parser) braceItem() BraceItem {
	start := p.pos
	k, next := p.kind(), p.peek(1)
	switch {
	case k == TokEllipsis:
		p.next()
		return &SpreadItem{X: p.expr(), Bounds: p.from(start)}
	case isModifier[k] && isWord(next), isWord(k) && (next == TokLBrace || next == TokAt):
		if e := p.entryItem(); e != nil {
			return e
		}
		return nil
	case isWord(k) && next == TokColon:
		name := p.word()
		p.next()
		return &FieldItem{Name: name, Value: p.expr(), Bounds: p.from(start)}
	}
	key := p.expr()
	if p.expect(TokColon) == NoTok {
		return nil
	}
	return &MapItem{Key: key, Value: p.expr(), Bounds: p.from(start)}
}

// entryItem is { DOC } [ "retired" ] WORD { annotation } braceLit.
func (p *parser) entryItem() *EntryItem {
	h := p.head()
	e := &EntryItem{Doc: h.doc, Mods: p.allowMods(h.mods, modRetired, diag.KindEntry)}
	if e.Key = p.dataWord(); e.Key == nil {
		return nil
	}
	e.Annotations = p.annotations()
	p.checkAnnotations(e.Annotations, siteTableEntry)
	if e.Value = p.braceLit(); e.Value == nil {
		return nil
	}
	e.Bounds = p.from(h.start)
	return e
}

// listLit is "[" [ expr { "," expr } [ "," ] ] "]" or "[" expr compClause { compClause } "]".
func (p *parser) listLit() Expr {
	start := p.pos
	var elems []Expr
	var comp *ListComp
	d := p.parenList(TokLBrack, TokRBrack, func() {
		x := p.expr()
		if len(elems) == 0 && comp == nil && p.at(KwFor) {
			comp = &ListComp{Elem: x}
			for p.at(KwFor) || p.at(KwIf) || p.at(KwLet) {
				comp.Clauses = append(comp.Clauses, p.compClause())
			}
			return
		}
		elems = append(elems, x)
	})
	switch {
	case d.Close == NoTok:
		return p.badExpr(start)
	case comp != nil && len(elems) > 0:
		p.failAt(elems[0].First(), expected(TokRBrack))
		return p.badExpr(start)
	case comp != nil:
		comp.Bounds = p.from(start)
		return comp
	}
	return &ListLit{Elems: elems, Bounds: p.from(start)}
}
