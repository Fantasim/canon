package check

import (
	"regexp"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// refine applies `(lo..hi)` or `(/re/)` to base, recording one dropped for an error (TYPES.md §7.4).
func (c *checker) refine(tc *typeCtx, base types.Type, args *syntax.TypeArgs) types.Type {
	if c.usesParam(tc, args) {
		return base
	}
	t, ok := c.refineArg(tc.env, base, args)
	if !ok {
		c.unrefined[args] = true
	}
	return t
}

// refineArg is the one refinement of args, else E3023; false when it is dropped.
func (c *checker) refineArg(env *env, base types.Type, args *syntax.TypeArgs) (types.Type, bool) {
	if len(args.Args) == 1 {
		switch a := args.Args[0].(type) {
		case *syntax.RegexLit:
			return c.refineRegex(env, base, a)
		case *syntax.RangeExpr:
			return c.refineRange(env, base, a)
		}
	}
	c.report(env, diag.E3023.AtInvalid(env.span(args), env.span(args), base))
	return base, false
}

func (c *checker) refineRegex(env *env, base types.Type, re *syntax.RegexLit) (types.Type, bool) {
	c.info.Types[re] = types.StringType
	if base.Base().Kind() != types.String {
		c.report(env, diag.E3023.AtInvalid(env.span(re), env.span(re), base))
		return base, false
	}
	pattern, err := regexp.Compile(re.Pattern)
	if err != nil {
		c.breakObj(env.owner)
		return base, false
	}
	return &types.Refined{Of: base, Pattern: pattern}, true
}

// refineRange folds the bounds (constant expressions of the base type, §15) and builds the range; `lo > hi` is E3023.
func (c *checker) refineRange(env *env, base types.Type, r *syntax.RangeExpr) (types.Type, bool) {
	bound, ok := boundType(base)
	c.info.Types[r] = types.RangeType
	if !ok {
		c.checkBounds(env, r, types.ErrorType)
		c.report(env, diag.E3023.AtInvalid(env.span(r), env.span(r), base))
		return base, false
	}
	lo, hi, folded := c.checkBounds(env, r, bound)
	if !folded {
		return base, false
	}
	b := &types.Bound{HasLo: lo != nil, HasHi: hi != nil, HiIncluded: r.Op == syntax.TokRangeIncl}
	b.Lo, b.Hi = limitOf(lo, bound), limitOf(hi, bound)
	if b.HasLo && b.HasHi && emptyBound(b, bound) {
		c.report(env, diag.E3023.AtEmpty(env.span(r), env.span(r)))
		return base, false
	}
	c.boundSpans[b] = boundAt{file: env.file, node: r}
	return &types.Refined{Of: base, Range: b}, true
}

// boundType is the type of a range's bounds on base: the scalar for a value range, Int for a
// length; false for a type no range refines.
func boundType(base types.Type) (types.Type, bool) {
	switch base.Base().Kind() {
	case types.Int:
		return types.IntType, true
	case types.Float:
		return types.FloatType, true
	case types.Duration:
		return types.DurationType, true
	case types.String, types.List, types.Map, types.DepMap:
		return types.IntType, true
	default:
		return nil, false
	}
}

// checkBounds types and folds each bound; one in error or naming a broken one is not folded (TYPES.md §1).
func (c *checker) checkBounds(env *env, r *syntax.RangeExpr, t types.Type) (lo, hi value.Value, ok bool) {
	ce := env.constant(diag.KindRefinementBound)
	ok = true
	fold := func(e syntax.Expr) value.Value {
		if e == nil {
			return nil
		}
		before := c.reported // counting here is sound: a const typed lazily during the bound is a dependency of the bound's owner
		got := c.expr(ce, e, t)
		if t.Kind() == types.Error {
			return nil
		}
		if c.reported != before || got.Kind() == types.Error || c.readsBroken(e) {
			ok = false
			return nil
		}
		v, folded := c.foldConst(ce, e)
		ok = ok && folded
		return v
	}
	lo, hi = fold(r.Lo), fold(r.Hi)
	return lo, hi, ok
}

// limitOf is a folded bound as a Limit of the bound type: F for Float, I otherwise (ms).
func limitOf(v value.Value, t types.Type) types.Limit {
	switch v := v.(type) {
	case *value.Int:
		if t.Kind() == types.Float {
			return types.Limit{F: float64(v.V)}
		}
		return types.Limit{I: v.V}
	case *value.Float:
		return types.Limit{F: v.V}
	case *value.Dur:
		return types.Limit{I: v.Ms}
	}
	return types.Limit{}
}

// emptyBound reports a range with no value: lo > hi, or lo >= hi when hi is excluded.
func emptyBound(b *types.Bound, t types.Type) bool {
	if t.Kind() == types.Float {
		return b.Lo.F > b.Hi.F || (!b.HiIncluded && b.Lo.F >= b.Hi.F)
	}
	return b.Lo.I > b.Hi.I || (!b.HiIncluded && b.Lo.I >= b.Hi.I)
}

// resolveWhere is `T where p`; p is checked later, `it` of type T, never optional (TYPES.md §7.4).
func (c *checker) resolveWhere(tc *typeCtx, t *syntax.WhereType) types.Type {
	base := c.resolveType(tc, t.Base)
	if c.usesParam(tc, t.Pred) {
		return base
	}
	it := base
	if o, ok := base.Base().(*types.OptionalType); ok {
		it = o.Elem
	}
	text := tc.env.written(t.Pred)
	c.wheres = append(c.wheres, whereJob{env: tc.env, pred: t.Pred, it: it})
	return &types.Refined{Of: base, Where: &types.Predicate{Expr: t.Pred, Text: text}}
}

// usesParam is E3803 for a type function's result refinement naming a parameter (TYPES.md §11.2).
func (c *checker) usesParam(tc *typeCtx, n syntax.Node) bool {
	if !tc.fnBody {
		return false
	}
	found := false
	syntax.Inspect(n, func(x syntax.Node) bool {
		if id, ok := x.(*syntax.IdentExpr); ok && tc.scope[id.Name].param != nil {
			found = true
		}
		return !found
	})
	if found {
		c.report(tc.env, diag.E3803.AtArgument(tc.env.span(n)))
	}
	return found
}

// whereJob is a `where` predicate to check with the bodies.
type whereJob struct {
	env  *env
	pred syntax.Expr
	it   types.Type
}

// checkWhere checks a predicate with `it` bound (TYPES.md §3.4: E2109 elsewhere).
func (c *checker) checkWhere(j whereJob) {
	env := j.env.push()
	env.it = c.newObject(ObjLocal, itName, env.pkg, j.pred, env.file)
	env.it.typ = j.it
	c.cond(env, j.pred)
}

// constText is the text of a string literal without interpolation.
func constText(s syntax.StrLit) string {
	switch s := s.(type) {
	case *syntax.StringLit:
		var b strings.Builder
		for _, p := range s.Parts {
			b.WriteString(p.Text)
		}
		return b.String()
	case *syntax.RawStringLit:
		return s.Value
	}
	return ""
}

// LiteralText is a string literal's text and whether it holds no interpolation: false for any
// other, the one test of a literal asset root, which build places too (TYPES.md 13.4).
func LiteralText(s syntax.StrLit) (string, bool) {
	if s == nil {
		return "", false
	}
	return constText(s), interpolationFree(s)
}

// resolveAsset is `asset(root, ext: […])`; a part holding a lexer error is not judged (TYPES.md §13.4, §1).
func (c *checker) resolveAsset(tc *typeCtx, t *syntax.AssetType) types.Type {
	env := tc.env
	root, literal := LiteralText(t.Dir)
	if !literal && t.Dir != nil {
		root = strings.Trim(env.written(t.Dir), quoteMark) // shown as written, interpolation included
	}
	c.info.Types[t.Dir] = types.StringType
	spec := &types.AssetSpec{Root: root}
	ok := !c.lexError(t.Dir) && c.assetRoot(env, t.Dir, root, literal)
	if len(t.Exts) == 0 {
		c.report(env, diag.E3704.AtMissing(env.span(t)))
		ok = false
	}
	for _, e := range t.Exts {
		ext := nameLitText(e)
		if s, isStr := e.(*syntax.StringLit); isStr {
			c.info.Types[s] = types.StringType
		}
		if c.lexError(e) {
			ok = false
			continue
		}
		if ext == "" || strings.ContainsAny(ext, extForbidden) {
			c.report(env, diag.E3704.AtExt(env.span(e), ext))
			ok = false
		}
		spec.Exts = append(spec.Exts, ext)
	}
	if !ok {
		return types.StringType
	}
	return &types.Refined{Of: types.StringType, Asset: spec}
}

// assetRoot is E3704 (an interpolated root, or no load path), E7003 (unknown root) or E7001 (leaves it): WIRE.md §2.1, §2.2.
func (c *checker) assetRoot(env *env, at syntax.Node, root string, literal bool) bool {
	if !literal || !loadSyntax(root) {
		c.report(env, diag.E3704.AtRoot(env.span(at), root))
		return false
	}
	name, rooted := strings.CutPrefix(root, rootSigil)
	if c.proj == nil {
		return true
	}
	name, _, _ = strings.Cut(name, slash)
	if _, declared := c.proj.Root(name); declared || !rooted {
		return c.contained(env, at, root)
	}
	names := make([]string, 0, len(c.proj.Roots))
	for _, r := range c.proj.Roots {
		names = append(names, r.Name)
	}
	c.report(env, diag.E7003.At(env.span(at), name, names))
	return false
}

// loadSyntax reports a path of WIRE.md §2.1's grammar: relative, no backslash or empty segment.
func loadSyntax(p string) bool {
	if p == "" || strings.ContainsRune(p, backslash) || strings.HasPrefix(p, slash) || strings.Index(p, colon) == 1 {
		return false
	}
	body := strings.TrimSuffix(p, slash)
	if after, rooted := strings.CutPrefix(body, rootSigil); rooted {
		name, rest, hasRest := strings.Cut(after, slash)
		if name == "" || !hasRest {
			return name != ""
		}
		body = rest
	}
	return !slices.Contains(strings.Split(body, slash), "")
}

// nameLitText is a name written as an identifier or a string.
func nameLitText(n syntax.NameLit) string {
	switch n := n.(type) {
	case *syntax.Ident:
		return n.Name
	case syntax.StrLit:
		return constText(n)
	}
	return ""
}
