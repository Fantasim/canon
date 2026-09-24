package eval

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// flow is how a statement completes.
type flow uint8

// stmtFn executes one kind of statement.
type stmtFn func(r *run, s syntax.Stmt) flow

// stmtTable dispatches on the statement kind; filled in init, its handlers running blocks.
var stmtTable [syntax.NodeKindCount]stmtFn

func init() {
	stmtTable = [syntax.NodeKindCount]stmtFn{
		syntax.KindLetStmt: execLocal, syntax.KindVarStmt: execLocal, syntax.KindAssignStmt: execAssign,
		syntax.KindIfStmt: execIf, syntax.KindForStmt: execFor, syntax.KindWhileStmt: execWhile,
		syntax.KindBreakStmt: execBreak, syntax.KindContinueStmt: execContinue,
		syntax.KindReturnStmt: execReturn, syntax.KindExpectStmt: execExpect,
		syntax.KindMatchStmt: execMatch, syntax.KindExprStmt: execExpr,
	}
}

// block runs statements in order; each executed statement costs a step (EVALUATION.md §12.1).
func (r *run) block(b *syntax.Block) flow {
	if b == nil {
		return flowNext
	}
	for _, s := range b.Stmts {
		fn := stmtTable[s.Kind()]
		if fn == nil {
			r.bug(s)
		}
		if r.failed || !r.step(s) {
			return flowAbort
		}
		if f := fn(r, s); f != flowNext {
			return f
		}
	}
	return flowNext
}

// orAbort is flowNext when v was computed, flowAbort when the root was aborted.
func orAbort(v value.Value) flow {
	if v == nil {
		return flowAbort
	}
	return flowNext
}

// execLocal is `let` or `var`: a declared type is a storage point (TYPES.md §6.2).
func execLocal(r *run, s syntax.Stmt) flow {
	var name *syntax.Ident
	var t syntax.Type
	var x syntax.Expr
	switch d := s.(type) {
	case *syntax.LetStmt:
		name, t, x = d.Name, d.Type, d.Value
	case *syntax.VarStmt:
		name, t, x = d.Name, d.Type, d.Value
	}
	v := r.eval(x)
	if t != nil {
		if obj := r.ev.info.Defs[name]; obj != nil {
			v = r.store(v, obj.Type(), declSite(r.fr.file, name, t), nil)
		}
	}
	if v == nil || !r.bind(name, v) {
		return flowAbort
	}
	return flowNext
}

func execIf(r *run, s syntax.Stmt) flow {
	for x := s.(*syntax.IfStmt); x != nil; x = x.ElseIf {
		c, ok := r.truth(x.Cond)
		switch {
		case !ok:
			return flowAbort
		case c:
			return r.block(x.Then)
		case x.Else != nil:
			return r.block(x.Else)
		}
	}
	return flowNext
}

// execFor iterates a snapshot of its collection; each entry into the body is a step.
func execFor(r *run, s syntax.Stmt) flow {
	x := s.(*syntax.ForStmt)
	coll := r.eval(x.Iter)
	if coll == nil {
		return flowAbort
	}
	out := flowNext
	f := r.each(x, coll, x.Vars, func() bool {
		switch b := r.block(x.Body); b {
		case flowNext, flowContinue:
			return true
		case flowReturn:
			out = flowReturn
		default:
		}
		return false
	})
	if f == flowAbort {
		return flowAbort
	}
	return out
}

func execWhile(r *run, s syntax.Stmt) flow {
	x := s.(*syntax.WhileStmt)
	for {
		c, ok := r.truth(x.Cond)
		if !ok {
			return flowAbort
		}
		if !c {
			return flowNext
		}
		if !r.step(x) {
			return flowAbort
		}
		switch f := r.block(x.Body); f {
		case flowBreak:
			return flowNext
		case flowReturn, flowAbort:
			return f
		default:
		}
	}
}

func execBreak(*run, syntax.Stmt) flow { return flowBreak }

func execContinue(*run, syntax.Stmt) flow { return flowContinue }

// execReturn sets the frame's result; a bare return belongs to a test or a check.
func execReturn(r *run, s syntax.Stmt) flow {
	if x := s.(*syntax.ReturnStmt).Value; x != nil {
		if r.fr.ret = r.eval(x); r.fr.ret == nil {
			return flowAbort
		}
	}
	return flowReturn
}

func execMatch(r *run, s syntax.Stmt) flow {
	x := s.(*syntax.MatchStmt)
	v := r.eval(x.Scrutinee)
	if v == nil {
		return flowAbort
	}
	pats := make([][]*syntax.Pattern, len(x.Arms))
	for i, a := range x.Arms {
		pats[i] = a.Patterns
	}
	i := r.arm(r.ev.info.Matches[x], pats, v)
	if i < 0 {
		return flowAbort
	}
	if a := x.Arms[i]; a.Block != nil {
		return r.block(a.Block)
	}
	return orAbort(r.eval(x.Arms[i].X))
}

func execExpr(r *run, s syntax.Stmt) flow {
	return orAbort(r.eval(s.(*syntax.ExprStmt).X))
}
