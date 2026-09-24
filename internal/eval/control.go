package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// evalIf evaluates only the branch its conditions choose; an `else if` is a node of its own.
func evalIf(r *run, e syntax.Expr, at *vpath) value.Value {
	for x := e.(*syntax.IfExpr); x != nil; x = x.ElseIf {
		if x != e && !r.step(x) {
			return nil
		}
		c, ok := r.truth(x.Cond)
		switch {
		case !ok:
			return nil
		case c:
			return r.evalAt(x.Then.X, at)
		case x.Else != nil:
			return r.evalAt(x.Else.X, at)
		}
	}
	r.bug(e)
	return nil
}

// evalMatch evaluates the arm covering the scrutinee (TYPES.md §12.6), its binder bound.
func evalMatch(r *run, e syntax.Expr, at *vpath) value.Value {
	x := e.(*syntax.MatchExpr)
	v := r.eval(x.Scrutinee)
	if v == nil {
		return nil
	}
	pats := make([][]*syntax.Pattern, len(x.Arms))
	for i, a := range x.Arms {
		pats[i] = a.Patterns
	}
	i := r.arm(r.ev.info.Matches[x], pats, v)
	if i < 0 {
		return nil
	}
	return r.evalAt(x.Arms[i].Body, at)
}

// arm is the index of the arm whose patterns cover v, its binder bound to v; -1 aborts.
func (r *run) arm(info *check.MatchInfo, pats [][]*syntax.Pattern, v value.Value) int {
	idx, ok := matchIndex(v)
	if info == nil || !ok {
		r.bug(nil)
		return -1
	}
	for i, covers := range info.Covers {
		if !slices.Contains(covers, idx) || i >= len(pats) {
			continue
		}
		if ps := pats[i]; len(ps) == 1 && ps[0].Binder != nil {
			if obj := r.ev.info.Defs[ps[0].Binder]; obj != nil {
				r.fr.vars[obj] = v
			}
		}
		return i
	}
	r.bug(nil)
	return -1
}

// matchIndex is what a match compares: a member's or a case's index, 0 and 1 for Bool, and
// check.NoneIndex for none.
func matchIndex(v value.Value) (int, bool) {
	switch x := v.(type) {
	case *value.None:
		return check.NoneIndex, true
	case *value.Member:
		return x.Index, true
	case *value.CaseKind:
		return x.Index, true
	case *value.Bool:
		if x.V {
			return boolIndex, true
		}
		return 0, true
	case *value.Record:
		if ct, ok := x.T.Base().(*types.CaseType); ok {
			return ct.Index, true
		}
	}
	return 0, false
}
