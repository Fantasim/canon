package cppgen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// intType is a sized integer's fixed-width C++ type (CODEGEN.md §4.1).
func intType(t ir.TypeRef) string {
	if t.Signed {
		return fmt.Sprintf(signedIntFormat, t.Bits)
	}
	return fmt.Sprintf(unsignedIntFormat, t.Bits)
}

// intRange is a narrow integer type's bounds as C++ literals; UInt64 stops at canon::kIntMax (TYPES.md §7.2).
func intRange(t ir.TypeRef) (lo, hi string) {
	if !t.Signed && t.Bits >= bits64 {
		return zeroInt, intMaxConst
	}
	if t.Signed {
		half := int64(1) << (t.Bits - 1)
		return intLit(-half), intLit(half - 1)
	}
	return zeroInt, intLit(int64(1)<<t.Bits - 1)
}

func floatType(t ir.TypeRef) string {
	if t.Bits == float32Bits {
		return cppFloat
	}
	return cppDouble
}

// storage is the C++ type a member holds for t; a ref holds its key (CODEGEN.md §4, §5.8, §7.2).
func (g *gen) storage(t ir.TypeRef) string {
	switch t.Kind {
	case types.Bool:
		return cppBool
	case types.Int:
		return intType(t)
	case types.Float:
		return floatType(t)
	case types.String:
		return cppString
	case types.Duration:
		return cppMillis
	case types.Enum, types.Record, types.Variant, types.TypeApp:
		return g.typeName(t.Named)
	case types.LitUnion:
		return g.unionStorage(t)
	case types.List:
		return g.listStorage(t)
	case types.Ref:
		return g.refStorage(t)
	case types.Map:
		return g.mapStorage(t)
	default:
		g.refuseKind(t.Kind, g.at, typeRefused)
		return cppInvalid
	}
}

// mapStorage is canon::FlatMap of the key's and the value's storage (CODEGEN.md §4.2).
func (g *gen) mapStorage(t ir.TypeRef) string {
	if t.Key == nil || t.Elem == nil {
		g.fail(fmt.Errorf("%w: map without its key or value type at %s", ErrMalformed, g.at))
		return cppInvalid
	}
	return fmt.Sprintf(flatMapFormat, g.storage(*t.Key), g.storage(*t.Elem))
}

// refuseUnion refuses the union being written, which stage E refuses first: its first arm has no string wire form (check's E3002, TYPES.md §13.2), is a dependent type (E8019 DependentType) or a ref (E8019 RefUnion).
func (g *gen) refuseUnion(t ir.TypeRef) {
	switch {
	case t.Elem == nil || !ir.StringWire(t.Elem):
		g.malformed(unionNonString, g.at)
	case t.Elem.Kind == types.TypeApp:
		g.malformed(unionDependent, g.at)
	default:
		g.malformed(unionRef, g.at) // E8019 RefUnion
	}
}

// unionStorage is a string-literal union's wire text: always a string (CODEGEN.md §4.1).
func (g *gen) unionStorage(t ir.TypeRef) string {
	if !g.stringWire(t) {
		g.refuseUnion(t)
	}
	return cppString
}

// listStorage is std::vector, or canon::KeyedList for a keyed list (CODEGEN.md §4.2).
func (g *gen) listStorage(t ir.TypeRef) string {
	if t.Elem == nil {
		g.fail(fmt.Errorf("%w: list without an element type at %s", ErrMalformed, g.at))
		return cppInvalid
	}
	if t.Elem.Kind == types.Optional {
		g.malformed(optionalElems, g.at) // E8019 OptionalElementList
	}
	elem := g.storage(*t.Elem)
	if t.KeyedBy == nil {
		return fmt.Sprintf(vectorFormat, elem)
	}
	key := g.keyField(t)
	if key == nil {
		return cppInvalid
	}
	return fmt.Sprintf(keyedListFormat, g.storage(key.Type), elem)
}

// refStorage is a ref's key (CODEGEN.md §5.8); a define ref's value has members of its own (define_refs.go).
func (g *gen) refStorage(t ir.TypeRef) string {
	if t.Key == nil {
		g.fail(fmt.Errorf("%w: ref without a key type at %s", ErrMalformed, g.at))
		return cppInvalid
	}
	return g.storage(*t.Key)
}

// byValue reports a type whose getter returns a copy: scalars, enums, a ref to one (§5.4, §5.8).
func byValue(t ir.TypeRef) bool {
	switch t.Kind {
	case types.Bool, types.Int, types.Float, types.Duration, types.Enum:
		return true
	case types.Ref:
		return t.Key != nil && byValue(*t.Key)
	default:
		return false
	}
}

// getterType is what a getter of t returns (CODEGEN.md §4.1–§4.3).
func (g *gen) getterType(t ir.TypeRef, optional bool) string {
	s := g.storage(t)
	switch {
	case byValue(t) && optional:
		return fmt.Sprintf(optionalFormat, s)
	case byValue(t):
		return s
	case optional:
		return fmt.Sprintf(constPtrFormat, s)
	}
	return fmt.Sprintf(constRefFormat, s)
}

// memberType is the storage of a field, std::optional for `T?` (CODEGEN.md §7.2).
func (g *gen) memberType(t ir.TypeRef, optional bool) string {
	if optional {
		return fmt.Sprintf(optionalFormat, g.storage(t))
	}
	return g.storage(t)
}

// memberInit initializes a scalar member (CODEGEN.md §7.2: members are initialized).
func memberInit(t ir.TypeRef, optional bool) string {
	switch {
	case optional:
		return ""
	case t.Kind == types.Bool:
		return initFalse
	case t.Kind == types.Int:
		return initZero
	case t.Kind == types.Float && t.Bits == float32Bits:
		return initZeroF
	case t.Kind == types.Float:
		return initZeroD
	case t.Kind == types.Duration:
		return initBraceZero
	case t.Kind == types.Enum:
		return initBraces
	case t.Kind == types.Ref && t.Key != nil:
		return memberInit(*t.Key, false)
	}
	return ""
}

// caseName is a case's class, T + UpperCamel(c), or its @cpp(name:) (CODEGEN.md §3.3, §3.5).
func (g *gen) caseName(v *ir.Variant, c *ir.Case) string {
	return g.qualifier(v.Pkg) + g.pl.CaseName(v, c)
}

// kindName is a variant's kind enum, TKind, qualified when v is imported (CODEGEN.md §5.5).
func (g *gen) kindName(v *ir.Variant) string { return g.qualifier(v.Pkg) + g.pl.KindName(v) }

// keyField is the key field of a keyed list's element record.
func (g *gen) keyField(t ir.TypeRef) *ir.Field {
	rec, ok := t.Elem.Named.(*ir.Record)
	if !ok {
		g.fail(fmt.Errorf("%w: keyed list of a non-record at %s", ErrMalformed, g.at))
		return nil
	}
	for _, f := range rec.Fields {
		if f.Name == t.KeyedBy.Name {
			return f
		}
	}
	g.fail(fmt.Errorf("%w: no key field %s in %s", ErrMalformed, t.KeyedBy.Name, rec.Name))
	return nil
}

// kindText names a kind in messages.
func kindText(k types.Kind) string {
	if int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return fmt.Sprintf(kindNumberFormat, k)
}
