package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Fit is how a dependent value fits its computed type.
type Fit uint8

// toBranch converts a dependent value to a computed type of one kind; written: a literal or loaded.
type toBranch func(v value.Value, base types.Type, written bool) (value.Value, Fit)

// branchConverters convert by the computed type's kind; another kind takes an assignable value (TYPES.md §6.2).
var branchConverters = map[types.Kind]toBranch{
	types.Enum:     toMember,
	types.Variant:  toCase,
	types.Case:     toCase,
	types.Ref:      toRef,
	types.Int:      toScalar,
	types.Duration: toScalar,
	types.String:   toScalar,
	types.Bool:     toScalar,
	types.Float:    toFloat,
}

// ToBranch is v as a value of base, a computed type without layers; only a written v converts (TYPES.md §6.2, §11.6).
func ToBranch(v value.Value, base types.Type, written bool) (value.Value, Fit) {
	if c, ok := branchConverters[base.Kind()]; ok {
		return c(v, base, written)
	}
	if k, ok := containerKinds[base.Kind()]; ok && k == containerOf(v) {
		return v, Fits // its parts are converted one by one
	}
	if _, isSymbol := v.(*value.Symbol); !isSymbol && v.Type() != nil && types.Assignable(v.Type(), base) {
		return v, Fits
	}
	return v, NoFit
}

// toMember resolves a symbol to the member of that name (TYPES.md §4.1).
func toMember(v value.Value, base types.Type, _ bool) (value.Value, Fit) {
	e, _ := base.(*types.EnumType)
	switch x := v.(type) {
	case *value.Symbol:
		if i := slices.IndexFunc(e.Members, func(m *types.Member) bool { return m.Name == x.Name }); i >= 0 {
			return &value.Member{Enum: e, Index: i, P: x.P}, Fits
		}
	case *value.Member:
		if x.Enum == e {
			return v, Fits
		}
	}
	return v, NoFit
}

// toCase resolves a symbol to the case of that name written bare (TYPES.md §8.2).
func toCase(v value.Value, base types.Type, _ bool) (value.Value, Fit) {
	cases := []*types.CaseType{}
	switch b := base.(type) {
	case *types.VariantType:
		cases = b.Cases
	case *types.CaseType:
		cases = append(cases, b)
	}
	switch x := v.(type) {
	case *value.Symbol:
		if i := slices.IndexFunc(cases, func(c *types.CaseType) bool { return c.Name == x.Name }); i >= 0 {
			return bare(x, cases[i])
		}
	case *value.Record:
		if c, ok := x.T.Base().(*types.CaseType); ok && slices.Contains(cases, c) {
			return v, Fits
		}
	}
	return v, NoFit
}

// bare is case c a symbol names, its fields still to default.
func bare(s *value.Symbol, c *types.CaseType) (value.Value, Fit) {
	n := len(c.Fields)
	rec := &value.Record{T: c, Fields: make([]value.Value, n), Set: make([]bool, n), P: s.P}
	if n == 0 {
		return rec, Fits
	}
	return rec, BareCase
}

// toRef resolves a symbol, or a written literal of the key's type, to a key; an entry stays (TYPES.md §4.1).
func toRef(v value.Value, base types.Type, written bool) (value.Value, Fit) {
	rt, _ := base.(*types.RefType)
	c := rt.Target
	var k value.Key
	switch x := v.(type) {
	case *value.Ref:
		if xt, ok := x.T.Base().(*types.RefType); ok && xt.Target == c {
			return v, Fits
		}
		return v, NoFit
	case *value.Record:
		return v, Fits
	case *value.Symbol:
		k, written = value.Key{S: x.Name}, true
	case *value.Str:
		k = value.Key{S: x.V}
	case *value.Int:
		k = value.Key{I: x.V, IsInt: true}
	default:
		return v, NoFit
	}
	intKeys := c != nil && c.KeyedBy != nil && c.KeyedBy.Type.Base().Kind() == types.Int
	if c == nil || !written || k.IsInt != intKeys {
		return v, NoFit
	}
	return &value.Ref{T: base, Key: k, P: v.Prov()}, Fits
}

// toFloat keeps a Float, and makes a written integer literal a Float (TYP-10); storage rounds a Float32.
func toFloat(v value.Value, base types.Type, written bool) (value.Value, Fit) {
	switch x := v.(type) {
	case *value.Int:
		if written {
			return &value.Float{V: float64(x.V), T: base, P: x.P}, Fits
		}
	case *value.Float:
		return v, Fits
	}
	return v, NoFit
}

// toScalar keeps an Int, Duration, String or Bool of the computed kind; storage checks a range.
func toScalar(v value.Value, base types.Type, _ bool) (value.Value, Fit) {
	if kindOf(v) == base.Kind() {
		return v, Fits
	}
	return v, NoFit
}

// containerKinds is the kind of value each container type holds.
var containerKinds = map[types.Kind]types.Kind{
	types.List: types.List, types.Map: types.Map, types.DepMap: types.Map, types.Pair: types.Pair,
}

// containerOf is the kind of a list, map or pair value, Never for any other.
func containerOf(v value.Value) types.Kind {
	switch v.(type) {
	case *value.List:
		return types.List
	case *value.Map:
		return types.Map
	case *value.Pair:
		return types.Pair
	}
	return types.Never
}

// IsContainer reports a list, map or pair, whose parts verification judges one by one.
func IsContainer(v value.Value) bool {
	return containerOf(v) != types.Never
}

// kindOf is the kind of a scalar value, Never for any other.
func kindOf(v value.Value) types.Kind {
	switch v.(type) {
	case *value.Int:
		return types.Int
	case *value.Dur:
		return types.Duration
	case *value.Str:
		return types.String
	case *value.Bool:
		return types.Bool
	}
	return types.Never
}
