//go:build linux

package main

import (
	"context"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
	"unicode"

	canon "github.com/fantasim/canonlang/api"
)

// Fresh values: drawn within a field's refinement (its TypeInfo.Expr), never held before.
const (
	freshTries    = 8
	freshSpan     = 10  // how far an unbounded number moves, as a hand edit would
	freshCents    = 100 // floats are drawn to the hundredth, as benchgen writes them
	rangeSep      = ".."
	rangeIncl     = "="
	refPrefix     = "ref "
	optionalMark  = "?"
	durationUnits = "dhms"
)

// durUnit is one unit of a canonical Duration text (LEX-04).
type durUnit struct {
	unit string
	d    time.Duration
}

// canonDurations are the units of a canonical Duration text, ms before m.
var canonDurations = []durUnit{{"ms", time.Millisecond}, {"d", 24 * time.Hour}, {"h", time.Hour}, {"m", time.Minute}, {"s", time.Second}}

// freshValues draws values: enums' live members, each ref target's keys, what each field held.
type freshValues struct {
	p      *canon.Project
	rng    *rand.Rand
	dir    string // the project directory, which asset roots are relative to
	enums  map[string][]string
	assets map[string]string // asset root to its directory
	keys   map[string][]string
	files  map[string][]string // an asset root's files, by root and extensions
	held   map[string]map[any]bool
}

// freshByKind draws a value of a field's kind, and the key it is held under.
var freshByKind = map[canon.ValueKind]func(*freshValues, *canon.Value) (canon.Lit, any, bool){
	canon.KindInt: freshInt, canon.KindFloat: freshFloat, canon.KindDuration: freshDur,
	canon.KindBool: freshBool, canon.KindString: freshText, canon.KindAsset: freshText, canon.KindEnum: freshMember,
	canon.KindRef: freshRef,
}

// value is a fresh value for v: of its type, within its refinement, never held before.
func (f *freshValues) value(v *canon.Value) (canon.Lit, bool) {
	held := f.held[v.Path]
	if held == nil {
		held = map[any]bool{current(v): true}
		f.held[v.Path] = held
	}
	for range freshTries {
		lit, key, ok := freshByKind[v.Kind](f, v)
		if !ok {
			return nil, false
		}
		if !held[key] {
			held[key] = true
			return lit, true
		}
	}
	return nil, false
}

// current is v's value under the key freshByKind gives it.
func current(v *canon.Value) any {
	switch v.Kind {
	case canon.KindInt:
		n, _ := v.Int()
		return n
	case canon.KindFloat:
		x, _ := v.Float()
		return x
	case canon.KindDuration:
		d, _ := v.Dur()
		return d
	case canon.KindBool:
		b, _ := v.Bool()
		return b
	case canon.KindEnum:
		_, name, _ := v.Member()
		return name
	case canon.KindRef:
		k, _ := v.Key()
		return k
	}
	if s, ok := v.Str(); ok {
		return s
	}
	return v.Text
}

func freshInt(f *freshValues, v *canon.Value) (canon.Lit, any, bool) {
	n, _ := v.Int()
	lo, hi := n-freshSpan, n+freshSpan
	if r, ok := bounds(v.Type.Expr); ok {
		lo, hi = intOr(r.lo, lo), intOr(r.hi, hi)
		if r.hi != "" && !r.incl {
			hi--
		}
	}
	if hi < lo {
		return nil, nil, false
	}
	x := lo + f.rng.Int64N(hi-lo+1)
	return canon.Int(x), x, true
}

func freshFloat(f *freshValues, v *canon.Value) (canon.Lit, any, bool) {
	x, _ := v.Float()
	lo, hi := x-freshSpan, x+freshSpan
	if r, ok := bounds(v.Type.Expr); ok {
		lo, hi = floatOr(r.lo, lo), floatOr(r.hi, hi)
	}
	lo, hi = math.Ceil(lo*freshCents), math.Floor(hi*freshCents)
	if hi <= lo {
		return nil, nil, false
	}
	y := (lo + float64(f.rng.Int64N(int64(hi-lo)))) / freshCents // hi excluded: exact for ..< too
	return canon.Float(y), y, true
}

// freshDur draws in the largest unit the current value is a whole number of: a JSON field's
// wire unit (`@json(unit: s)`) takes only whole units of it.
func freshDur(f *freshValues, v *canon.Value) (canon.Lit, any, bool) {
	d, _ := v.Dur()
	unit := grain(d)
	if unit == 0 {
		return nil, nil, false
	}
	lo, hi := time.Duration(0), 2*d+unit
	if r, ok := bounds(v.Type.Expr); ok {
		lo, hi = durOr(r.lo, lo), durOr(r.hi, hi)
		if r.hi != "" && !r.incl {
			hi--
		}
	}
	first, last := (lo+unit-1)/unit, hi/unit
	if last < first {
		return nil, nil, false
	}
	x := (first + time.Duration(f.rng.Int64N(int64(last-first)+1))) * unit
	return canon.Dur(x), x, true
}

// grain is the largest unit of canonDurations d is a whole number of; 0 for no duration.
func grain(d time.Duration) time.Duration {
	best := time.Duration(0)
	for _, u := range canonDurations {
		if d > 0 && d%u.d == 0 && u.d > best {
			best = u.d
		}
	}
	return best
}

func freshBool(_ *freshValues, v *canon.Value) (canon.Lit, any, bool) {
	b, _ := v.Bool()
	return canon.Bool(!b), !b, true
}

func freshMember(f *freshValues, v *canon.Value) (canon.Lit, any, bool) {
	members := f.enums[strings.TrimSuffix(v.Type.Expr, optionalMark)]
	if len(members) == 0 {
		return nil, nil, false
	}
	m := members[f.rng.IntN(len(members))]
	return canon.Member(m), m, true
}

// freshRef is another key of the collection the ref targets.
func freshRef(f *freshValues, v *canon.Value) (canon.Lit, any, bool) {
	keys := f.targetKeys(v.Type.Expr)
	if len(keys) == 0 {
		return nil, nil, false
	}
	k := keys[f.rng.IntN(len(keys))]
	if n, err := strconv.ParseInt(k, 10, 64); err == nil {
		return canon.IntKey(n), k, true
	}
	return canon.Key(k), k, true
}

// targetKeys are the keys of the collection a ref type names ("ref items.categories" is
// items:categories), read once.
func (f *freshValues) targetKeys(expr string) []string {
	name := strings.TrimSuffix(strings.TrimPrefix(expr, refPrefix), optionalMark)
	if keys, ok := f.keys[name]; ok {
		return keys
	}
	var keys []string
	if at := strings.LastIndexByte(name, '.'); at > 0 {
		keys = f.keysAt(name[:at] + ":" + name[at+1:])
	}
	f.keys[name] = keys
	return keys
}

// keysAt are the keys of the collection at path; none when it cannot be read.
func (f *freshValues) keysAt(path string) []string {
	coll, err := f.p.Value(context.Background(), path)
	if err != nil {
		return nil
	}
	var keys []string
	for _, c := range coll.Children() {
		if k, ok := c.Key(); ok {
			keys = append(keys, k)
		}
	}
	return keys
}

// span is a refinement's range bounds as written ("" when open), and whether the upper one is
// included: Int(1..=10), Float(0..500), Duration(1m..).
type span struct {
	lo, hi string
	incl   bool
}

func bounds(expr string) (span, bool) {
	open, end := strings.IndexByte(expr, '('), strings.LastIndexByte(expr, ')')
	if open < 0 || end < open {
		return span{}, false
	}
	var r span
	lo, hi, ok := strings.Cut(expr[open+1:end], rangeSep)
	r.lo = lo
	r.hi, r.incl = strings.CutPrefix(hi, rangeIncl)
	return r, ok
}

func intOr(s string, or int64) int64 {
	n, err := strconv.ParseInt(strings.ReplaceAll(s, "_", ""), 10, 64)
	if err != nil {
		return or
	}
	return n
}

func floatOr(s string, or float64) float64 {
	x, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
	if err != nil {
		return or
	}
	return x
}

// durOr reads a canonical Duration text (1h8m41s999ms, 1d), else or.
func durOr(s string, or time.Duration) time.Duration {
	if s == "" || !strings.ContainsAny(s, durationUnits) {
		return or
	}
	var total time.Duration
	for s != "" {
		i := strings.IndexFunc(s, func(r rune) bool { return !unicode.IsDigit(r) })
		if i <= 0 {
			return or
		}
		n, _ := strconv.ParseInt(s[:i], 10, 64)
		unit, ok := durationUnit(s[i:])
		if !ok {
			return or
		}
		total += time.Duration(n) * unit.d
		s = s[i+len(unit.unit):]
	}
	return total
}

func durationUnit(s string) (durUnit, bool) {
	for _, u := range canonDurations {
		if strings.HasPrefix(s, u.unit) {
			return u, true
		}
	}
	return canonDurations[0], false
}
