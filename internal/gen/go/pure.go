package gogen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// tr translates one body of the portable subset, one checked helper per operation (CONFORMANCE.md §2–§3).
type tr struct {
	g      *gen
	p      *pure
	scopes []*scope
}

// lines are the statements a body or a branch is translated into, each ending in a newline;
// go/format indents them.
type lines struct {
	l []string
}

func (b *lines) add(format string, args ...any) {
	b.put(fmt.Sprintf(format, args...))
}

// put adds statements already written.
func (b *lines) put(s ...string) {
	b.l = append(b.l, s...)
}

// scope is one Go block's lets, so a let no path reads gets `_ = x` (decision log, ir export fns review).
type scope struct {
	lets []*binding
}

// binding is a let: its Canon and Go names, the line after its declaration, whether it is read.
type binding struct {
	canon, local string
	at           int
	used         bool
}

// body is the fn's statements: a lone expression is its return, a Block its statements.
func (t *tr) body() string {
	var b lines
	t.open()
	t.stmts(t.p.fn.Body, &b)
	t.close(&b)
	return strings.Join(b.l, "")
}

func (t *tr) open() { t.scopes = append(t.scopes, &scope{}) }

// close ends the innermost scope: each let it bound that nothing read is marked used.
func (t *tr) close(b *lines) {
	sc := t.scopes[len(t.scopes)-1]
	t.scopes = t.scopes[:len(t.scopes)-1]
	for _, l := range slices.Backward(sc.lets) {
		if !l.used {
			b.l = slices.Insert(b.l, l.at, fmt.Sprintf(discardFormat, l.local))
		}
	}
}

// stmts translates n in statement position: a Block's statements, a let, an if returning from
// each branch, or the returned value with its exit checks.
func (t *tr) stmts(n ir.PExpr, b *lines) {
	switch x := n.(type) {
	case *ir.Block:
		t.block(x, b)
	case *ir.Let:
		t.let(x.Name, x.Value, b)
		t.stmts(x.Body, b)
	case *ir.If:
		b.add(ifOpenFormat, t.expr(x.Cond, b))
		t.branch(func(inner *lines) { t.stmts(x.Then, inner) }, b)
		b.add(closeBrace)
		t.stmts(x.Else, b)
	default:
		v := t.expr(n, b)
		b.add(returnLineFormat, t.g.exitChecks(t.p.fn.Result, t.p.fn.ResultRange, v))
	}
}

// block emits a Block's statements in order: a let is a local of the block, an if a Go block
// per branch falling through when it does not return, a return the result.
func (t *tr) block(x *ir.Block, b *lines) {
	for _, st := range x.Stmts {
		switch s := st.(type) {
		case *ir.LetStmt:
			t.let(s.Name, s.Value, b)
		case *ir.IfStmt:
			t.ifStmt(s, b)
		case *ir.ReturnStmt:
			t.stmts(s.X, b)
		default:
			t.g.failf(ErrMalformed, "statement %T in %s", st, t.g.at)
		}
	}
}

// ifStmt is `if c { … } else { … }`, each branch a Go block that scopes its lets.
func (t *tr) ifStmt(s *ir.IfStmt, b *lines) {
	b.add(ifOpenFormat, t.expr(s.Cond, b))
	t.chain(s, b)
}

// chain writes an if's branches and its closing brace; an else that is one if whose condition
// needs no statement is `} else if c {`.
func (t *tr) chain(s *ir.IfStmt, b *lines) {
	if s.Then == nil {
		t.g.failf(ErrMalformed, "if without then in %s", t.g.at)
		return
	}
	t.branch(func(inner *lines) { t.block(s.Then, inner) }, b)
	next := soleIf(s.Else)
	switch {
	case next != nil:
		var pre lines
		c := t.expr(next.Cond, &pre)
		if len(pre.l) == 0 {
			b.add(elseIfFormat, c)
			t.chain(next, b)
			return
		}
		b.add(elseLine)
		t.branch(func(inner *lines) {
			inner.put(pre.l...)
			inner.add(ifOpenFormat, c)
			t.chain(next, inner)
		}, b)
	case s.Else != nil:
		b.add(elseLine)
		t.branch(func(inner *lines) { t.block(s.Else, inner) }, b)
	}
	b.add(closeBrace)
}

// soleIf is the one if statement an else block holds, nil otherwise.
func soleIf(x *ir.Block) *ir.IfStmt {
	if x == nil || len(x.Stmts) != 1 {
		return nil
	}
	s, _ := x.Stmts[0].(*ir.IfStmt)
	return s
}

// branch translates a branch into a scope of its own, then appends it.
func (t *tr) branch(fill func(*lines), b *lines) {
	var inner lines
	t.open()
	fill(&inner)
	t.close(&inner)
	b.put(inner.l...)
}

// let declares a local of the current scope; a numeric constant is typed explicitly, since Go would give it int or float64 (CONFORMANCE.md §2.3: every integer is int64).
func (t *tr) let(name string, value ir.PExpr, b *lines) {
	local, ok := t.p.names[name]
	if !ok {
		t.g.failf(ErrMalformed, "let %s without its local in %s", name, t.g.at)
		return
	}
	b.put(t.decl(local, value, t.expr(value, b)))
	sc := t.scopes[len(t.scopes)-1]
	sc.lets = append(sc.lets, &binding{canon: name, local: local, at: len(b.l)})
}

// decl declares local as v: `local := v`, or `var local T = v` for an untyped constant.
func (t *tr) decl(local string, x ir.PExpr, v string) string {
	if untyped(x) {
		return fmt.Sprintf(varInitFormat, local, t.g.pureType(x.Type()), v)
	}
	return fmt.Sprintf(defineFormat, local, v)
}

// untyped reports an expression Go types as an untyped constant: a number, or min/max of integers.
func untyped(x ir.PExpr) bool {
	switch n := x.(type) {
	case *ir.Lit:
		return n.T.Kind == types.Int || n.T.Kind == types.Float || n.T.Kind == types.Duration
	case *ir.Call:
		return (n.Fn == ir.BuiltinMin || n.Fn == ir.BuiltinMax) && n.T.Kind != types.Float
	}
	return false
}

// local is the Go name of the innermost let bound to name, now read.
func (t *tr) local(name string) string {
	for _, sc := range slices.Backward(t.scopes) {
		for _, l := range slices.Backward(sc.lets) {
			if l.canon == name {
				l.used = true
				return l.local
			}
		}
	}
	t.g.failf(ErrMalformed, "a read of %s, which no let binds, in %s", name, t.g.at)
	return nilLit
}

// bind binds v, the Go form of x, to a fresh local so it is evaluated here.
func (t *tr) bind(x ir.PExpr, v string, b *lines) string {
	tmp := t.p.temp(t.g, tempLocal)
	b.put(t.decl(tmp, x, v))
	return tmp
}
