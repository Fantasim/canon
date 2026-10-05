package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// staticView is t with each type application expanded and each dependent map `{ref c: V}` (TYPES.md §11.3–§11.5, §13.1).
func staticView(t types.Type) types.Type {
	e := eraser{seen: map[*types.TypeFunc]bool{}}
	if v, changed := e.erase(t); changed {
		return v
	}
	return t
}

// eraser is one staticView walk; seen stops an expansion at a type function already being expanded (E3021 is its declaration's).
type eraser struct {
	seen map[*types.TypeFunc]bool
}

// erase is staticView on the components of t; unchanged, t itself is returned, aliases kept.
func (e eraser) erase(t types.Type) (types.Type, bool) {
	switch x := t.Base().(type) {
	case *types.TypeAppType:
		return e.expand(x.Fn), true
	case *types.DepUnionType:
		if x.Fn.Body != nil {
			return e.expand(x.Fn), true
		}
	case *types.DepMapType:
		v, _ := e.erase(x.Value)
		return &types.MapType{Key: &types.RefType{Target: x.Coll}, Value: v}, true
	case *types.OptionalType:
		if el, ch := e.erase(x.Elem); ch {
			return &types.OptionalType{Elem: el}, true
		}
	case *types.ListType:
		if el, ch := e.erase(x.Elem); ch {
			return &types.ListType{Elem: el, KeyedBy: x.KeyedBy}, true
		}
	case *types.MapType:
		k, kc := e.erase(x.Key)
		v, vc := e.erase(x.Value)
		if kc || vc {
			return &types.MapType{Key: k, Value: v}, true
		}
	case *types.LitUnionType:
		if of, ch := e.erase(x.Of); ch {
			return flatUnion(of, x.Literals), true
		}
	}
	return t, false
}

// flatUnion is `of | lits`, one union when of is one: an expanded `SK(e) | "y"` (TYPES.md §13.1, §13.2).
func flatUnion(of types.Type, lits []string) *types.LitUnionType {
	if u, ok := of.Base().(*types.LitUnionType); ok {
		return &types.LitUnionType{Of: u.Of, Literals: append(slices.Clip(u.Literals), lits...)}
	}
	return &types.LitUnionType{Of: of, Literals: lits}
}

// expand is an application of fn: `F(*)` for a match body, else its body's static view (TYPES.md §11.4, §13.1).
func (e eraser) expand(fn *types.TypeFunc) types.Type {
	if fn.Body == nil || e.seen[fn] {
		return &types.DepUnionType{Fn: fn}
	}
	e.seen[fn] = true
	v, _ := e.erase(fn.Body)
	delete(e.seen, fn)
	return v
}

// depFunc is the type function of a dependent type (an application or its union), after
// aliases, refinements and one optional; nil for any other type.
func depFunc(t types.Type) *types.TypeFunc {
	switch x := unwrap(t).(type) {
	case *types.DepUnionType:
		return x.Fn
	case *types.TypeAppType:
		return x.Fn
	}
	return nil
}

// branches are the result types of a type function, the branches of a result that is itself
// a type application flattened in, in declaration order (the body, then the arms).
func branches(fn *types.TypeFunc) []types.Type {
	w := &branchWalk{seen: map[*types.TypeFunc]bool{}}
	w.walk(fn)
	return w.out
}

// branchWalk collects the branches of type functions, each function once (an alias cycle
// through type functions is E3021, reported where it is declared).
type branchWalk struct {
	seen map[*types.TypeFunc]bool
	out  []types.Type
}

func (w *branchWalk) walk(fn *types.TypeFunc) {
	if w.seen[fn] {
		return
	}
	w.seen[fn] = true
	results := []types.Type{fn.Body}
	for _, a := range fn.Arms {
		results = append(results, a.Result)
	}
	for _, r := range results {
		switch inner := depFunc(r); {
		case r == nil:
		case inner != nil && r.Base().Kind() != types.Optional:
			w.walk(inner)
		default:
			w.out = append(w.out, r)
		}
	}
}

// firstBranch is the first branch of fn that fits, nil when none does.
func firstBranch(fn *types.TypeFunc, fits func(types.Type) bool) types.Type {
	for _, b := range branches(fn) {
		if fits(b) {
			return b
		}
	}
	return nil
}

// dependentLiteral is E3002 (E3403 for none) for a literal no branch of a dependent type fits (TYPES.md §11.4 "Literals").
func (c *checker) dependentLiteral(env *env, e syntax.Expr, s, want types.Type) bool {
	if depFunc(unwrapUnion(want)) == nil || !isValueLiteral(e) || c.literalFits(e, s, want) {
		return false
	}
	if s.Kind() == types.None {
		view := staticView(want)
		c.report(env, diag.E3403.At(env.span(e), view, foundOptional(s, view)))
	} else {
		c.report(env, diag.E3002.At(env.span(e), staticView(want), s))
	}
	return true
}

// isValueLiteral is a scalar literal token or `none`, parenthesized or not. A list or brace
// literal is classified against the branches instead (dependentList, braceBranch).
func isValueLiteral(e syntax.Expr) bool {
	switch inner(e).(type) {
	case *syntax.IntLit, *syntax.FloatLit, *syntax.DurationLit, *syntax.StringLit,
		*syntax.RawStringLit, *syntax.BoolLit, *syntax.NoneLit:
		return true
	}
	return false
}

// literalFits reports a literal e of type s fitting the branch b: `none` an optional branch,
// an integer literal a Float or an integer-keyed ref, a string literal one of a union's
// literals or a String-keyed ref, a nested dependent type one of its branches.
func (c *checker) literalFits(e syntax.Expr, s, b types.Type) bool {
	f := &litFit{c: c, e: e, s: s, seen: map[*types.TypeFunc]bool{}}
	return f.fits(b)
}

// litFit is one literalFits walk; seen stops at a type function met twice (E3021 is its declaration's).
type litFit struct {
	c    *checker
	e    syntax.Expr
	s    types.Type
	seen map[*types.TypeFunc]bool
}

func (f *litFit) fits(b types.Type) bool {
	if f.s.Kind() == types.None && b.Base().Kind() == types.Optional {
		return true
	}
	if fn := depFunc(b); fn != nil {
		if f.seen[fn] {
			return false
		}
		f.seen[fn] = true
		return firstBranch(fn, f.fits) != nil
	}
	if f.s.Kind() == types.None {
		return false
	}
	if u, ok := unwrap(b).(*types.LitUnionType); ok {
		return f.c.unionLiteral(f.e, u) || f.fits(u.Of)
	}
	if coll := f.c.keyTarget(b); coll != nil && keyLiteral(f.e, coll) {
		return true
	}
	if isIntLiteral(f.e) && floatTarget(b) {
		return true
	}
	return f.c.assignable(f.s, b)
}

// unionLiteral reports a string literal of u or of a union its alternative reaches (TYPES.md §13.2).
func (c *checker) unionLiteral(e syntax.Expr, u *types.LitUnionType) bool {
	s, ok := inner(e).(syntax.StrLit)
	lits, _, _ := unionChain(u)
	return ok && interpolationFree(s) && slices.Contains(lits, constText(s))
}

// unionChain follows a literal union through the union bodies of the type functions its
// alternatives apply: every literal met, those type functions and the last alternative.
func unionChain(u *types.LitUnionType) ([]string, []*types.TypeFunc, types.Type) {
	var lits []string
	var fns []*types.TypeFunc
	for {
		lits = append(lits, u.Literals...)
		fn := depFunc(u.Of)
		if fn == nil || slices.Contains(fns, fn) {
			return lits, fns, u.Of
		}
		fns = append(fns, fn)
		next := types.UnionBody(fn)
		if next == nil {
			return lits, fns, u.Of
		}
		u = next
	}
}

// keyLiteral reports a literal that is a key of coll: an integer literal of an integer-keyed
// list, a string literal of any other collection.
func keyLiteral(e syntax.Expr, coll *types.Collection) bool {
	if isIntLiteral(e) {
		return keyedByInt(coll)
	}
	s, ok := inner(e).(syntax.StrLit)
	return ok && interpolationFree(s) && !keyedByInt(coll)
}

// dependentList checks a list literal against a dependent type's first list branch, E3002 without one (DECISIONS 171).
func (c *checker) dependentList(env *env, e *syntax.ListLit, want types.Type) types.Type {
	b := firstBranch(depFunc(unwrapUnion(want)), func(b types.Type) bool {
		_, ok := unwrap(b).(*types.ListType)
		return ok
	})
	if b != nil {
		return c.listLit(env, e, b)
	}
	for _, x := range e.Elems {
		c.expr(env, x, types.ErrorType)
	}
	c.report(env, diag.E3002.At(env.span(e), staticView(want), env.literalText(e)))
	return types.ErrorType
}

// notDependent is E3804 for op on a dependent value, optional or not (TYPES.md §11.4); false otherwise.
func (c *checker) notDependent(env *env, n syntax.Node, t types.Type, op string) bool {
	d := optElem(t)
	if d.Base().Kind() != types.DepUnion {
		return false
	}
	c.report(env, diag.E3804.At(env.span(n), op, depName(d)))
	return true
}

// dependentIdent is a bare name against a dependent type or a union over one (TYPES.md §4.1).
func (c *checker) dependentIdent(env *env, e *syntax.IdentExpr, want types.Type, fn *types.TypeFunc) types.Type {
	if o := c.dependentName(env, e, e.Name, fn); o != nil {
		return c.use(env, e, o)
	}
	if o := c.scopedKey(env, e.Name, fn); o != nil {
		return c.notAKey(env, e, o, want)
	}
	c.info.Symbols[e] = true
	if u, isUnion := unwrap(want).(*types.LitUnionType); isUnion {
		return u
	}
	return &types.DepUnionType{Fn: fn}
}

// dependentName is the local, parameter or field a bare name against a dependent type names
// (E2101 when a branch offers it too), else nil when a branch offers it (package names and
// built-ins lose), else a let or const of the package or its imports; nil stays symbolic.
func (c *checker) dependentName(env *env, n syntax.Node, name string, fn *types.TypeFunc) *object {
	if local := localValue(env, name); local != nil {
		c.offered(fn, name, func(m *object, t types.Type) bool { return c.ambiguity(env, n, name, m, t) })
		return local
	}
	if c.offered(fn, name, func(*object, types.Type) bool { return true }) {
		return nil
	}
	if o := c.lookupGlobal(env, name); o != nil && (o.kind == ObjLet || o.kind == ObjConst) {
		return o
	}
	return nil
}

// scopedKey is what a name in scope names where a branch of fn takes dynamic keys (TYPES.md §4.1, §11.4).
func (c *checker) scopedKey(env *env, name string, fn *types.TypeFunc) *object {
	o := c.lookup(env, name)
	if o == nil || c.offered(fn, name, func(*object, types.Type) bool { return true }) {
		return nil
	}
	if !slices.ContainsFunc(branches(fn), func(b types.Type) bool { return c.dynamicKeys(b) != nil }) {
		return nil
	}
	return o
}

// offered reports a branch of fn offering name statically (a member, case or static key) for
// which stop holds, trying each in turn.
func (c *checker) offered(fn *types.TypeFunc, name string, stop func(*object, types.Type) bool) bool {
	for _, b := range branches(fn) {
		if m, t := c.inExpected(unwrapUnion(b), name); m != nil && stop(m, t) {
			return true
		}
	}
	return false
}

// localValue is a local, parameter or in-scope field named name (TYPES.md §3.3 steps 2–3).
func localValue(env *env, name string) *object {
	if o := env.lookupLocal(name); o != nil {
		return o
	}
	if o := env.lookupRecord(name); o != nil && o.kind != ObjMethod {
		return o
	}
	return nil
}
