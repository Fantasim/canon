package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// coverage tracks what the arms of a match cover (TYPES.md §12.6).
type coverage struct {
	c        *checker
	env      *env
	t        types.Type
	optional bool
	names    []string
	objs     []*object
	covered  map[int]bool
	info     *MatchInfo
	dead     bool
}

// newCoverage starts the coverage of a scrutinee of type t: enum members, variant cases or
// Kind members (retired ones included), Bool's false and true, and none for an optional.
func (c *checker) newCoverage(env *env, t types.Type) *coverage {
	cov := &coverage{c: c, env: env, covered: map[int]bool{}, info: &MatchInfo{Scrutinee: t}}
	b := t.Base()
	if o, ok := b.(*types.OptionalType); ok {
		cov.optional, b = true, o.Elem.Base()
	}
	cov.t = b
	switch x := b.(type) {
	case *types.EnumType:
		c.completeEnum(c.typeObjects[x], x)
		cov.objs = c.members[x]
	case *types.VariantType:
		c.completeVariant(x)
		cov.objs = c.cases[x]
	case *types.VariantKindType:
		c.completeVariant(x.Variant)
		cov.objs = c.cases[x.Variant]
	}
	for _, o := range cov.objs {
		cov.names = append(cov.names, o.name)
	}
	if b.Kind() == types.Bool {
		cov.names = []string{falseWord, trueWord}
	}
	return cov
}

// matchable reports a scrutinee type a match accepts (E3604 otherwise).
func (cov *coverage) matchable() bool {
	return len(cov.names) > 0
}

// arm records one arm's patterns and returns the case its binders and facts narrow to.
func (cov *coverage) arm(ps []*syntax.Pattern) *object {
	if !cov.matchable() {
		return nil
	}
	var covers []int
	var only *object
	unreachable := false
	for _, p := range ps {
		if p.Binder != nil && len(ps) > 1 {
			cov.c.report(cov.env, diag.E3603.At(cov.env.span(p.Binder), cov.env.span(p), cov.kind(), cov.t))
		}
		idx, o, dead := cov.pattern(p)
		covers = append(covers, idx...)
		unreachable = unreachable || dead
		if len(ps) == 1 {
			only = o
		}
	}
	if unreachable {
		cov.info.Unreachable = append(cov.info.Unreachable, len(cov.info.Covers))
	}
	cov.info.Covers = append(cov.info.Covers, covers)
	return only
}

// pattern resolves one pattern: `_`, `none`, or a member or case (qualified or not); it
// returns the indexes it covers, the case it names, and whether it was reported unreachable.
func (cov *coverage) pattern(p *syntax.Pattern) ([]int, *object, bool) {
	c, env := cov.c, cov.env
	switch p.Keyword {
	case syntax.TokUnderscore:
		return cov.wildcard(p)
	case syntax.KwNone:
		if !cov.optional {
			c.report(env, diag.E3603.At(env.span(p), env.span(p), cov.kind(), cov.t))
			return nil, nil, false
		}
		return cov.mark(p, NoneIndex), nil, cov.dead
	default:
	}
	idx, o := cov.resolve(p)
	if idx < 0 {
		return nil, nil, false
	}
	if p.Binder != nil && !cov.bindable(p, o) {
		o = nil
	}
	return cov.mark(p, idx), o, cov.dead
}

// resolve finds the member or case a pattern names: E3603 when it is not one of the
// scrutinee's type, or is qualified by another type.
func (cov *coverage) resolve(p *syntax.Pattern) (int, *object) {
	c, env := cov.c, cov.env
	parts := p.Name.Parts
	last := parts[len(parts)-1]
	if len(parts) > 1 {
		o := c.typeName(env, &syntax.QualifiedName{Bounds: p.Name.Bounds, Parts: parts[:len(parts)-1]})
		if t := c.typeOfNameOrNil(env, o); t == nil || !types.Identical(t, qualifierType(cov.t)) {
			c.report(env, diag.E3603.At(env.span(p), env.span(p), cov.kind(), cov.t))
			return -1, nil
		}
	}
	for i, n := range cov.names {
		if n == last.Name {
			var o *object
			if i < len(cov.objs) {
				o = cov.objs[i]
				c.info.NameUses[last] = o
			}
			return i, o
		}
	}
	c.report(env, diag.E3603.At(env.span(p), env.span(p), cov.kind(), cov.t))
	return -1, nil
}

// qualifierType is the type a qualified pattern names: the variant for a Kind.
func qualifierType(t types.Type) types.Type {
	if k, ok := t.(*types.VariantKindType); ok {
		return k.Variant
	}
	return t
}

// kind is how E3603 names what a pattern must be.
func (cov *coverage) kind() diag.Kind {
	if cov.t.Kind() == types.Variant || cov.t.Kind() == types.VariantKind {
		return diag.KindCase
	}
	return diag.KindMember
}

// bindable is `c(x)` on a variant; a binding on anything else is E3603.
func (cov *coverage) bindable(p *syntax.Pattern, o *object) bool {
	if cov.t.Kind() == types.Variant && o != nil {
		return true
	}
	cov.c.report(cov.env, diag.E3603.At(cov.env.span(p), cov.env.span(p), cov.kind(), cov.t))
	return false
}

// mark covers idx; a pattern already covered is E3602.
func (cov *coverage) mark(p *syntax.Pattern, idx int) []int {
	cov.dead = false
	if cov.covered[idx] {
		cov.c.report(cov.env, diag.E3602.At(cov.env.span(p), cov.env.span(p)))
		cov.dead = true
		return nil
	}
	cov.covered[idx] = true
	return []int{idx}
}

// wildcard is `_`: what is left; W3601 when nothing is.
func (cov *coverage) wildcard(p *syntax.Pattern) ([]int, *object, bool) {
	var left []int
	if cov.optional && !cov.covered[NoneIndex] {
		left = append(left, NoneIndex)
	}
	for i := range cov.names {
		if !cov.covered[i] {
			left = append(left, i)
		}
	}
	if len(left) == 0 {
		cov.c.warn(cov.env, diag.W3601.At(cov.env.span(p)))
		return nil, nil, true
	}
	for _, i := range left {
		cov.covered[i] = true
	}
	return left, nil, false
}

// finish is E3601 for what no arm covers, and records the match.
func (cov *coverage) finish(node syntax.Node) {
	var missing []string
	if cov.optional && !cov.covered[NoneIndex] {
		missing = append(missing, noneWord)
	}
	for i, n := range cov.names {
		if !cov.covered[i] {
			missing = append(missing, n)
		}
	}
	cov.info.Exhaustive = len(missing) == 0
	if len(missing) > 0 {
		cov.c.report(cov.env, diag.E3601.At(cov.env.tokSpan(node.First()), missing))
	}
}
