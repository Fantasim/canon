package wire

import (
	"fmt"
	"math"
	"strconv"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// scope is a field's @json(unit:) and @json(int), reset by a record (WIRE.md §4.1).
type scope struct {
	unit  types.Unit
	asInt bool
}

// encoder turns values into nodes: methods gives the `$` keys (§5.11), omitAbsent the §8.5 form.
type encoder struct {
	methods    Methods
	omitAbsent bool
}

// value encodes v per WIRE.md §5 in scope s.
func (e *encoder) value(v value.Value, s scope) (*node, error) {
	switch x := v.(type) {
	case *value.Bool:
		return boolNode(x.V, s.asInt), nil
	case *value.Int:
		return scalar(strconv.AppendInt(nil, x.V, decimalBase)), nil
	case *value.Float:
		return floatNode(x)
	case *value.Str:
		return stringNode(x.V), nil
	case *value.Dur:
		return durNode(x.Ms, s.unit)
	case *value.Member:
		return memberNode(x.Enum, x.Index)
	case *value.None:
		return text(textNull), nil
	case *value.Record:
		return e.record(x, objectNode())
	case *value.List:
		return e.list(x.Elems, s)
	case *value.Map:
		return e.mapNode(x, s)
	case *value.Table:
		return e.table(x)
	case *value.Ref:
		return refNode(x)
	}
	return nil, fmt.Errorf("%w: %T", ErrNoWire, v)
}

// boolNode is true / false, or 0 / 1 under @json(int) (§5.2).
func boolNode(v, asInt bool) *node {
	switch {
	case asInt && v:
		return text(textOne)
	case asInt:
		return text(textZero)
	case v:
		return text(textTrue)
	}
	return text(textFalse)
}

// floatNode is ECMAScript Number::toString of a float64, or of the shortest float32 (§7.2).
func floatNode(v *value.Float) (*node, error) {
	if math.IsNaN(v.V) || math.IsInf(v.V, 0) {
		return nil, fmt.Errorf("%w: %v", ErrNoWire, v.V)
	}
	return text(types.FloatText(v.V, floatBits(v.T))), nil
}

// floatBits is 32 for a Float32 value, 64 otherwise.
func floatBits(t types.Type) int {
	if t == nil {
		return float64Bits
	}
	if b, ok := t.Base().(types.Basic); ok && b.Bits == float32Bits {
		return float32Bits
	}
	return float64Bits
}

// durNode is the integer count of the field's unit (§5.1); a remainder has no encoding (E8102).
func durNode(ms int64, unit types.Unit) (*node, error) {
	per := unit.Millis()
	if ms%per != 0 {
		return nil, fmt.Errorf("%w: %d ms in %s", ErrNotWholeUnit, ms, unit)
	}
	return scalar(strconv.AppendInt(nil, ms/per, decimalBase)), nil
}

// memberNode is the member's wire string, or its code with @json(codes) (§5.3).
func memberNode(enum *types.EnumType, index int) (*node, error) {
	key, err := memberKey(enum, index)
	if err != nil {
		return nil, err
	}
	if enum.WireCodes {
		return text(key), nil
	}
	return stringNode(key), nil
}

func (e *encoder) list(elems []value.Value, s scope) (*node, error) {
	arr := arrayNode(len(elems))
	for _, v := range elems {
		n, err := e.value(v, s)
		if err != nil {
			return nil, err
		}
		arr.elems = append(arr.elems, n)
	}
	return arr, nil
}

// mapNode is an object in insertion order, keyed by §5.8's wire keys, which must differ (E3317).
func (e *encoder) mapNode(m *value.Map, s scope) (*node, error) {
	if len(m.Keys) != len(m.Vals) {
		return nil, fmt.Errorf("%w: %d keys, %d values", ErrShape, len(m.Keys), len(m.Vals))
	}
	obj := objectNode()
	seen := make(map[string]bool, len(m.Keys))
	for i, k := range m.Keys {
		key, err := keyText(k, seen)
		if err != nil {
			return nil, err
		}
		v, err := e.value(m.Vals[i], s)
		if err != nil {
			return nil, err
		}
		obj.add(key, v)
	}
	return obj, nil
}

// table is a nested table: an object keyed by entry id, retired rows marked first (§5.7).
func (e *encoder) table(t *value.Table) (*node, error) {
	obj := objectNode()
	for _, r := range t.Entries {
		if r.Ident == nil {
			return nil, fmt.Errorf("%w: table entry without identity", ErrShape)
		}
		head := objectNode()
		if r.Ident.Retired {
			head.add(keyRetired, text(textTrue))
		}
		row, err := e.record(r, head)
		if err != nil {
			return nil, err
		}
		obj.add(r.Ident.Key.Text(), row)
	}
	return obj, nil
}
