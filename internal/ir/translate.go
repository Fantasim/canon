package ir

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// translator turns one translated fn's body into IR: scan translates each expression once, body then builds the statements; refused marks a finding, broken what the checker reported, badRes a refused result, whose conversions are not judged.
type translator struct {
	s        *stage
	u        *unit
	site     *fnSite
	file     *syntax.File
	exprs    map[syntax.Expr]PExpr
	reads    map[string]*pendingRead
	order    []*pendingRead
	uses     []readUse
	reported map[source.Span]bool
	letOf    map[syntax.Expr]*syntax.LetStmt
	inside   int // > 0 while scanning inside a refused construct: no further E9001 (decision 194)
	badRes   bool
	refused  bool
	broken   bool
}

// translateFns fills Body and Reads of u's translated fns, or refuses each one outside the portable subset (CONFORMANCE.md §2, DECISIONS 37, 204).
func (s *stage) translateFns(u *unit) {
	for _, site := range s.ownFns(u) {
		if site.fn.Kind != FnTranslated || s.info.Broken[site.obj] || site.decl.Body == nil {
			continue
		}
		paramsOK, resultOK := s.checkTranslatedParams(u, site), s.checkTranslatedResult(u, site)
		t := &translator{
			s: s, u: u, site: site, file: site.obj.File(), exprs: map[syntax.Expr]PExpr{},
			reads: map[string]*pendingRead{}, reported: map[source.Span]bool{}, letOf: map[syntax.Expr]*syntax.LetStmt{}, badRes: !resultOK,
		}
		t.scan(site.decl.Body.Stmts)
		if t.broken && !t.refused && s.errorless() {
			site.fn.Err = fmt.Errorf(fmtUntranslated, ErrInternal, site.label)
		}
		if !paramsOK || !resultOK || t.refused || t.broken || !t.nameReads() {
			continue
		}
		site.fn.Body = t.body(site.decl.Body.Stmts)
		site.fn.Reads = t.finishReads()
	}
}

// errorless reports that no bag holds an error: a fn the checker left untranslatable then breaks an invariant (decision 196).
func (s *stage) errorless() bool {
	n := 0
	for _, bag := range s.in.Bags { //canon:unordered a sum
		n += bag.Summary().Errors
	}
	return n == 0
}

// scan translates the expressions of every statement, dead ones included, and refuses any statement but `let`, `if` and `return` (CONFORMANCE.md §2.2).
func (t *translator) scan(stmts []syntax.Stmt) {
	for _, st := range stmts {
		switch x := st.(type) {
		case *syntax.LetStmt:
			t.letOf[syntax.Unparen(x.Value)] = x
			t.expr(x.Value)
			t.typedLet(x)
		case *syntax.IfStmt:
			t.scanIf(x)
		case *syntax.ReturnStmt:
			if x.Value == nil {
				t.broken = true // E3019
				continue
			}
			t.expr(x.Value)
		case *syntax.BadStmt:
			t.broken = true
		default:
			t.refuse(st)
			t.within(func() { t.scanStmt(st) })
		}
	}
}

// scanStmt translates the expressions and nested statements of a refused statement.
func (t *translator) scanStmt(st syntax.Node) {
	for c := range syntax.Children(st) {
		switch x := c.(type) {
		case syntax.Expr:
			t.expr(x)
		case *syntax.Block:
			t.scan(x.Stmts)
		case *syntax.StmtArm:
			t.scanStmt(x)
		}
	}
}

// within runs f inside a refused construct: what f finds reports every code but E9001.
func (t *translator) within(f func()) {
	t.inside++
	f()
	t.inside--
}

func (t *translator) scanIf(x *syntax.IfStmt) {
	for c := x; c != nil; c = c.ElseIf {
		t.expr(c.Cond)
		if c.Then != nil {
			t.scan(c.Then.Stmts)
		}
		if c.Else != nil {
			t.scan(c.Else.Stmts)
		}
	}
}

// typedLet refuses a `let` whose declared type checks the stored value, which no IR node does: a sized integer, Float32, a Duration, a refinement (EVALUATION.md §4.3).
func (t *translator) typedLet(x *syntax.LetStmt) {
	if x.Type == nil {
		return
	}
	obj := t.s.info.Defs[x.Name]
	if obj == nil || obj.Type() == nil {
		t.broken = true
		return
	}
	if storeChecks(obj.Type()) {
		t.refuse(x)
	}
}

// storeChecks reports a type whose store checks the value: a sized integer, Float32, a Duration, or any written refinement; a `past` alone checks nothing (TYPES.md §8.4).
func storeChecks(ty types.Type) bool {
	if r, refined := ty.Underlying().(*types.Refined); refined {
		return r.Range != nil || r.Pattern != nil || r.Where != nil || r.Asset != nil || storeChecks(r.Of)
	}
	b, ok := ty.Base().(types.Basic)
	if !ok {
		return false
	}
	switch b.K {
	case types.Int:
		return b != types.IntType
	case types.Float:
		return b != types.FloatType
	default:
		return b.K == types.Duration
	}
}

// body is the fn's Body: the value of a lone `return`, else its statements as a Block.
func (t *translator) body(stmts []syntax.Stmt) PExpr {
	if len(stmts) == 1 {
		if r, ok := stmts[0].(*syntax.ReturnStmt); ok {
			return t.exprs[syntax.Unparen(r.Value)]
		}
	}
	return t.block(stmts)
}

// block is stmts as IR statements, one per source statement, up to the first `return`: the IR stays linear in the source.
func (t *translator) block(stmts []syntax.Stmt) *Block {
	b := &Block{T: t.site.fn.Result}
	for _, st := range stmts {
		switch x := st.(type) {
		case *syntax.LetStmt:
			b.Stmts = append(b.Stmts, &LetStmt{Name: x.Name.Name, Value: t.exprs[syntax.Unparen(x.Value)]})
		case *syntax.IfStmt:
			s := t.ifStmt(x)
			b.Stmts = append(b.Stmts, s)
			if returns(s) {
				return b
			}
		case *syntax.ReturnStmt:
			b.Stmts = append(b.Stmts, &ReturnStmt{X: t.exprs[syntax.Unparen(x.Value)]})
			return b
		}
	}
	return b
}

// returns reports an `if` every branch of which returns: what follows is unreachable.
func returns(s *IfStmt) bool {
	return s.Else != nil && blockReturns(s.Then) && blockReturns(s.Else)
}

// blockReturns reports a Block that returns on every path, which block ends right there.
func blockReturns(b *Block) bool {
	if len(b.Stmts) == 0 {
		return false
	}
	switch s := b.Stmts[len(b.Stmts)-1].(type) {
	case *ReturnStmt:
		return true
	case *IfStmt:
		return returns(s)
	}
	return false
}

// ifStmt is an `if` statement; an `else if` is an Else block holding the next IfStmt.
func (t *translator) ifStmt(x *syntax.IfStmt) *IfStmt {
	out := &IfStmt{Cond: t.exprs[syntax.Unparen(x.Cond)], Then: &Block{T: t.site.fn.Result}}
	if x.Then != nil {
		out.Then = t.block(x.Then.Stmts)
	}
	switch {
	case x.ElseIf != nil:
		out.Else = &Block{T: t.site.fn.Result, Stmts: []Stmt{t.ifStmt(x.ElseIf)}}
	case x.Else != nil:
		out.Else = t.block(x.Else.Stmts)
	}
	return out
}

// refuse reports n as outside the portable subset (E9001), once per construct.
func (t *translator) refuse(n syntax.Node) {
	if t.inside > 0 {
		t.refused = true
		return
	}
	t.report(t.headSpan(n), func(sp source.Span) *diag.Builder { return diag.E9001.At(sp, sp, t.site.label) })
}

// report reports one finding at sp unless one was already, and refuses the fn.
func (t *translator) report(sp source.Span, b func(source.Span) *diag.Builder) {
	t.refused = true
	if t.reported[sp] {
		return
	}
	t.reported[sp] = true
	t.u.report(b(sp))
}

// headSpan names a construct in a message: a block construct by its head (`for x in xs`,
// `while c`, `match x`), anything else whole.
func (t *translator) headSpan(n syntax.Node) source.Span {
	var tail syntax.Node
	switch x := n.(type) {
	case *syntax.ForStmt:
		tail = x.Iter
	case *syntax.WhileStmt:
		tail = x.Cond
	case *syntax.MatchStmt:
		tail = x.Scrutinee
	case *syntax.MatchExpr:
		tail = x.Scrutinee
	}
	sp := t.file.Span(n)
	if tail != nil {
		sp.End = t.file.Span(tail).End
	}
	return sp
}
