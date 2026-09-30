package edit

import (
	"context"
	"math"
	"slices"
	"time"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// Typer types an operation's values against the static type expected where they go (V1, V2):
// a typed record holds only the fields the value writes (Set), the others nil (V3). A typed
// value's provenance is nil or points into a file of its own, never into the project.
type Typer struct {
	Host  wire.Host    // FromJSON's defaults and dereferences: required
	Pkg   string       // the package whose names FromJSON's messages print unqualified
	Outer wire.Outer   // what the value's type arguments name around it (WIRE.md 5.9)
	marks *symMarks    // what the applier keeps of the symbols typed, nil for none
	json  bool         // the value goes into a JSON source, its Durations in whole units
	scope *types.Field // the field whose wire rules FromJSON and those units follow, nil for none
}

// Value is lit typed as a value of t; a value that does not fit is a *ValueError (V1).
func (ty Typer) Value(ctx context.Context, lit Lit, t types.Type) (value.Value, error) {
	if ty.Host == nil {
		return nil, ErrNoHost
	}
	tc := &typing{ctx: ctx, ty: ty}
	v, err := tc.value(lit, t)
	if err != nil || !ty.json {
		return v, err
	}
	if err := tc.wholeUnits(lit, t, v, unitOf(ty.scope)); err != nil {
		return nil, err
	}
	return v, nil
}

// Key is lit typed as a key of type kt: a map key, a table entry's key (String) or a new key
// of Rename, where a PathKey is read as a path key (API.md E25, P1, P2).
func (ty Typer) Key(ctx context.Context, lit Lit, kt types.Type) (value.Value, error) {
	if ty.Host == nil {
		return nil, ErrNoHost
	}
	tc := &typing{ctx: ctx, ty: ty}
	return tc.key(lit, kt)
}

// typing is one Value or Key call: where in the value it stands, for the error's detail.
type typing struct {
	ctx context.Context
	ty  Typer
	at  []Seg
}

// value is lit as a value of t: a T is accepted where a T? is expected (SPEC §5.6).
func (tc *typing) value(lit Lit, t types.Type) (value.Value, error) {
	if len(tc.at) > maxLitDepth {
		return nil, tc.mismatch(lit, t, detailDeep)
	}
	switch x := lit.(type) {
	case FromJSON:
		return tc.fromJSON(x, t)
	case Source:
		return tc.source(x, t)
	case None:
		if isOptional(t) {
			return &value.None{T: t}, nil
		}
		return nil, tc.mismatch(lit, t, "")
	}
	et := present(t)
	if dependent(et) {
		return tc.symbol(lit, et)
	}
	switch x := lit.(type) {
	case List:
		return tc.list(x, et)
	case Obj:
		return tc.obj(x, et)
	case Map:
		return tc.mapLit(x, et)
	case Case:
		return tc.caseLit(x, et)
	}
	if v, ok := scalar(lit, et); ok {
		return v, nil
	}
	return nil, tc.mismatch(lit, t, scalarDetail(lit))
}

// key is lit as a key of kt: Key, IntKey and PathKey are keys; any other form is a value of kt.
func (tc *typing) key(lit Lit, kt types.Type) (value.Value, error) {
	if s, ok := tc.dependentKey(lit, kt); ok {
		return s, nil
	}
	var v value.Value
	switch x := lit.(type) {
	case Key:
		v = textKeyValue(string(x), kt, false)
	case PathKey:
		v = textKeyValue(string(x), kt, true)
	case IntKey:
		v = intKeyValue(int64(x), kt)
	default:
		return tc.value(lit, kt)
	}
	if v == nil {
		return nil, tc.mismatch(lit, kt, "")
	}
	return v, nil
}

// scalar is a scalar Lit as a value of t, whose optional layer is peeled.
func scalar(lit Lit, t types.Type) (value.Value, bool) {
	switch x := lit.(type) {
	case Bool:
		return &value.Bool{V: bool(x)}, t.Base().Kind() == types.Bool
	case Int:
		return intValue(int64(x), t)
	case Float:
		return floatValue(float64(x), t)
	case Str:
		return strValue(string(x), t)
	case Dur:
		d := time.Duration(x)
		return &value.Dur{Ms: d.Milliseconds()}, t.Base().Kind() == types.Duration && d%time.Millisecond == 0
	case Member:
		return memberValue(string(x), t)
	case Key:
		return refValue(t, func(kt types.Type) value.Value { return textKeyValue(string(x), kt, false) })
	case IntKey:
		return refValue(t, func(kt types.Type) value.Value { return intKeyValue(int64(x), kt) })
	}
	return nil, false
}

// scalarDetail says why a scalar fits no type at all: a Dur with a fraction of a millisecond,
// a Float that is not finite.
func scalarDetail(lit Lit) string {
	switch x := lit.(type) {
	case Dur:
		if time.Duration(x)%time.Millisecond != 0 {
			return detailFraction
		}
	case Float:
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return detailNotFinite
		}
	}
	return ""
}

// intValue is an integer of an integer type, or of a Float type as an integer literal would be (TYP-10).
func intValue(n int64, t types.Type) (value.Value, bool) {
	switch b := t.Base().(type) {
	case types.Basic:
		if b.K == types.Int {
			return &value.Int{V: n, T: t}, true
		}
		if b.K == types.Float {
			return floatValue(float64(n), t)
		}
	case *types.LitUnionType:
		return intValue(n, b.Of)
	}
	return nil, false
}

// floatValue is a finite float of a Float type, rounded to a Float32's precision (TYP-11); one
// past Float32's range is kept whole for the re-check's E3202, as an Int8's range is (V2).
func floatValue(f float64, t types.Type) (value.Value, bool) {
	b, ok := t.Base().(types.Basic)
	if !ok || b.K != types.Float || math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, false
	}
	if r := float64(float32(f)); b.Bits == float32Bits && !math.IsInf(r, 0) {
		f = r
	}
	return &value.Float{V: f, T: t}, true
}

// strValue is a String, an asset path, or a literal of a literal union (TYP-09).
func strValue(s string, t types.Type) (value.Value, bool) {
	switch b := t.Base().(type) {
	case types.Basic:
		return &value.Str{V: s, T: t}, b.K == types.String
	case *types.LitUnionType:
		if slices.Contains(b.Literals, s) {
			return &value.Str{V: s, T: t}, true
		}
		return strValue(s, b.Of)
	}
	return nil, false
}

// memberValue is an enum member by Canon name; also the key of a ref into an enum-keyed collection.
func memberValue(name string, t types.Type) (value.Value, bool) {
	switch b := t.Base().(type) {
	case *types.EnumType:
		if i := memberIndex(b, name); i >= 0 {
			return &value.Member{Enum: b, Index: i}, true
		}
	case *types.LitUnionType:
		return memberValue(name, b.Of)
	case *types.RefType:
		if e, ok := refKeyType(b).Base().(*types.EnumType); ok && memberIndex(e, name) >= 0 {
			return &value.Ref{T: t, Key: value.Key{S: name}}, true
		}
	}
	return nil, false
}

// memberIndex is the index of the member name in e, -1 when e has none.
func memberIndex(e *types.EnumType, name string) int {
	for i, m := range e.Members {
		if m.Name == name {
			return i
		}
	}
	return -1
}

// refValue is a ref of type t whose key keyOf reads as a key of the target's key type.
func refValue(t types.Type, keyOf func(types.Type) value.Value) (value.Value, bool) {
	switch b := t.Base().(type) {
	case *types.RefType:
		k := keyOf(refKeyType(b))
		if k == nil {
			return nil, false
		}
		return &value.Ref{T: t, Key: keyOfValue(k)}, true
	case *types.LitUnionType:
		return refValue(b.Of, keyOf)
	}
	return nil, false
}

// refKeyType is the type of the keys of a ref's target: a table's are Strings, a keyed list's
// its key field's (RES-09).
func refKeyType(r *types.RefType) types.Type {
	if r.Target != nil && r.Target.KeyedBy != nil {
		return r.Target.KeyedBy.Type
	}
	return types.StringType
}

// keyOfValue is a key value as the key a ref holds: a member by its Canon name.
func keyOfValue(k value.Value) value.Key {
	switch x := k.(type) {
	case *value.Int:
		return value.Key{I: x.V, IsInt: true}
	case *value.Ref:
		return x.Key
	}
	return value.Key{S: k.CanonText()}
}

// isOptional reports t is T?.
func isOptional(t types.Type) bool {
	_, ok := t.Base().(*types.OptionalType)
	return ok
}

// present is t with its optional layer peeled: what a present value of t is.
func present(t types.Type) types.Type {
	if o, ok := t.Base().(*types.OptionalType); ok {
		return o.Elem
	}
	return t
}

// dependent reports a type that verification computes from a value (SPEC §5.11).
func dependent(t types.Type) bool {
	k := t.Base().Kind()
	return k == types.TypeApp || k == types.DepUnion
}

// symbol is a name given to a dependent type: kept as a symbol, resolved at verification (DEP-02).
func (tc *typing) symbol(lit Lit, t types.Type) (value.Value, error) {
	switch x := lit.(type) {
	case Member:
		return &value.Symbol{Name: string(x), T: t}, nil
	case Key:
		return &value.Symbol{Name: string(x), T: t}, nil
	}
	return nil, tc.mismatch(lit, t, detailDependent)
}
