package edit

import (
	"strconv"
	"time"

	"github.com/fantasim/canonlang/internal/types"
)

// Lit is a value an operation carries; the set of forms is closed (API.md §8.2).
type Lit interface {
	isLit()
}

type (
	// Bool is `true` or `false`.
	Bool bool
	// Int is an integer, accepted by every integer type and by Float.
	Int int64
	// Float is a Float or Float32 value; NaN and the infinities fit no type.
	Float float64
	// Str is a String, an asset path or a literal of a literal union.
	Str string
	// Dur is a Duration, a whole number of milliseconds.
	Dur time.Duration
	// Member is an enum member by Canon name.
	Member string
	// Key is a String key: of a ref, a table entry or a String map key.
	Key string
	// IntKey is an integer key: of a ref to an integer-keyed collection, or of a map.
	IntKey int64
	// PathKey is a key as an edit's JSON form gives it, read as a path key (API.md E25).
	PathKey string
	// None is `none`.
	None struct{}
	// List is a list, plain or keyed.
	List []Lit
	// Obj is a record, a case's fields or a table entry: fields by Canon name (API.md V3).
	Obj map[string]Lit
	// Map is a map, in the given order.
	Map []KV
	// FromJSON is a value in its wire form (WIRE.md), decoded as the expected type.
	FromJSON []byte
	// Source is a Canon literal, typed as the expected type.
	Source string
)

// Case is a variant case by name, with its fields; Fields may be nil.
type Case struct {
	Name   string
	Fields Obj
}

// KV is one entry of a Map.
type KV struct {
	Key   Lit
	Value Lit
}

func (Bool) isLit()     {}
func (Int) isLit()      {}
func (Float) isLit()    {}
func (Str) isLit()      {}
func (Dur) isLit()      {}
func (Member) isLit()   {}
func (Key) isLit()      {}
func (IntKey) isLit()   {}
func (PathKey) isLit()  {}
func (None) isLit()     {}
func (List) isLit()     {}
func (Obj) isLit()      {}
func (Map) isLit()      {}
func (FromJSON) isLit() {}
func (Source) isLit()   {}
func (Case) isLit()     {}

// describe is what a ValueError says was given: the constructor, with a scalar's argument.
func describe(l Lit) string {
	switch x := l.(type) {
	case Bool:
		return call(nameBool, strconv.FormatBool(bool(x)))
	case Int:
		return call(nameInt, strconv.FormatInt(int64(x), decimalBase))
	case Float:
		return call(nameFloat, strconv.FormatFloat(float64(x), floatFormat, shortestPrec, int64Bits))
	case Str:
		return call(nameStr, types.QuoteString(string(x)))
	case Dur:
		return call(nameDur, time.Duration(x).String())
	case Member:
		return call(nameMember, string(x))
	case Key:
		return call(nameKey, types.QuoteString(string(x)))
	case IntKey:
		return call(nameIntKey, strconv.FormatInt(int64(x), decimalBase))
	case PathKey:
		return call(namePathKey, types.QuoteString(string(x)))
	case Case:
		return call(nameCase, x.Name)
	}
	return litNames[litForm(l)]
}

func call(name, arg string) string {
	return name + callOpen + arg + callClose
}

// litForm is the form of a composite, None, FromJSON or Source Lit; formOther for a scalar.
func litForm(l Lit) form {
	switch l.(type) {
	case None:
		return formNone
	case List:
		return formList
	case Obj:
		return formObj
	case Map:
		return formMap
	case FromJSON:
		return formJSON
	case Source:
		return formSource
	}
	return formOther
}
