package edit

import (
	"slices"
	"time"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// asWritten is scalar literal e given to dependent type t when a branch takes it (TYPES.md 11.4,
// API.md V1): a string or integer kept as written for verification to convert, a float, Bool or
// Duration of its own type; false for another expression, or a literal no branch takes.
func (st *srcTyping) asWritten(e syntax.Expr, t types.Type) (value.Value, bool) {
	var g given
	switch x := e.(type) {
	case *syntax.IntLit:
		if !x.Value.IsInt64() {
			return nil, false
		}
		g = givenInt(x.Value.Int64())
	case *syntax.StringLit, *syntax.RawStringLit:
		s, ok := constString(e)
		if !ok {
			return nil, false
		}
		g = givenStr(s)
	case *syntax.FloatLit:
		g = givenFloat(floatOf(x))
	case *syntax.BoolLit:
		g = givenKind(&value.Bool{V: x.Value}, types.Bool)
	case *syntax.DurationLit:
		g = givenKind(&value.Dur{Ms: x.Millis}, types.Duration)
	default:
		return nil, false
	}
	return g.to(t)
}

// givenLit is a scalar Lit given to dependent type t, as asWritten takes a literal; false for
// a Lit of another form, or one no branch of t takes (V1).
func givenLit(lit Lit, t types.Type) (value.Value, bool) {
	var g given
	switch x := lit.(type) {
	case Int:
		g = givenInt(int64(x))
	case Str:
		g = givenStr(string(x))
	case Float:
		g = givenFloat(float64(x))
	case Bool:
		g = givenKind(&value.Bool{V: bool(x)}, types.Bool)
	case Dur:
		d := time.Duration(x)
		if d%time.Millisecond != 0 {
			return nil, false
		}
		g = givenKind(&value.Dur{Ms: d.Milliseconds()}, types.Duration)
	default:
		return nil, false
	}
	return g.to(t)
}

// given is a scalar literal given to a dependent type: the value kept, and the tests of a
// branch taking it (branchTest).
type given struct {
	v     value.Value
	fits  func(types.Type) bool
	union func([]string) bool
}

// to is g's value when t, or a branch it computes, takes it.
func (g given) to(t types.Type) (value.Value, bool) {
	bt := branchTest{fits: g.fits, union: g.union, seen: map[*types.TypeFunc]bool{}}
	return g.v, bt.takes(t)
}

func givenInt(n int64) given {
	return given{v: &value.Int{V: n, T: types.IntType}, fits: func(b types.Type) bool { return intTaken(n, b) }}
}

func givenStr(s string) given {
	return given{
		v:     &value.Str{V: s, T: types.StringType},
		fits:  func(b types.Type) bool { return strTaken(s, b) },
		union: func(lits []string) bool { return slices.Contains(lits, s) },
	}
}

// givenFloat is a finite float, taken by a Float branch (floatValue).
func givenFloat(f float64) given {
	return given{v: &value.Float{V: f, T: types.FloatType}, fits: func(b types.Type) bool {
		_, ok := floatValue(f, b)
		return ok
	}}
}

// givenKind is v, taken by a branch of kind k.
func givenKind(v value.Value, k types.Kind) given {
	return given{v: v, fits: func(b types.Type) bool { return b.Base().Kind() == k }}
}

// branchTest asks whether a branch of a dependent type takes a literal: fits for a plain
// branch, union for a literal union's own literals (TYPES.md 13.2); seen stops at a type
// function met twice.
type branchTest struct {
	fits  func(types.Type) bool
	union func([]string) bool
	seen  map[*types.TypeFunc]bool
}

// takes reports t, or one of the branches it computes, taking the literal.
func (bt branchTest) takes(t types.Type) bool {
	switch b := present(t).Base().(type) {
	case *types.TypeAppType:
		return bt.fnTakes(b.Fn)
	case *types.DepUnionType:
		return bt.fnTakes(b.Fn)
	case *types.LitUnionType:
		return bt.union != nil && bt.union(b.Literals) || bt.takes(b.Of)
	}
	return bt.fits(present(t))
}

// fnTakes reports one of fn's results, its body or an arm's, taking the literal.
func (bt branchTest) fnTakes(fn *types.TypeFunc) bool {
	if bt.seen[fn] {
		return false
	}
	bt.seen[fn] = true
	results := []types.Type{fn.Body}
	for _, a := range fn.Arms {
		results = append(results, a.Result)
	}
	return slices.ContainsFunc(results, func(r types.Type) bool { return r != nil && bt.takes(r) })
}

// intTaken reports an integer literal n fitting branch b: an integer type, a Float, a ref into
// an integer-keyed collection (TYPES.md 4.1).
func intTaken(n int64, b types.Type) bool {
	_, isInt := intValue(n, b)
	_, isKey := refValue(b, func(kt types.Type) value.Value { return intKeyValue(n, kt) })
	return isInt || isKey
}

// strTaken reports a string literal s fitting branch b: a String, a ref into a collection keyed
// by strings (TYPES.md 4.1).
func strTaken(s string, b types.Type) bool {
	_, isStr := strValue(s, b)
	_, isKey := refValue(b, func(kt types.Type) value.Value { return textKeyValue(s, kt, false) })
	return isStr || isKey
}
