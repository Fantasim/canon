package syntax

import "github.com/fantasim/canonlang/internal/diag"

// stmtTable dispatches a statement on its first token; any other starts a simple statement.
var stmtTable [TokenKindCount]func(*parser) Stmt

func init() {
	stmtTable[KwLet], stmtTable[KwVar], stmtTable[KwIf] = (*parser).letStmt, (*parser).letStmt, (*parser).ifStmtItem
	stmtTable[KwFor], stmtTable[KwWhile], stmtTable[KwMatch] = (*parser).forStmt, (*parser).whileStmt, (*parser).matchStmt
	stmtTable[KwBreak], stmtTable[KwContinue] = (*parser).jumpStmt, (*parser).jumpStmt
	stmtTable[KwReturn], stmtTable[KwExpect] = (*parser).returnStmt, (*parser).expectStmt
}

// stmt is one statement; nil when it did not parse.
func (p *parser) stmt() Stmt {
	if f := stmtTable[p.kind()]; f != nil {
		return f(p)
	}
	return p.simpleStmt()
}

// simpleStmt is expr [ assignOp expr ] (GRAMMAR.md §5.10).
func (p *parser) simpleStmt() Stmt {
	x := p.expr()
	if !isAssign[p.kind()] {
		return &ExprStmt{X: x, Bounds: p.from(x.First())}
	}
	op := p.next()
	v := p.expr()
	return &AssignStmt{Target: x, Op: p.toks[op].Kind, OpTok: op, Value: v, Bounds: p.from(x.First())}
}

// letStmt is ( "let" | "var" ) IDENT [ ":" type ] "=" expr.
func (p *parser) letStmt() Stmt {
	start := p.pos
	isVar := p.next() != NoTok && p.toks[start].Kind == KwVar
	name := p.ident(diag.KindLocal)
	if name == nil {
		return nil
	}
	var t Type
	if p.accept(TokColon) != NoTok {
		t = p.typ()
	}
	if p.expect(TokAssign) == NoTok {
		return nil
	}
	v := p.expr()
	if isVar {
		return &VarStmt{Name: name, Type: t, Value: v, Bounds: p.from(start)}
	}
	return &LetStmt{Name: name, Type: t, Value: v, Bounds: p.from(start)}
}

func (p *parser) ifStmtItem() Stmt {
	if s := p.ifStmt(); s != nil {
		return s
	}
	return nil
}

// ifStmt is "if" headerExpr block [ "else" ( ifStmt | block ) ] (GRM-21).
func (p *parser) ifStmt() *IfStmt {
	start := p.next()
	s := &IfStmt{Cond: p.headerExpr()}
	if s.Then = p.block(); s.Then == nil {
		return nil
	}
	if p.accept(KwElse) == NoTok {
		s.Bounds = p.from(start)
		return s
	}
	if p.at(KwIf) {
		if s.ElseIf = p.ifStmt(); s.ElseIf == nil {
			return nil
		}
	} else if s.Else = p.block(); s.Else == nil {
		return nil
	}
	s.Bounds = p.from(start)
	return s
}

// forStmt is "for" binder [ "," binder ] "in" headerExpr block.
func (p *parser) forStmt() Stmt {
	start := p.next()
	s := &ForStmt{Vars: p.binders()}
	if len(s.Vars) == 0 || p.expect(KwIn) == NoTok {
		return nil
	}
	s.Iter = p.headerExpr()
	if s.Body = p.loopBody(); s.Body == nil {
		return nil
	}
	s.Bounds = p.from(start)
	return s
}

// whileStmt is "while" headerExpr block.
func (p *parser) whileStmt() Stmt {
	start := p.next()
	s := &WhileStmt{Cond: p.headerExpr()}
	if s.Body = p.loopBody(); s.Body == nil {
		return nil
	}
	s.Bounds = p.from(start)
	return s
}

func (p *parser) loopBody() *Block {
	p.ctx.loops++
	defer func() { p.ctx.loops-- }()
	return p.block()
}

// matchStmt is "match" headerExpr BraceList( stmtArm ); a "{" after "=>" is a block (GRM-19).
func (p *parser) matchStmt() Stmt {
	start := p.next()
	m := &MatchStmt{Scrutinee: p.headerExpr()}
	m.Braces = p.braceList(func() bool {
		at := p.pos
		arm := &StmtArm{Patterns: p.patterns()}
		if arm.Patterns == nil || p.expect(TokFatArrow) == NoTok {
			return false
		}
		if p.at(TokLBrace) {
			if arm.Block = p.block(); arm.Block == nil {
				return false
			}
		} else {
			arm.X = p.expr()
		}
		arm.Bounds = p.from(at)
		m.Arms = append(m.Arms, arm)
		return true
	}, nil)
	if m.Braces.Close == NoTok {
		return nil
	}
	m.Bounds = p.from(start)
	return m
}

// jumpStmt is "break" or "continue", E1134 outside a loop.
func (p *parser) jumpStmt() Stmt {
	t := p.next()
	if p.ctx.loops == 0 {
		diag.E1134.At(p.span(t, t), p.text(t)).Report(p.bag)
	}
	if p.toks[t].Kind == KwBreak {
		return &BreakStmt{Bounds: Bounds{From: t, To: t}}
	}
	return &ContinueStmt{Bounds: Bounds{From: t, To: t}}
}

// returnStmt is "return" [ expr ], the value absent before NL, "," or "}"; E1135 outside a
// function.
func (p *parser) returnStmt() Stmt {
	t := p.next()
	if !p.ctx.fn {
		diag.E1135.At(p.span(t, t)).Report(p.bag)
	}
	s := &ReturnStmt{}
	if k := p.kind(); k != TokNL && k != TokComma && k != TokRBrace && k != TokEOF {
		s.Value = p.expr()
	}
	s.Bounds = p.from(t)
	return s
}

// expectStmt is "expect" expr [ ( "fails" | "warns" ) ( stringLit | IDENT ) | "passes" ];
// E1130 outside a test block.
func (p *parser) expectStmt() Stmt {
	t := p.next()
	if !p.ctx.test {
		diag.E1130.At(p.span(t, t)).Report(p.bag)
	}
	s := &ExpectStmt{X: p.expr()}
	switch {
	case p.atWord(wordPasses):
		s.Outcome = p.ref()
	case p.atWord(wordFails) || p.atWord(wordWarns):
		s.Outcome = p.ref()
		if s.Message = p.expectMessage(); s.Message == nil {
			return nil
		}
	}
	s.Bounds = p.from(t)
	return s
}

func (p *parser) expectMessage() NameLit {
	if startsString[p.kind()] {
		if s := p.strLit(); s != nil {
			return s
		}
		return nil
	}
	if id := p.ref(); id != nil {
		return id
	}
	return nil
}
