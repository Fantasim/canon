package format

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

// block is a statement block: single-line only when written on one line with exactly one
// statement and no comment.
func (b *builder) block(n *syntax.Block) *doc {
	br := b.blockBranch(nil, n)
	return listGroup(!b.breaks(br), b.braceParts(br.open, br.close, br.items))
}

// branch is one block of an if chain: its condition (nil for the last "else") and its body.
type branch struct {
	cond        syntax.Expr
	open, close syntax.Tok
	items       []entry
	many        bool
}

// breaks reports a block that is not single-line: several statements, a comment, or items
// written on several lines; an empty block is "{}".
func (b *builder) breaks(br branch) bool {
	return br.many || !b.single(br.open, br.close, br.items)
}

// ifChain is an if chain as one group: every block breaks when one does not qualify for a
// single line; "} else {" stays on one line.
func (b *builder) ifChain(first syntax.Tok, branches []branch) *doc {
	force := false
	for _, br := range branches {
		force = force || b.breaks(br)
	}
	return listGroup(!force, b.links(first, branches))
}

// links is the chain from branches[0], whose keyword is kw. A comment that ends the line of a
// "}" or precedes an "else" puts that "else" on a line one level deeper, with what follows it;
// one that ends the line of an "else" does the same for what follows the "else".
func (b *builder) links(kw syntax.Tok, brs []branch) *doc {
	var ds []*doc
	if brs[0].cond != nil {
		ds = append(ds, b.tok(kw), spaceDoc, b.node(brs[0].cond), spaceDoc)
	}
	ds = append(ds, b.braceParts(brs[0].open, brs[0].close, brs[0].items))
	if len(brs) == 1 {
		return cat(ds...)
	}
	els := b.after(brs[0].close)
	next := b.links(b.after(els), brs[1:])
	rest := cat(b.tok(els), spaceDoc, next)
	if b.lineEnds(els) {
		rest = cat(b.tok(els), indent(hardlineDoc, next))
	}
	if b.lineEnds(brs[0].close) || len(b.notes[els].lead) > 0 {
		return cat(append(ds, indent(hardlineDoc, rest))...)
	}
	return cat(append(ds, spaceDoc, rest)...)
}

func (b *builder) ifStmt(n *syntax.IfStmt) *doc {
	var brs []branch
	for s := n; s != nil; s = s.ElseIf {
		brs = append(brs, b.blockBranch(s.Cond, s.Then))
		if s.Else != nil {
			brs = append(brs, b.blockBranch(nil, s.Else))
		}
	}
	return b.ifChain(n.First(), brs)
}

func (b *builder) blockBranch(cond syntax.Expr, blk *syntax.Block) branch {
	return branch{cond: cond, open: blk.First(), close: blk.Last(), items: entries(b, blk.Stmts), many: len(blk.Stmts) > 1}
}

func (b *builder) ifExpr(n *syntax.IfExpr) *doc {
	var brs []branch
	for s := n; s != nil; s = s.ElseIf {
		brs = append(brs, b.bodyBranch(s.Cond, s.Then))
		if s.Else != nil {
			brs = append(brs, b.bodyBranch(nil, s.Else))
		}
	}
	return b.ifChain(n.First(), brs)
}

func (b *builder) bodyBranch(cond syntax.Expr, body *syntax.ExprBody) branch {
	return branch{cond: cond, open: body.First(), close: body.Last(), items: entries(b, []syntax.Expr{body.X})}
}

func (b *builder) forStmt(n *syntax.ForStmt) *doc {
	return cat(b.tok(n.First()), spaceDoc, b.commaList(nodes(n.Vars)), spaceDoc, b.tok(b.before(n.Iter.First())),
		spaceDoc, b.node(n.Iter), spaceDoc, b.node(n.Body))
}

func (b *builder) whileStmt(n *syntax.WhileStmt) *doc {
	return cat(b.tok(n.First()), spaceDoc, b.node(n.Cond), spaceDoc, b.node(n.Body))
}

// matchList is "match header { arms }", in statement, expression or type position.
func (b *builder) matchList(first syntax.Tok, scrutinee syntax.Expr, braces syntax.Delims, arms []entry) *doc {
	return cat(b.tok(first), spaceDoc, b.node(scrutinee), spaceDoc, b.braceList(braces.Open, braces.Close, arms))
}

func (b *builder) matchStmt(n *syntax.MatchStmt) *doc {
	return b.matchList(n.First(), n.Scrutinee, n.Braces, entries(b, n.Arms))
}

func (b *builder) matchExpr(n *syntax.MatchExpr) *doc {
	return b.matchList(n.First(), n.Scrutinee, n.Braces, entries(b, n.Arms))
}

func (b *builder) matchType(n *syntax.MatchType) *doc {
	return b.matchList(b.first(n), n.Scrutinee, n.Braces, entries(b, n.Arms))
}

// arm is "patterns => body" by rule A (§7.2).
func (b *builder) arm(patterns []*syntax.Pattern, body syntax.Node) *doc {
	arrow := b.after(patterns[len(patterns)-1].Last())
	return cat(b.commaList(nodes(patterns)), b.assign(arrow, body))
}

func (b *builder) stmtArm(n *syntax.StmtArm) *doc {
	if n.Block != nil {
		return b.arm(n.Patterns, n.Block)
	}
	return b.arm(n.Patterns, n.X)
}

func (b *builder) letStmt(n *syntax.LetStmt) *doc {
	return cat(b.tok(n.First()), spaceDoc, b.binding(n.Name, n.Type, n.Value))
}

func (b *builder) varStmt(n *syntax.VarStmt) *doc {
	return cat(b.tok(n.First()), spaceDoc, b.binding(n.Name, n.Type, n.Value))
}

func (b *builder) assignStmt(n *syntax.AssignStmt) *doc {
	return cat(b.node(n.Target), b.assign(n.OpTok, n.Value))
}

func (b *builder) returnStmt(n *syntax.ReturnStmt) *doc {
	if n.Value == nil {
		return b.tok(n.First())
	}
	return cat(b.tok(n.First()), spaceDoc, b.node(n.Value))
}

// expectStmt keeps its verdict on the line of the subject's end (§7.2).
func (b *builder) expectStmt(n *syntax.ExpectStmt) *doc {
	ds := []*doc{b.tok(n.First()), spaceDoc, b.node(n.X)}
	if n.Outcome != nil {
		ds = append(ds, spaceDoc, b.node(n.Outcome))
	}
	if n.Message != nil {
		ds = append(ds, spaceDoc, b.node(n.Message))
	}
	return cat(ds...)
}
