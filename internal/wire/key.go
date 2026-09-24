package wire

import (
	"fmt"
	"strconv"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// keyText is the wire key of a map key or argument, new to seen (WIRE.md §5.8, E3317).
func keyText(k value.Value, seen map[string]bool) (string, error) {
	key, err := keyOf(k)
	if err != nil {
		return "", err
	}
	if seen[key] {
		return "", fmt.Errorf("%w: %q", ErrKeyCollision, key)
	}
	seen[key] = true
	return key, nil
}

func keyOf(k value.Value) (string, error) {
	switch x := k.(type) {
	case *value.Str:
		return x.V, nil
	case *value.Int:
		return strconv.FormatInt(x.V, decimalBase), nil
	case *value.Bool:
		return strconv.FormatBool(x.V), nil
	case *value.Member:
		return memberKey(x.Enum, x.Index)
	case *value.Ref:
		return refKey(x)
	}
	return "", fmt.Errorf("%w: key %T", ErrNoWire, k)
}

// memberKey is a member's wire value as text: its code in decimal with @json(codes).
func memberKey(enum *types.EnumType, index int) (string, error) {
	if index < 0 || index >= len(enum.Members) {
		return "", fmt.Errorf("%w: member %d of %s", ErrShape, index, enum)
	}
	if enum.WireCodes {
		return strconv.FormatInt(enum.Members[index].Code, decimalBase), nil
	}
	return enum.Members[index].Wire, nil
}

// refKey is a ref's entry key as text: the key field's wire value, decimal for an integer.
func refKey(r *value.Ref) (string, error) {
	if r.Key.IsInt {
		return strconv.FormatInt(r.Key.I, decimalBase), nil
	}
	enum := keyEnum(r)
	if enum == nil {
		return r.Key.S, nil
	}
	i, err := memberNamed(enum, r.Key.S)
	if err != nil {
		return "", err
	}
	return memberKey(enum, i)
}

// refNode is a ref's key (§5.9): a string, an integer, or an enum key field's wire value.
func refNode(r *value.Ref) (*node, error) {
	if r.Key.IsInt {
		return scalar(strconv.AppendInt(nil, r.Key.I, decimalBase)), nil
	}
	enum := keyEnum(r)
	if enum == nil {
		return stringNode(r.Key.S), nil
	}
	i, err := memberNamed(enum, r.Key.S)
	if err != nil {
		return nil, err
	}
	return memberNode(enum, i)
}

func memberNamed(enum *types.EnumType, name string) (int, error) {
	for i, m := range enum.Members {
		if m.Name == name {
			return i, nil
		}
	}
	return 0, fmt.Errorf("%w: %s is no member of %s", ErrShape, name, enum)
}

// keyEnum is the enum type of the key field of the keyed list a ref targets, or nil.
func keyEnum(r *value.Ref) *types.EnumType {
	if r.T == nil {
		return nil
	}
	rt, ok := r.T.Base().(*types.RefType)
	if !ok || rt.Target == nil || rt.Target.KeyedBy == nil {
		return nil
	}
	enum, _ := rt.Target.KeyedBy.Type.Base().(*types.EnumType)
	return enum
}

// mapKey decodes an object key as a key of kt (WIRE.md §5.8), by the rules of a value of kt.
func (r *run) mapKey(m *jsonsrc.Member, kt types.Type, fr *frame) value.Value {
	s := keyAt(m)
	switch kt.Kind() {
	case types.String:
		return &value.Str{V: m.Key, T: kt, P: s.prov()}
	case types.Int:
		if i, ok := r.intKey(s, m.Key, kt); ok {
			return &value.Int{V: i, T: kt, P: s.prov()}
		}
	case types.Enum:
		return r.enumKey(s, m.Key, kt.Base().(*types.EnumType))
	case types.Ref:
		return r.refMapKey(m, kt, fr)
	case types.LitUnion:
		u := kt.Base().(*types.LitUnionType)
		if isLiteral(u, m.Key) {
			return &value.Str{V: m.Key, T: kt, P: s.prov()}
		}
		return r.mapKey(m, u.Of, fr)
	case types.TypeApp:
		return r.dependentKey(m, kt, fr)
	default:
		r.misuse(ErrNoWireType, kt)
	}
	return nil
}

// intKey is a canonical decimal integer key, E7103 otherwise, in kt's range (E3201).
func (r *run) intKey(s site, key string, kt types.Type) (int64, bool) {
	if !intKeyPattern.MatchString(key) {
		r.report(diag.E7103.AtMapKey(s.span, key), s.node)
		return 0, false
	}
	i, err := strconv.ParseInt(key, decimalBase, float64Bits)
	if err != nil || !fits(i, kt) {
		r.report(diag.E3201.At(s.span, literal(key), kt), s.node)
		return 0, false
	}
	return i, true
}

// enumKey is a member by its wire value, or by its code in decimal with @json(codes).
func (r *run) enumKey(s site, key string, e *types.EnumType) value.Value {
	if !e.WireCodes {
		return r.memberByWire(s, key, e)
	}
	if !intKeyPattern.MatchString(key) {
		r.report(diag.E7103.AtMapKey(s.span, key), s.node)
		return nil
	}
	return r.memberByCode(s, key, e)
}

// refMapKey is a ref key: a table's entry key, or a keyed list's key field as text.
func (r *run) refMapKey(m *jsonsrc.Member, kt types.Type, fr *frame) value.Value {
	rt := kt.Base().(*types.RefType)
	key := value.Key{S: m.Key}
	if rt.Target != nil && rt.Target.KeyedBy != nil {
		k, ok := keyOfValue(r.mapKey(m, rt.Target.KeyedBy.Type, fr))
		if !ok {
			return nil
		}
		key = k
	}
	return &value.Ref{T: kt, Key: key, Owner: r.owner(rt.Target), P: keyAt(m).prov()}
}

// dependentKey is a key of its application's branch; on Never, a symbol (DECISIONS 175).
func (r *run) dependentKey(m *jsonsrc.Member, kt types.Type, fr *frame) value.Value {
	branch, inner, ok := r.branch(kt.Base().(*types.TypeAppType), fr)
	switch {
	case !ok:
		return nil
	case branch.Base().Kind() == types.Never:
		return &value.Symbol{Name: m.Key, T: kt, P: keyAt(m).prov()}
	}
	return r.mapKey(m, branch, inner)
}
