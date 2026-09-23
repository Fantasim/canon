package wire

import (
	"fmt"
	"strconv"

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
