package check

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// unsureState is the checker's record of unsure facts (DECISIONS 516).
type unsureState struct {
	unsure map[syntax.Node]types.Type // reads an unsure fact narrowed, with their T?
	conds  int                        // the tags given to unsure facts so far
}

// pattern is the paths an operand in error could read as (DECISIONS 516): one part unknown.
type pattern struct {
	root   int    // the fixed root's id; anyRoot when the root is in error
	prefix string // the key of the fixed part; "" when the root is in error
	tail   string // the segments after the part in error, joined by dots
	below  int    // a root in error reads as a root declared before: an id below it
	kills  []int  // roots assigned since (a root in error only)
}

// hidden reports an operand in error: what it would have proved is unknown (TYPES.md §1).
func (c *checker) hidden(e syntax.Expr) bool {
	t := c.info.Types[e]
	return t != nil && c.inError(t)
}

// unsureRead reports a read no presence finding stands on: an unsure fact narrowed it (DECISIONS 516).
func (c *checker) unsureRead(e syntax.Expr) bool {
	return c.unsure[inner(e)] != nil
}

// inferredInit is the env after local o: a declared type is never unsure (DECISIONS 516).
func (c *checker) inferredInit(env *env, o *object, typ syntax.Type, value syntax.Expr) *env {
	if typ != nil {
		return env
	}
	return c.unsureInit(env, o, value)
}

// unsureInit is the env after local o, which keeps the T? of the unsure read value initialising it,
// under an unsure fact that o is present (DECISIONS 516).
func (c *checker) unsureInit(env *env, o *object, value syntax.Expr) *env {
	if !c.unsureRead(value) {
		return env
	}
	c.conds++
	return env.withFacts(facts{strconv.Itoa(o.id): {root: o.id, unsure: true, cond: c.conds, side: true}})
}

// patternFacts are the unsure facts of a path in error: the paths it could read as (DECISIONS 516).
func (c *checker) patternFacts(env *env, p syntax.Expr) facts {
	var segs []string
	for e := inner(p); ; {
		switch x := e.(type) {
		case *syntax.IdentExpr:
			if key, root, ok := c.pathKey(env, x); ok { // a root of a type in error: its own path only
				return facts{strings.Join(append([]string{key}, segs...), dot): {root: root, unsure: true}}
			}
			pat := &pattern{root: anyRoot, tail: joinSegs(segs), below: c.nextID + 1}
			return facts{pat.key(): {root: anyRoot, unsure: true, pat: pat}}
		case *syntax.SelectorExpr:
			if x.X == nil {
				return nil
			}
			if !c.hidden(x.X) {
				return c.segmentFacts(env, x.X, segs)
			}
			segs, e = append([]string{x.Name.Name}, segs...), inner(x.X)
		default:
			return nil
		}
	}
}

// segmentFacts are those of a path whose segment after the stable prefix x is in error.
func (c *checker) segmentFacts(env *env, x syntax.Expr, segs []string) facts {
	key, root, ok := c.pathKey(env, x)
	if !ok {
		return nil
	}
	pat := &pattern{root: root, prefix: key, tail: joinSegs(segs)}
	return facts{pat.key(): {root: root, unsure: true, pat: pat}}
}

// joinSegs joins path segments with dots.
func joinSegs(segs []string) string {
	return strings.Join(segs, dot)
}

// key is the pattern's key in a fact set; tagged facts add their condition and side.
func (p *pattern) key() string {
	return patternMark + strconv.Itoa(p.root) + p.prefix + patternTail + p.tail
}

// matches reports that the path key, of root id root, is one the pattern could read as.
func (p *pattern) matches(key string, root int) bool {
	if p.root == anyRoot {
		_, rest, _ := strings.Cut(key, dot)
		return root < p.below && !slices.Contains(p.kills, root) && rest == p.tail
	}
	rest, ok := strings.CutPrefix(key, p.prefix+dot)
	if !ok {
		return false
	}
	_, rest, _ = strings.Cut(rest, dot)
	return rest == p.tail
}

// killed is the pattern once root is assigned.
func (p *pattern) killed(root int) *pattern {
	q := *p
	q.kills = append(slices.Clone(p.kills), root)
	return &q
}

// tagged gives a's unsure facts the condition and side they came from (DECISIONS 516).
func (a facts) tagged(cond int, side bool) facts {
	if len(a) == 0 {
		return a
	}
	out := make(facts, len(a))
	for k, v := range a { //canon:unordered builds a map
		if v.unsure {
			v.cond, v.side = cond, side
			if v.pat != nil {
				k = v.pat.key() + strconv.Itoa(cond) + strconv.FormatBool(side)
			}
		}
		out[k] = v
	}
	return out
}

// unsured is a with every path unsure: present on one side of a test whose other operand is in error.
func (a facts) unsured() facts {
	out := make(facts, len(a))
	for k, v := range a { //canon:unordered builds a map
		v.unsure, v.kase = true, nil
		out[k] = v
	}
	return out
}

// lookup is the fact of a path: its own, else the first pattern that covers it.
func (a facts) lookup(key string, root int) (fact, bool) {
	if f, ok := a[key]; ok {
		return f, true
	}
	return a.cover(key, root, fact{})
}

// cover is the pattern fact of a covering the path, none that both sides with avoid's condition
// would add; the earliest condition first, so the choice does not depend on map order.
func (a facts) cover(key string, root int, avoid fact) (fact, bool) {
	var best fact
	found := false
	for _, f := range a { //canon:unordered keeps the least by condition and side
		if f.pat == nil || !f.pat.matches(key, root) || f.opposes(avoid) {
			continue
		}
		if !found || f.cond < best.cond || f.cond == best.cond && !f.side {
			best, found = f, true
		}
	}
	return best, found
}

// opposes reports two unsure facts from the two sides of one condition: they never hold together.
func (v fact) opposes(w fact) bool {
	return v.unsure && w.unsure && v.cond != 0 && v.cond == w.cond && v.side != w.side
}
