package value

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/types"
)

// Bool is a Bool value.
type Bool struct {
	V bool
	P *Prov
}

func (v *Bool) Type() types.Type { return types.BoolType }

func (v *Bool) Prov() *Prov { return v.P }

func (v *Bool) CanonText() string { return strconv.FormatBool(v.V) }

func (v *Bool) equal(o Value) bool {
	w, ok := o.(*Bool)
	return ok && v.V == w.V
}

// Int is a value of any integer type (TYP-03); T is the type it was stored as.
type Int struct {
	V int64
	T types.Type
	P *Prov
}

func (v *Int) Type() types.Type { return v.T }

func (v *Int) Prov() *Prov { return v.P }

func (v *Int) CanonText() string { return strconv.FormatInt(v.V, 10) }

func (v *Int) equal(o Value) bool {
	w, ok := o.(*Int)
	return ok && v.V == w.V
}

// Float is a Float or Float32 value, always finite; a Float32 is already rounded (TYP-11).
type Float struct {
	V float64
	T types.Type
	P *Prov
}

func (v *Float) Type() types.Type { return v.T }

func (v *Float) Prov() *Prov { return v.P }

func (v *Float) CanonText() string { return types.FloatText(v.V, floatBits(v.T)) }

func (v *Float) equal(o Value) bool {
	w, ok := o.(*Float)
	return ok && v.V == w.V
}

// floatBits is the width of a Float type: 32 for Float32, 64 otherwise.
func floatBits(t types.Type) int {
	if b, ok := t.Base().(types.Basic); ok && b.Bits != 0 {
		return b.Bits
	}
	return types.FloatType.Bits
}

// Str is a String value: a plain string, an asset path, or a literal of a literal union.
type Str struct {
	V string
	T types.Type
	P *Prov
}

func (v *Str) Type() types.Type { return v.T }

func (v *Str) Prov() *Prov { return v.P }

func (v *Str) CanonText() string { return v.V }

func (v *Str) equal(o Value) bool {
	w, ok := o.(*Str)
	return ok && v.V == w.V
}

// Dur is a Duration value, in milliseconds (EVL-10).
type Dur struct {
	Ms int64
	P  *Prov
}

func (v *Dur) Type() types.Type { return types.DurationType }

func (v *Dur) Prov() *Prov { return v.P }

func (v *Dur) CanonText() string { return types.DurationText(v.Ms) }

func (v *Dur) equal(o Value) bool {
	w, ok := o.(*Dur)
	return ok && v.Ms == w.Ms
}

// Member is an enum member; Index counts retired members too.
type Member struct {
	Enum  *types.EnumType
	Index int
	P     *Prov
}

func (v *Member) Type() types.Type { return v.Enum }

func (v *Member) Prov() *Prov { return v.P }

func (v *Member) CanonText() string { return v.Enum.Members[v.Index].Name }

func (v *Member) equal(o Value) bool {
	w, ok := o.(*Member)
	return ok && v.Enum == w.Enum && v.Index == w.Index
}

// CaseKind is a value of v.kind: one case of a variant, as a member of Kind(V).
type CaseKind struct {
	T     *types.VariantKindType
	Index int
	P     *Prov
}

func (v *CaseKind) Type() types.Type { return v.T }

func (v *CaseKind) Prov() *Prov { return v.P }

func (v *CaseKind) CanonText() string { return v.T.Variant.Cases[v.Index].Name }

func (v *CaseKind) equal(o Value) bool {
	w, ok := o.(*CaseKind)
	return ok && v.T.Variant == w.T.Variant && v.Index == w.Index
}

// Symbol is a bare identifier given to a dependent field, resolved against the branch at
// verification (DEP-02); T is the field's dependent type.
type Symbol struct {
	Name string
	T    types.Type
	P    *Prov
}

func (v *Symbol) Type() types.Type { return v.T }

func (v *Symbol) Prov() *Prov { return v.P }

func (v *Symbol) CanonText() string { return v.Name }

func (v *Symbol) equal(o Value) bool {
	w, ok := o.(*Symbol)
	return ok && v.Name == w.Name
}

// None is `none`; T is the optional type it is none of, or types.NoneType before context.
type None struct {
	T types.Type
	P *Prov
}

func (v *None) Type() types.Type { return v.T }

func (v *None) Prov() *Prov { return v.P }

func (v *None) CanonText() string { return textNone }

func (v *None) equal(o Value) bool {
	_, ok := o.(*None)
	return ok
}

// Range is an integer range value (TYP-12): End is exclusive, and absent when open.
type Range struct {
	Start, End int64
	HasEnd     bool
	P          *Prov
}

func (v *Range) Type() types.Type { return types.RangeType }

func (v *Range) Prov() *Prov { return v.P }

func (v *Range) CanonText() string {
	s := strconv.FormatInt(v.Start, 10) + textRange
	if v.HasEnd {
		s += strconv.FormatInt(v.End, 10)
	}
	return s
}

func (v *Range) equal(o Value) bool {
	w, ok := o.(*Range)
	return ok && v.Start == w.Start && v.HasEnd == w.HasEnd && (!v.HasEnd || v.End == w.End)
}
