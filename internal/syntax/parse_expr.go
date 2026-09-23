package syntax

import "github.com/fantasim/canonlang/internal/diag"

// levelTable holds the parser of each precedence level, lowest first; binaryLevel (constants.go)
// gives the level of each binary operator.
var levelTable [levelCount]func(*parser, uint8) Expr

func init() {
	levelTable = [levelCount]func(*parser, uint8) Expr{
		levelCoalesce: (*parser).rightAssoc, levelOr: (*parser).leftAssoc, levelAnd: (*parser).leftAssoc,
		levelNot: (*parser).notExpr, levelCompare: (*parser).compare, levelRange: (*parser).rangeExpr,
		levelAdditive: (*parser).leftAssoc, levelMultiplicative: (*parser).leftAssoc,
		levelUnary: (*parser).unary, levelPostfix: (*parser).postfix,
	}
}

// expr is lambda | coalesce (GRAMMAR.md §5.11, §6.3).
func (p *parser) expr() Expr {
	start := p.pos
	if !p.enter() {
		return p.badExpr(start)
	}
	defer p.leave()
	if p.lambdaAhead() {
		return p.lambda()
	}
	return p.level(levelCoalesce)
}

// headerExpr is an expr in header mode: at its depth 0 a "{" ends it (GRAMMAR.md §6.1).
func (p *parser) headerExpr() Expr {
	header := p.header
	p.header = true
	defer func() { p.header = header }()
	return p.expr()
}

func (p *parser) level(l uint8) Expr { return levelTable[l](p, l) }

func (p *parser) leftAssoc(l uint8) Expr {
	x := p.level(l + 1)
	for binaryLevel[p.kind()] == l {
		x = p.binary(x, l+1)
	}
	return x
}

// rightAssoc is coalesce = orExpr [ "??" coalesce ].
func (p *parser) rightAssoc(l uint8) Expr {
	x := p.level(l + 1)
	if binaryLevel[p.kind()] == l {
		return p.binary(x, l)
	}
	return x
}

// binary reads the operator at pos and its right operand at level next.
func (p *parser) binary(x Expr, next uint8) Expr {
	op := p.next()
	y := p.level(next)
	return &BinaryExpr{X: x, Op: p.toks[op].Kind, OpTok: op, Y: y, Bounds: p.from(x.First())}
}

// notExpr is "not" notExpr | compare: "not a in b" is "not (a in b)".
func (p *parser) notExpr(l uint8) Expr {
	if !p.at(KwNot) {
		return p.level(l + 1)
	}
	start := p.pos
	if !p.enter() {
		return p.badExpr(start)
	}
	defer p.leave()
	p.next()
	x := p.level(l)
	return &UnaryExpr{Op: KwNot, X: x, Bounds: p.from(start)}
}

// compare is range [ compOp range | "is" qualifiedWord ]; a second comparison is E1128.
func (p *parser) compare(l uint8) Expr {
	x := p.level(l + 1)
	for n := 0; binaryLevel[p.kind()] == l || p.at(KwIs); n++ {
		if n == 1 {
			diag.E1128.At(p.span(p.pos, p.pos)).Report(p.bag)
		}
		if p.at(KwIs) {
			x = p.isExpr(x)
			continue
		}
		x = p.binary(x, l+1)
	}
	return x
}

// isExpr is x "is" qualifiedWord (GRAMMAR.md §6.7).
func (p *parser) isExpr(x Expr) Expr {
	p.next()
	target := p.qualifiedWord()
	if target == nil {
		return p.badExpr(x.First())
	}
	return &IsExpr{X: x, Target: target, Bounds: p.from(x.First())}
}

// rangeExpr is additive [ rangeOp [ additive ] ] | rangeOp additive; an operand is left out
// when the next token cannot start one, and a second range is E1128.
func (p *parser) rangeExpr(l uint8) Expr {
	start := p.pos
	if binaryLevel[p.kind()] == l {
		op := p.next()
		hi := p.level(l + 1)
		return &RangeExpr{Op: p.toks[op].Kind, OpTok: op, Hi: hi, Bounds: p.from(start)}
	}
	x := p.level(l + 1)
	for n := 0; binaryLevel[p.kind()] == l; n++ {
		if n == 1 {
			diag.E1128.At(p.span(p.pos, p.pos)).Report(p.bag)
		}
		op := p.next()
		r := &RangeExpr{Lo: x, Op: p.toks[op].Kind, OpTok: op}
		if startsOperand[p.kind()] {
			r.Hi = p.level(l + 1)
		}
		r.Bounds = p.from(start)
		x = r
	}
	return x
}

// unary is "-" unary | postfix; "-" directly on a numeric literal is folded (GRAMMAR.md §2.4).
func (p *parser) unary(l uint8) Expr {
	if !p.at(TokMinus) {
		return p.level(l + 1)
	}
	start := p.pos
	if !p.enter() {
		return p.badExpr(start)
	}
	defer p.leave()
	p.next()
	folds := isNumber[p.kind()] && !startsPostfix[p.peek(1)]
	if folds {
		return negate(start, p.numLit(p.next(), true))
	}
	x := p.level(l)
	return &UnaryExpr{Op: TokMinus, X: x, Bounds: p.from(start)}
}

// lambdaAhead reports a lambda at pos: a binder or a parenthesized list before "=>" (GRM-12).
func (p *parser) lambdaAhead() bool {
	switch k := p.kind(); {
	case isWord(k) || k == TokUnderscore:
		return p.peek(1) == TokFatArrow
	case k == TokLParen:
		i := matchingParen(p.toks, int(p.pos))
		return i+1 < len(p.toks) && p.toks[i+1].Kind == TokFatArrow
	}
	return false
}

// lambda is lambdaParams "=>" expr, the body extending as far as possible.
func (p *parser) lambda() Expr {
	start := p.pos
	l := &LambdaExpr{}
	if p.at(TokLParen) {
		l.Parens = p.parenList(TokLParen, TokRParen, func() {
			if b := p.binder(); b != nil {
				l.Params = append(l.Params, b)
			}
		})
		if l.Parens.Close == NoTok {
			return p.badExpr(start)
		}
	} else {
		l.Params = []*Ident{p.binder()}
	}
	p.expect(TokFatArrow)
	l.Body = p.expr()
	l.Bounds = p.from(start)
	return l
}
