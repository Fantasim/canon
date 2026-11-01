package check

import (
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// foldedDefault checks a field default that is a constant expression, and not itself a literal, as the value it folds to: at the default, whether or not a value takes it (TYPES.md §7.4; DECISIONS 412).
func (c *checker) foldedDefault(env *env, e syntax.Expr, got, want types.Type, before int) {
	if literalOf(e) != nil || c.reported != before || got.Kind() == types.Error || !foldable(want) {
		return
	}
	if !c.constantDefault(e) || c.readsBroken(e) || c.spentNow(env.pkg) {
		return
	}
	// A silent failure (a read of a const whose own fold failed with a finding) breaks no record: that const is broken, not this default (DECISIONS 150; log-2026-10-31, CHECK-OWED opus choice 1).
	if v, ok, _ := c.foldReported(env, e); ok {
		c.checkFolded(env, e, v, want)
	}
}

// constantDefault reports e built only of literals, collections, operators, built-ins and constants of any package; a field, a parameter, `self` or a user fn is no constant (TYPES.md §15).
func (c *checker) constantDefault(e syntax.Expr) bool {
	ok := true
	syntax.Inspect(e, func(n syntax.Node) bool {
		switch x := n.(type) {
		case nil, *syntax.IntLit, *syntax.FloatLit, *syntax.DurationLit, *syntax.BoolLit, *syntax.RawStringLit,
			*syntax.StringLit, *syntax.ParenExpr, *syntax.UnaryExpr, *syntax.BinaryExpr, *syntax.ListLit,
			*syntax.BraceLit, *syntax.MapItem, *syntax.IndexExpr, *syntax.CallExpr, *syntax.Arg, *syntax.Ident:
		case *syntax.IdentExpr:
			ok = ok && constantObject(c.info.Uses[x])
		case *syntax.SelectorExpr:
			o := c.info.NameUses[x.Name]
			ok = ok && constantObject(o)
			return ok && o.Kind() == ObjBuiltin // a built-in method keeps its receiver, a package constant is whole
		default:
			ok = false
		}
		return ok
	})
	return ok
}

// foldable reports a type checkFolded judges, holding no record whose body may be unchecked yet (EVALUATION.md §1).
func foldable(t types.Type) bool {
	switch x := t.(type) {
	case *types.Alias:
		return foldable(x.Def)
	case *types.OptionalType:
		return foldable(x.Elem)
	case *types.Refined:
		return foldable(x.Of)
	case types.Basic:
		return true
	case *types.ListType:
		return foldable(x.Elem)
	case *types.MapType:
		return foldable(x.Key) && foldable(x.Value)
	default:
		return false
	}
}

func constantObject(o Object) bool {
	return o != nil && (o.Kind() == ObjConst || o.Kind() == ObjBuiltin)
}

// checkFolded is the static check of a folded value v against want, a literal's check on a value: the first refinement or limit v breaks is reported at at, and reports true (TYPES.md §7.2, §7.4).
func (c *checker) checkFolded(env *env, at syntax.Node, v value.Value, want types.Type) bool {
	switch x := want.(type) {
	case *types.Alias:
		return c.checkFolded(env, at, v, x.Def)
	case *types.OptionalType:
		return c.checkFolded(env, at, v, x.Elem)
	case *types.Refined:
		return c.foldedRefinement(env, at, v, x) || c.checkFolded(env, at, v, x.Of)
	case types.Basic:
		return c.foldedLimits(env, at, v, x)
	case *types.ListType:
		l, ok := v.(*value.List)
		return ok && slices.ContainsFunc(l.Elems, func(e value.Value) bool { return c.checkFolded(env, at, e, x.Elem) })
	case *types.MapType:
		m, ok := v.(*value.Map)
		return ok && (slices.ContainsFunc(m.Keys, func(k value.Value) bool { return c.checkFolded(env, at, k, x.Key) }) ||
			slices.ContainsFunc(m.Vals, func(e value.Value) bool { return c.checkFolded(env, at, e, x.Value) }))
	default:
		return false
	}
}

// foldedRefinement is E3205 or E3204 for v against one written refinement, as literalRefinement.
func (c *checker) foldedRefinement(env *env, at syntax.Node, v value.Value, r *types.Refined) bool {
	if s, isStr := v.(*value.Str); isStr && r.Pattern != nil && !r.Pattern.MatchString(s.V) {
		c.patternFinding(env, at, r, types.QuoteString(s.V))
		return true
	}
	if r.Range == nil {
		return false
	}
	k := r.Of.Base().Kind()
	m, text, ok := measureValue(v, k)
	m.F = stored(r.Of, m.F)
	if !ok || inBound(m, r.Range, k) {
		return false
	}
	c.rangeFinding(env, at, r, text)
	return true
}

// foldedLimits is E3201 for an Int or Duration v outside the sized type b.
func (c *checker) foldedLimits(env *env, at syntax.Node, v value.Value, b types.Basic) bool {
	lo, hi, ok := b.Limits()
	if !ok {
		return false
	}
	switch x := v.(type) {
	case *value.Int:
		if b.K == types.Int && (x.V < lo || x.V > hi) {
			c.limitFinding(env, at, b, strconv.FormatInt(x.V, decimalBase))
			return true
		}
	case *value.Dur:
		if b.K == types.Duration && (x.Ms < lo || x.Ms > hi) {
			c.limitFinding(env, at, b, types.DurationText(x.Ms))
			return true
		}
	}
	return false
}

// measureValue is what a range refinement of kind k compares for a folded value, with its canonical text, as measure does for a literal.
func measureValue(v value.Value, k types.Kind) (types.Limit, string, bool) {
	switch x := v.(type) {
	case *value.Int:
		return types.Limit{I: x.V, F: float64(x.V)}, strconv.FormatInt(x.V, decimalBase), k == types.Int || k == types.Float
	case *value.Float:
		return types.Limit{F: x.V}, types.FloatText(x.V, bits64), k == types.Float
	case *value.Dur:
		return types.Limit{I: x.Ms}, types.DurationText(x.Ms), k == types.Duration
	case *value.Str:
		return types.Limit{I: int64(len(x.V))}, types.QuoteString(x.V), k == types.String
	case *value.List:
		return types.Limit{I: int64(len(x.Elems))}, strconv.Itoa(len(x.Elems)), k == types.List
	case *value.Map:
		return types.Limit{I: int64(len(x.Keys))}, strconv.Itoa(len(x.Keys)), k == types.Map
	default:
		return types.Limit{}, "", false
	}
}
