package check

import (
	"maps"
	"strconv"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// facts are what holds about stable paths (TYPES.md §6.6).
type facts map[string]fact

// fact is what is known of one path; kase is nil unless `p is c` holds. An unsure fact (DECISIONS
// 516) came from side of condition cond, an operand in error in it; pat when it covers a pattern.
type fact struct {
	root   int
	kase   *types.CaseType
	unsure bool
	cond   int
	side   bool
	pat    *pattern
}

// union is a ∪ b; a case fact of b wins, and a path either side is sure of is sure.
func (a facts) union(b facts) facts {
	if len(b) == 0 {
		return a
	}
	out := make(facts, len(a)+len(b))
	maps.Copy(out, a)
	for k, w := range b { //canon:unordered builds a map
		if v, ok := a[k]; ok && w.unsure && !v.unsure {
			continue
		}
		out[k] = w
	}
	return out
}

// intersect is a ∩ b: the paths both hold, a case only when both know the same one, sure only when
// both are; never a path the two sides of one condition gave (DECISIONS 516).
func (a facts) intersect(b facts) facts {
	out := facts{}
	a.meet(b, out)
	b.meet(a, out)
	return out
}

// meet adds to out each fact of a that b also holds: a pattern b holds as well, a path b holds or
// covers by a pattern, unsure unless both are sure of it.
func (a facts) meet(b facts, out facts) {
	for k, v := range a { //canon:unordered builds a map
		if v.pat != nil {
			if _, ok := b[k]; ok {
				out[k] = v
			}
			continue
		}
		w, ok := b[k]
		if !ok {
			w, ok = b.cover(k, v.root, v)
		}
		if !ok || v.opposes(w) {
			continue
		}
		out[k] = v.meet(w)
	}
}

// meet is the fact of a path both v and w hold: a case both know, unsure if either is.
func (v fact) meet(w fact) fact {
	if w.kase != v.kase {
		v.kase = nil
	}
	if !v.unsure && w.unsure {
		v.unsure, v.cond, v.side = true, w.cond, w.side
	}
	if v.unsure {
		v.kase = nil
	}
	return v
}

// kill removes every fact rooted at the object id (§6.6 kills), and the root from each pattern.
func (a facts) kill(id int) facts {
	out := facts{}
	for k, v := range a { //canon:unordered builds a map
		if v.root == id {
			continue
		}
		if v.pat != nil && v.pat.root == anyRoot {
			v.pat = v.pat.killed(id)
		}
		out[k] = v
	}
	return out
}

// narrowed applies the facts to a path's type: an unsure fact narrows T? only (DECISIONS 516).
func (c *checker) narrowed(env *env, e syntax.Expr, t types.Type) types.Type {
	delete(c.unsure, e)
	if len(env.facts) == 0 || t == nil {
		return t
	}
	key, root, ok := c.pathKey(env, e)
	if !ok {
		return t
	}
	f, known := env.facts.lookup(key, root)
	switch {
	case !known:
		return t
	case f.kase != nil && !f.unsure:
		return f.kase
	}
	if o, isOpt := t.Base().(*types.OptionalType); isOpt {
		if f.unsure {
			c.unsure[e] = t
		}
		return o.Elem
	}
	return t
}

// pathKey is the key of a stable path (TYPES.md §6.6).
func (c *checker) pathKey(env *env, e syntax.Expr) (string, int, bool) {
	switch x := e.(type) {
	case *syntax.IdentExpr:
		o, ok := c.info.Uses[x].(*object)
		if !ok || !stableRoot(o) {
			return "", 0, false
		}
		if o.kind == ObjField {
			return selfKey + dot + o.name, selfID, true
		}
		return strconv.Itoa(o.id), o.id, true
	case *syntax.SelfExpr:
		return selfKey, selfID, env.rec != nil
	case *syntax.SelectorExpr:
		if x.X == nil {
			return "", 0, false
		}
		sel := c.info.Selections[x]
		if sel == nil || (sel.Kind != SelField && sel.Kind != SelEntry) {
			return "", 0, false
		}
		key, root, ok := c.pathKey(env, x.X)
		return key + dot + x.Name.Name, root, ok
	}
	return "", 0, false
}

// stableRoot reports the objects a stable path may start from (TYPES.md §6.6).
func stableRoot(o *object) bool {
	switch o.kind {
	case ObjLocal, ObjParam, ObjField, ObjLet, ObjConst:
		return true
	default:
		return false
	}
}

// factsOf are T(C) and F(C) of a checked condition (TYPES.md §6.6's table).
func (c *checker) factsOf(env *env, e syntax.Expr) (tf, ff facts) {
	switch x := e.(type) {
	case *syntax.ParenExpr:
		return c.factsOf(env, x.X)
	case *syntax.UnaryExpr:
		if x.Op == syntax.KwNot {
			t, f := c.factsOf(env, x.X)
			return f, t
		}
	case *syntax.BinaryExpr:
		return c.binaryFacts(env, x)
	case *syntax.IsExpr:
		return c.isFacts(env, x), nil
	}
	return nil, nil
}

func (c *checker) binaryFacts(env *env, x *syntax.BinaryExpr) (tf, ff facts) {
	switch x.Op {
	case syntax.KwAnd:
		t1, f1 := c.factsOf(env, x.X)
		t2, f2 := c.factsOf(env, x.Y)
		return t1.union(t2), f1.intersect(f2)
	case syntax.KwOr:
		t1, f1 := c.factsOf(env, x.X)
		t2, f2 := c.factsOf(env, x.Y)
		return t1.intersect(t2), f1.union(f2)
	case syntax.TokEq, syntax.TokNe:
		t1, f1 := c.equalFacts(env, x.X, x.Y)
		t2, f2 := c.equalFacts(env, x.Y, x.X)
		tf, ff := t1.union(t2), f1.union(f2)
		if x.Op == syntax.TokNe {
			tf, ff = ff, tf
		}
		c.conds++
		return tf.tagged(c.conds, true), ff.tagged(c.conds, false)
	default:
	}
	return nil, nil
}

// equalFacts are (T, F) of `p == other` for the path p (TYPES.md §6.6's rows, either side).
func (c *checker) equalFacts(env *env, p, other syntax.Expr) (tf, ff facts) {
	if c.hidden(other) {
		f := c.presentFacts(env, p).unsured()
		return f, f
	}
	if !c.presentWhenEqual(other) {
		return nil, nil
	}
	return swapIfNone(other, c.presentFacts(env, p))
}

// presentWhenEqual reports an operand whose equality with p proves or disproves presence:
// `none`, or a value of a non-optional type.
func (c *checker) presentWhenEqual(e syntax.Expr) bool {
	if _, isNone := inner(e).(*syntax.NoneLit); isNone {
		return true
	}
	t := c.info.Types[e]
	return t != nil && t.Base().Kind() != types.Optional && t.Base().Kind() != types.None
}

// swapIfNone is (T, F) of `p == other`: `p == none` holds when p is none, `p == e` when p is
// present.
func swapIfNone(other syntax.Expr, known facts) (tf, ff facts) {
	if _, isNone := inner(other).(*syntax.NoneLit); isNone {
		return nil, known
	}
	return known, nil
}

// presentFacts are the facts that p is present: the path, and each prefix of it when it is
// written as an optional chain.
func (c *checker) presentFacts(env *env, p syntax.Expr) facts {
	key, root, ok := c.pathKey(env, p)
	switch {
	case !ok && c.hidden(p):
		return c.patternFacts(env, p)
	case !ok:
		return nil
	}
	out := facts{key: {root: root}}
	for sel, isSel := p.(*syntax.SelectorExpr); isSel && chainHasOpt(sel); sel, isSel = sel.X.(*syntax.SelectorExpr) {
		if k, r, found := c.pathKey(env, sel.X); found {
			out[k] = fact{root: r}
		}
	}
	return out
}

// chainHasOpt reports a `?.` in a selector chain.
func chainHasOpt(sel *syntax.SelectorExpr) bool {
	for e := syntax.Expr(sel); ; {
		s, ok := e.(*syntax.SelectorExpr)
		if !ok {
			return false
		}
		if s.Optional {
			return true
		}
		e = s.X
	}
}

// isFacts is T(p is c): p is present and of case c; unsure, with no case, when p is in error or
// c names no case (DECISIONS 516).
func (c *checker) isFacts(env *env, x *syntax.IsExpr) facts {
	key, root, ok := c.pathKey(env, x.X)
	o, isCase := c.info.NameUses[x.Target.Parts[len(x.Target.Parts)-1]].(*object)
	switch {
	case !ok && c.hidden(x.X):
		c.conds++
		return c.patternFacts(env, x.X).tagged(c.conds, true)
	case !ok:
		return nil
	case !isCase || o.kind != ObjCase:
		c.conds++
		return facts{key: {root: root, unsure: true, cond: c.conds, side: true}}
	}
	return facts{key: {root: root, kase: o.typ.(*types.CaseType)}}
}

// withFacts is a copy of env where f also holds.
func (env *env) withFacts(f facts) *env {
	if len(f) == 0 {
		return env
	}
	e := env.with()
	e.facts = env.facts.union(f)
	return e
}
