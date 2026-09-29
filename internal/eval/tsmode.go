package eval

import (
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// runtimeInput reports a translated fn: a parameter not Bool, an enum or a ref to an unkeyed collection (SPEC §9.4).
func runtimeInput(obj check.Object) bool {
	ft, ok := obj.Type().(*types.FuncType)
	return ok && slices.ContainsFunc(ft.Params, func(p types.Type) bool {
		if rt, isRef := p.Base().(*types.RefType); isRef {
			return rt.Target == nil || rt.Target.KeyedBy != nil
		}
		return p.Base().Kind() != types.Bool && p.Base().Kind() != types.Enum
	})
}

// tsEntry checks every integer read from self, then every argument, is safe: E8303 (CONFORMANCE.md §2.3, §4).
func (r *run) tsEntry(fr *frame, d *syntax.FnDecl, args []value.Value) bool {
	saved := r.fr
	r.fr = fr
	r.ev.depth++
	ok := r.tsReads(d)
	r.fr = saved
	r.ev.depth--
	if ok && slices.ContainsFunc(args, unsafeInt) {
		r.tsFail()
		return false
	}
	return ok
}

// tsReads reads each path of self d's body reads once, on entry, paying there what it costs, a
// value it first forces included; the body reuses the value for free (decisions log, round 2).
func (r *run) tsReads(d *syntax.FnDecl) bool {
	r.fr.reads = map[syntax.Expr]selfRead{}
	for _, x := range r.ev.readsOf(d) {
		v := r.node(x, nil)
		read := selfRead{v: v, none: r.chainNone}
		r.chainNone = false
		if r.failed || v == nil && !read.none {
			return false
		}
		r.fr.reads[x] = read
		if !read.none && r.tsRead(v) == nil {
			return false
		}
	}
	return true
}

// selfRead is a path of self read on entry: its value, or none through `?.`.
type selfRead struct {
	v    value.Value
	none bool
}

// tsRead is v, an integer crossing TS-mode code: E8303 unless it is safe (CONFORMANCE.md §4).
func (r *run) tsRead(v value.Value) value.Value {
	if unsafeInt(v) {
		r.tsFail()
		return nil
	}
	return v
}

// tsFail stops the vector with E8303, unless an error came first.
func (r *run) tsFail() {
	if s := r.ev.vec; s != nil && s.first == "" && s.limit == NoLimit {
		s.first = diag.E8303.Def().Code
	}
	r.ev.exhausted, r.failed = true, true
}

// unsafeInt reports an Int or Duration outside ±(2^53 − 1).
func unsafeInt(v value.Value) bool {
	n, ok := intLike(v)
	return ok && (n > maxSafeInt || n < -maxSafeInt)
}

// intLike is the int64 an Int or a Duration (in milliseconds) holds.
func intLike(v value.Value) (int64, bool) {
	switch x := v.(type) {
	case *value.Int:
		return x.V, true
	case *value.Dur:
		return x.Ms, true
	}
	return 0, false
}

// overflows reports Int or Duration `a op b` beyond int64: E8303, not E4101, in TS mode (CONFORMANCE.md §4).
func overflows(op std.Op, a, b value.Value) bool {
	x, okA := intLike(a)
	y, okB := intLike(b)
	if !okA || !okB {
		return false
	}
	switch op {
	case std.OpAdd:
		s := x + y
		return (x^s)&(y^s) < 0
	case std.OpSub:
		d := x - y
		return (x^y)&(x^d) < 0
	case std.OpMul:
		p := x * y
		return x != 0 && (p/x != y || x == -1 && y == math.MinInt64)
	default:
	}
	return false
}

// readsOf is each maximal path of self d's body reads, in source order (CONFORMANCE.md §2.3).
func (e *Evaluator) readsOf(d *syntax.FnDecl) []syntax.Expr {
	if xs, ok := e.selfReads[d]; ok {
		return xs
	}
	var xs []syntax.Expr
	if d.Body != nil {
		syntax.Inspect(d.Body, func(n syntax.Node) bool {
			x, isExpr := n.(syntax.Expr)
			if isExpr && e.selfPath(x) {
				xs = append(xs, syntax.Unparen(x))
				return false
			}
			return true
		})
	}
	e.selfReads[d] = xs
	return xs
}

// selfPath reports self, a field path of self, or a parameterless method on one (CONFORMANCE.md §2.2).
func (e *Evaluator) selfPath(x syntax.Expr) bool {
	switch n := syntax.Unparen(x).(type) {
	case *syntax.SelfExpr:
		return true
	case *syntax.IdentExpr:
		obj := e.info.Uses[n]
		return obj != nil && obj.Kind() == check.ObjField
	case *syntax.SelectorExpr:
		sel := e.info.Selections[n]
		return sel != nil && sel.Kind == check.SelField && !sel.Deref && n.X != nil && e.selfPath(n.X)
	case *syntax.CallExpr:
		return len(n.Args) == 0 && e.selfMethod(n)
	}
	return false
}

// selfMethod reports a call of a parameterless method on self or on a path of self.
func (e *Evaluator) selfMethod(n *syntax.CallExpr) bool {
	callee := e.info.Calls[n]
	if callee == nil || callee.Kind != check.CalleeMethod || callee.Obj == nil {
		return false
	}
	if ft, ok := callee.Obj.Type().(*types.FuncType); !ok || len(ft.Params) != 0 {
		return false
	}
	switch f := n.Fun.(type) {
	case *syntax.IdentExpr:
		return true
	case *syntax.SelectorExpr:
		sel := e.info.Selections[f]
		return f.X != nil && (sel == nil || !sel.Deref) && e.selfPath(f.X)
	}
	return false
}
