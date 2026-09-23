package syntax

import "github.com/fantasim/canonlang/internal/diag"

// call is fun callArgs, callArgs being ParenList( arg ); a REGEX is allowed as the only
// argument of a method named matches.
func (p *parser) call(fun Expr) Expr {
	c := &CallExpr{Fun: fun}
	sel, ok := fun.(*SelectorExpr)
	p.regexOK, p.regex = ok && sel.Name.Name == matchesName && p.peek(1) == TokRegex, nil
	c.Parens = p.parenList(TokLParen, TokRParen, func() { c.Args = append(c.Args, p.arg()) })
	p.regexOK = false
	if c.Parens.Close == NoTok {
		return p.badExpr(fun.First())
	}
	values := make([]Expr, len(c.Args))
	for i, a := range c.Args {
		values[i] = a.Value
	}
	p.loneRegex(values)
	p.checkArgs(c.Args)
	if id, ok := fun.(*IdentExpr); ok && !p.ctx.check && (id.Name == failName || id.Name == KwWarn.String()) {
		diag.E1105.At(p.nodeSpan(id), id.Name).Report(p.bag)
	}
	c.Bounds = p.from(fun.First())
	return c
}

// arg is [ IDENT ":" ] expr (GRAMMAR.md §6.7).
func (p *parser) arg() *Arg {
	start := p.pos
	a := &Arg{}
	if p.at(TokIdent) && p.peek(1) == TokColon {
		a.Name = p.ref()
		p.next()
	}
	a.Value = p.expr()
	a.Bounds = p.from(start)
	return a
}

// checkArgs reports a positional argument after a named one, and a name given twice (E1121).
func (p *parser) checkArgs(args []*Arg) {
	named := map[string]bool{}
	for _, a := range args {
		switch {
		case a.Name == nil && len(named) > 0:
			diag.E1121.AtOrder(p.nodeSpan(a)).Report(p.bag)
		case a.Name != nil && named[a.Name.Name]:
			diag.E1121.AtTwice(p.nodeSpan(a), a.Name.Name).Report(p.bag)
		case a.Name != nil:
			named[a.Name.Name] = true
		}
	}
}

// loadExpr is "load" [ "." IDENT ] callArgs (GRAMMAR.md §5.11).
func (p *parser) loadExpr() Expr {
	start := p.next()
	l := &LoadExpr{}
	if p.accept(TokDot) != NoTok {
		if l.Method = p.ref(); l.Method == nil {
			return p.badExpr(start)
		}
	}
	l.Parens = p.parenList(TokLParen, TokRParen, func() { l.Args = append(l.Args, p.arg()) })
	if l.Parens.Close == NoTok {
		return p.badExpr(start)
	}
	p.checkArgs(l.Args)
	l.Bounds = p.from(start)
	return l
}

// ifExpr is "if" headerExpr exprBody { "else" "if" headerExpr exprBody } "else" exprBody.
func (p *parser) ifExpr() Expr {
	return p.operand(func() Expr {
		start := p.pos
		if e := p.ifNode(); e != nil {
			return e
		}
		return p.badExpr(start)
	})
}

func (p *parser) ifNode() *IfExpr {
	start := p.next()
	e := &IfExpr{Cond: p.headerExpr()}
	if e.Then = p.exprBody(); e.Then == nil || p.expect(KwElse) == NoTok {
		return nil
	}
	if p.at(KwIf) {
		if e.ElseIf = p.ifNode(); e.ElseIf == nil {
			return nil
		}
	} else if e.Else = p.exprBody(); e.Else == nil {
		return nil
	}
	e.Bounds = p.from(start)
	return e
}

// exprBody is "{" [ SepRun ] expr [ SepRun ] "}".
func (p *parser) exprBody() *ExprBody {
	start := p.pos
	if p.expect(TokLBrace) == NoTok {
		return nil
	}
	b := &ExprBody{}
	p.sepRun()
	b.X = p.inBrackets(p.expr)
	p.sepRun()
	if p.expect(TokRBrace) == NoTok {
		return nil
	}
	b.Bounds = p.from(start)
	return b
}

// matchExpr is "match" headerExpr BraceList( exprArm ), exprArm being patterns "=>" expr; a
// "{" after "=>" starts a brace literal (GRM-19).
func (p *parser) matchExpr() Expr {
	return p.operand(func() Expr {
		start := p.next()
		m := &MatchExpr{Scrutinee: p.headerExpr()}
		m.Braces = p.braceList(func() bool {
			at := p.pos
			arm := &MatchArm{Patterns: p.patterns()}
			if arm.Patterns == nil || p.expect(TokFatArrow) == NoTok {
				return false
			}
			arm.Body = p.expr()
			arm.Bounds = p.from(at)
			m.Arms = append(m.Arms, arm)
			return true
		}, nil)
		if m.Braces.Close == NoTok {
			return p.badExpr(start)
		}
		m.Bounds = p.from(start)
		return m
	})
}
