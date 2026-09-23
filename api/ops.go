package canon

import (
	"encoding/json"
	"time"
)

// Lit is a value given to an edit operation; the set is closed (API.md §8.2).
type Lit interface {
	isLit()
}

type (
	boolLit   struct{ v bool }
	intLit    struct{ v int64 }
	floatLit  struct{ v float64 }
	strLit    struct{ v string }
	durLit    struct{ v time.Duration }
	memberLit struct{ name string }
	keyLit    struct{ key string }
	intKeyLit struct{ key int64 }
	caseLit   struct {
		name   string
		fields Obj
	}
	noneLit   struct{}
	listLit   struct{ elems []Lit }
	mapLit    struct{ entries []KV }
	jsonLit   struct{ raw json.RawMessage }
	sourceLit struct{ text string }
)

func (boolLit) isLit()   {}
func (intLit) isLit()    {}
func (floatLit) isLit()  {}
func (strLit) isLit()    {}
func (durLit) isLit()    {}
func (memberLit) isLit() {}
func (keyLit) isLit()    {}
func (intKeyLit) isLit() {}
func (caseLit) isLit()   {}
func (noneLit) isLit()   {}
func (listLit) isLit()   {}
func (mapLit) isLit()    {}
func (jsonLit) isLit()   {}
func (sourceLit) isLit() {}
func (Obj) isLit()       {}

// Obj is a record, case or table entry value: fields by Canon name, in any order (rule V3).
type Obj map[string]Lit

// KV is one entry of a map value.
type KV struct {
	Key   Lit
	Value Lit
}

// Bool is a Bool value.
func Bool(b bool) Lit {
	return boolLit{b}
}

// Int is an integer value, accepted by every integer type and by Float.
func Int(n int64) Lit {
	return intLit{n}
}

// Float is a Float or Float32 value. NaN and infinities are refused (E3202).
func Float(f float64) Lit {
	return floatLit{f}
}

// Str is a String value; also accepted by asset types and string literal unions.
func Str(s string) Lit {
	return strLit{s}
}

// Dur is a Duration value. It must be a whole number of milliseconds.
func Dur(d time.Duration) Lit {
	return durLit{d}
}

// Member is an enum member, by Canon name (not wire value).
func Member(name string) Lit {
	return memberLit{name}
}

// Key is a string key: the target of a ref, a table key, or a String map key.
func Key(k string) Lit {
	return keyLit{k}
}

// IntKey is an integer key: of a ref to an integer-keyed collection, or of a map.
func IntKey(n int64) Lit {
	return intKeyLit{n}
}

// Case is a variant case with its fields; fields may be nil for a case without fields.
func Case(name string, fields Obj) Lit {
	return caseLit{name, fields}
}

// List is a list value, for [T] and keyed lists.
func List(elems ...Lit) Lit {
	return listLit{elems}
}

// Map is a map value; entries keep the given order.
func Map(entries ...KV) Lit {
	return mapLit{entries}
}

// FromJSON is a value in its wire form (WIRE.md), decoded against the expected type.
func FromJSON(raw []byte) Lit {
	return jsonLit{json.RawMessage(raw)}
}

// Source is a value written as a Canon literal, typed against the expected type.
func Source(text string) Lit {
	return sourceLit{text}
}

// Op is one operation of an Edit, built by its constructor (API.md §8.3).
type Op struct {
	Kind  OpKind
	Path  string
	Value Lit // Set, Add, Insert, AddEntry; SetCase fields (an Obj, or nil)
	Key   Lit // AddEntry key; Rename new key
	Index int // Insert, Move
	Case  string
}

// Set replaces the value at path (rules E5-E7).
func Set(path string, v Lit) Op {
	return Op{Kind: OpSet, Path: path, Value: v}
}

// Reset removes the field at path from its literal or JSON object, so it takes its default.
func Reset(path string) Op {
	return Op{Kind: OpReset, Path: path}
}

// Add appends v to the list or keyed list at path.
func Add(path string, v Lit) Op {
	return Op{Kind: OpAdd, Path: path, Value: v}
}

// Insert inserts v at position index of the list or keyed list at path.
func Insert(path string, index int, v Lit) Op {
	return Op{Kind: OpInsert, Path: path, Index: index, Value: v}
}

// AddEntry adds the entry key with value v to the table or map at path.
func AddEntry(path string, key, v Lit) Op {
	return Op{Kind: OpAddEntry, Path: path, Key: key, Value: v}
}

// Remove removes the list element, map entry or non-stable table entry at path.
func Remove(path string) Op {
	return Op{Kind: OpRemove, Path: path}
}

// Move moves the element or entry at path to position index among its siblings.
func Move(path string, index int) Op {
	return Op{Kind: OpMove, Path: path, Index: index}
}

// Rename changes the key at path and every reference to it (rules E11-E13).
func Rename(path string, newKey Lit) Op {
	return Op{Kind: OpRename, Path: path, Key: newKey}
}

// Retire marks the stable entry or @codes member at path as retired (SPEC §12).
func Retire(path string) Op {
	return Op{Kind: OpRetire, Path: path}
}

// Unretire always fails with ErrStableKey (rule E4); it exists so the refusal is explicit.
func Unretire(path string) Op {
	return Op{Kind: OpUnretire, Path: path}
}

// SetCase changes the case of the variant at path, applying fields on top (rule E14).
func SetCase(path, caseName string, fields Obj) Op {
	op := Op{Kind: OpSetCase, Path: path, Case: caseName}
	if fields != nil {
		op.Value = fields
	}
	return op
}

// MarshalJSON writes the JSON form of rules E24-E26.
func (o Op) MarshalJSON() ([]byte, error) {
	return nil, errUnimplemented()
}

// UnmarshalJSON reads the JSON form of rules E24-E26.
func (o *Op) UnmarshalJSON(data []byte) error {
	return errUnimplemented()
}
