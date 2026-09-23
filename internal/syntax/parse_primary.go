package syntax

import (
	"bytes"

	"github.com/fantasim/canonlang/internal/diag"
)

// primaryTable dispatches a primary on its first token and postfixTable a postfix step on its
// operator; a nil postfix result means "not a step here".
var (
	primaryTable  [TokenKindCount]func(*parser) Expr
	postfixTable  [TokenKindCount]func(*parser, Expr) Expr
	startsOperand [TokenKindCount]bool
)

func init() {
	for k := range primaryTable {
		switch {
		case IsNameable(TokenKind(k)) || TokenKind(k) == TokIdent:
			primaryTable[k] = (*parser).identExpr
		case startsString[k] || TokenKind(k) == TokRaw || TokenKind(k) == TokRawML:
			primaryTable[k] = (*parser).stringExpr
		case isNumber[k]:
			primaryTable[k] = (*parser).numberExpr
		}
	}
	primaryTable[TokRegex], primaryTable[KwTrue], primaryTable[KwFalse] = (*parser).regexExpr, (*parser).boolExpr, (*parser).boolExpr
	primaryTable[KwNone], primaryTable[KwSelf], primaryTable[TokLParen] = (*parser).noneExpr, (*parser).selfExpr, (*parser).parenExpr
	primaryTable[TokDot], primaryTable[TokLBrack], primaryTable[TokLBrace] = (*parser).shorthand, (*parser).listLit, (*parser).braceExpr
	primaryTable[KwIf], primaryTable[KwMatch], primaryTable[KwLoad] = (*parser).ifExpr, (*parser).matchExpr, (*parser).loadExpr
	postfixTable[TokDot], postfixTable[TokOptDot] = (*parser).selector, (*parser).selector
	postfixTable[TokLBrack], postfixTable[TokLParen] = (*parser).index, (*parser).call
	postfixTable[TokBang], postfixTable[TokLBrace] = (*parser).force, (*parser).typedLit
	for k, f := range primaryTable {
		startsOperand[k] = f != nil || TokenKind(k) == TokMinus
	}
}

// postfix is primary { postfixOp }.
func (p *parser) postfix(uint8) Expr {
	return p.steps(p.primary())
}

// steps applies the postfix steps that follow x.
func (p *parser) steps(x Expr) Expr {
	for f := postfixTable[p.kind()]; f != nil; f = postfixTable[p.kind()] {
		y := f(p, x)
		if y == nil {
			return x
		}
		x = y
	}
	return x
}

// primary reads one primary; anything else is E1116 and a BadExpr (GRAMMAR.md §10).
func (p *parser) primary() Expr {
	if f := primaryTable[p.kind()]; f != nil {
		return f(p)
	}
	start := p.pos
	p.fail(exprName)
	if !stopToken[p.kind()] && !p.topSync() {
		p.next()
	}
	return p.badExpr(start)
}

func (p *parser) identExpr() Expr {
	t := p.next()
	return &IdentExpr{Bounds: Bounds{From: t, To: t}, Name: p.text(t)}
}

func (p *parser) stringExpr() Expr {
	start := p.pos
	if s := p.strLit(); s != nil {
		return s
	}
	return p.badExpr(start)
}

func (p *parser) numberExpr() Expr { return p.numLit(p.next(), false) }

// regexExpr is a REGEX where one is allowed; anywhere else it is E1115 (GRAMMAR.md §2.7).
func (p *parser) regexExpr() Expr {
	t := p.next()
	body := bytes.TrimSuffix(p.src.Content[p.toks[t].Start+1:p.toks[t].End], []byte(slashText))
	r := &RegexLit{Bounds: Bounds{From: t, To: t}, Pattern: regexPattern(body)}
	if p.regexOK {
		p.regex = r
	} else {
		diag.E1115.At(p.span(t, t)).Report(p.bag)
	}
	p.regexOK = false
	return r
}

func (p *parser) boolExpr() Expr {
	t := p.next()
	return &BoolLit{Bounds: Bounds{From: t, To: t}, Value: p.toks[t].Kind == KwTrue}
}

func (p *parser) noneExpr() Expr {
	t := p.next()
	return &NoneLit{Bounds: Bounds{From: t, To: t}}
}

func (p *parser) selfExpr() Expr {
	t := p.next()
	return &SelfExpr{Bounds: Bounds{From: t, To: t}}
}

// parenExpr is "(" expr ")".
func (p *parser) parenExpr() Expr {
	start := p.next()
	x := p.inBrackets(p.expr)
	if p.expect(TokRParen) == NoTok {
		return p.badExpr(start)
	}
	return &ParenExpr{X: x, Bounds: p.from(start)}
}

// shorthand is "." WORD { postfixOp }: the whole chain is the body of an implicit lambda.
func (p *parser) shorthand() Expr {
	start := p.pos
	root := p.selector(nil)
	if root == nil {
		return p.badExpr(start)
	}
	body := p.steps(root)
	return &ShorthandLambda{Body: body, Bounds: p.from(start)}
}

// braceExpr is a brace literal; at depth 0 of a header it is E1129 (GRAMMAR.md §6.1).
func (p *parser) braceExpr() Expr {
	return p.operand(func() Expr {
		start := p.pos
		if b := p.braceLit(); b != nil {
			return b
		}
		return p.badExpr(start)
	})
}

// operand reads an if, a match or a brace literal where an operand is expected; at depth 0 of
// a header it is E1129 and read as if parenthesized.
func (p *parser) operand(parse func() Expr) Expr {
	header := p.header
	if header {
		diag.E1129.At(p.span(p.pos, p.pos)).Report(p.bag)
		p.header = false
	}
	defer func() { p.header = header }()
	return parse()
}

// selector is "." WORD or "?." WORD after x (nil x: the root of a shorthand lambda).
func (p *parser) selector(x Expr) Expr {
	start := p.pos
	if x != nil {
		start = x.First()
	}
	op := p.next()
	name := p.word()
	if name == nil {
		return p.badExpr(start)
	}
	return &SelectorExpr{X: x, Optional: p.toks[op].Kind == TokOptDot, Name: name, Bounds: p.from(start)}
}

// index is x "[" expr "]".
func (p *parser) index(x Expr) Expr {
	open := p.next()
	e := &IndexExpr{X: x, Brackets: Delims{Open: open}}
	e.Index = p.inBrackets(p.expr)
	if e.Brackets.Close = p.expect(TokRBrack); e.Brackets.Close == NoTok {
		return p.badExpr(x.First())
	}
	e.Bounds = p.from(x.First())
	return e
}

func (p *parser) force(x Expr) Expr {
	p.next()
	return &ForceExpr{X: x, Bounds: p.from(x.First())}
}

// typedLit is qualifiedWord braceLit, the "{" on the name's line and never at depth 0 of a
// header; nil when x is not a name or the "{" does not qualify.
func (p *parser) typedLit(x Expr) Expr {
	name := qualifiedOf(x)
	if name == nil || p.header || !p.sameLine(p.pos-1, p.pos) {
		return nil
	}
	lit := p.braceLit()
	if lit == nil {
		return p.badExpr(x.First())
	}
	return &TypedLit{Type: name, Lit: lit, Bounds: p.from(x.First())}
}

// qualifiedOf is the dotted name x spells: a name, or "." steps down from one.
func qualifiedOf(x Expr) *QualifiedName {
	switch x := x.(type) {
	case *IdentExpr:
		id := &Ident{Bounds: x.Bounds, Name: x.Name}
		return &QualifiedName{Bounds: x.Bounds, Parts: []*Ident{id}}
	case *SelectorExpr:
		if x.X == nil || x.Optional {
			return nil
		}
		if q := qualifiedOf(x.X); q != nil {
			q.Parts = append(q.Parts, x.Name)
			q.Bounds = x.Bounds
			return q
		}
	}
	return nil
}
