package rules

import (
	"sync"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// identKey is an instance under one identity: a check observing identity runs once per key (log-2026-11-01).
type identKey struct {
	inst  *value.Record
	coll  *types.Collection
	owner *value.Record
	key   value.Key
	has   bool
}

// keyOf is rec's instance inst under rec's identity.
func keyOf(inst, rec *value.Record) identKey {
	k := identKey{inst: inst}
	if id := rec.Ident; id != nil {
		k.coll, k.owner, k.key, k.has = id.Coll, id.Owner, id.Key, true
	}
	return k
}

// observing is, by check, whether it reads its instance's identity: computed once per index, for its runners.
type observing struct {
	mu sync.Mutex
	of map[*syntax.CheckDecl]bool
}

func newObserving() *observing {
	return &observing{of: map[*syntax.CheckDecl]bool{}}
}

// observes reports a check reading self bare, there or in a method it calls on self (TYPES.md §6.3).
func (r *Runner) observes(c *syntax.CheckDecl) bool {
	o := r.observe
	o.mu.Lock()
	defer o.mu.Unlock()
	if v, known := o.of[c]; known {
		return v
	}
	v := r.info == nil || r.readsSelf(c, map[*syntax.FnDecl]bool{})
	o.of[c] = v
	return v
}

// readsSelf reports n using self bare, or calling on self a method of seen's that does.
func (r *Runner) readsSelf(n syntax.Node, seen map[*syntax.FnDecl]bool) bool {
	found := false
	syntax.Inspect(n, func(x syntax.Node) bool {
		if found || x == nil {
			return false
		}
		found = r.bareSelf(x, seen)
		return !found && !onSelf(x)
	})
	return found
}

// bareSelf reports x self, a member of self that is no field or method (id, retired: TYPES.md §9.3), or a self-reading method call.
func (r *Runner) bareSelf(x syntax.Node, seen map[*syntax.FnDecl]bool) bool {
	switch e := x.(type) {
	case *syntax.SelfExpr:
		return true
	case *syntax.SelectorExpr:
		if !onSelf(e) {
			return false
		}
		sel := r.info.Selections[e]
		return sel == nil || sel.Kind != check.SelField && sel.Kind != check.SelMethod
	case *syntax.CallExpr:
		fd := r.selfMethod(e)
		if fd == nil || fd.Body == nil || seen[fd] {
			return false
		}
		seen[fd] = true
		return r.readsSelf(fd.Body, seen)
	}
	return false
}

// onSelf reports `self.name`, a selection bareSelf judged: its receiver is no further use of self.
func onSelf(x syntax.Node) bool {
	s, ok := x.(*syntax.SelectorExpr)
	if !ok {
		return false
	}
	_, isSelf := s.X.(*syntax.SelfExpr)
	return isSelf
}

// selfMethod is the method call calls on self, `m(…)` or `self.m(…)`; nil for any other call.
func (r *Runner) selfMethod(call *syntax.CallExpr) *syntax.FnDecl {
	callee := r.info.Calls[call]
	if callee == nil || callee.Kind != check.CalleeMethod || callee.Obj == nil {
		return nil
	}
	if s, isSel := call.Fun.(*syntax.SelectorExpr); isSel && !onSelf(s) {
		return nil
	}
	fd, _ := callee.Obj.Decl().(*syntax.FnDecl)
	return fd
}
