package value_test

import (
	"math"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

func pairs(kvs ...value.Value) *value.Map {
	m := &value.Map{T: &types.MapType{Key: types.StringType, Value: types.IntType}}
	for i := 0; i < len(kvs); i += 2 {
		m.Keys = append(m.Keys, kvs[i])
		m.Vals = append(m.Vals, kvs[i+1])
	}
	return m
}

func list(vs ...value.Value) *value.List {
	return &value.List{T: &types.ListType{Elem: types.IntType}, Elems: vs}
}

func table(es ...*value.Record) *value.Table {
	return &value.Table{T: &types.TableType{Elem: status}, Entries: es}
}

var equalCases = []struct {
	name string
	a, b value.Value
	want bool
}{
	{"bool", &value.Bool{V: true}, &value.Bool{V: true}, true},
	{"int across widths", &value.Int{V: 3, T: types.UInt8Type}, num(3), true},
	{"int differs", num(3), num(4), false},
	{"float -0 == 0", &value.Float{V: math.Copysign(0, -1), T: types.FloatType}, &value.Float{V: 0, T: types.FloatType}, true},
	{"float differs", &value.Float{V: 0.5, T: types.FloatType}, &value.Float{V: 0.25, T: types.FloatType}, false},
	{"string bytes", str("é"), str("é"), true},
	{"duration", &value.Dur{Ms: 1500}, &value.Dur{Ms: 1500}, true},
	{"enum member", &value.Member{Enum: tone, Index: 1}, &value.Member{Enum: tone, Index: 1}, true},
	{"enum members differ", &value.Member{Enum: tone}, &value.Member{Enum: tone, Index: 1}, false},
	{"case kind", &value.CaseKind{T: &types.VariantKindType{Variant: reward}}, &value.CaseKind{T: &types.VariantKindType{Variant: reward}}, true},
	{"none equals none", &value.None{T: types.NoneType}, &value.None{T: &types.OptionalType{Elem: types.IntType}}, true},
	{"none equals only none", &value.None{T: types.NoneType}, num(0), false},
	{"ranges", &value.Range{Start: 1, End: 5, HasEnd: true}, &value.Range{Start: 1, End: 5, HasEnd: true}, true},
	{"open ranges", &value.Range{Start: 1, End: 9}, &value.Range{Start: 1}, true},
	{"ranges differ", &value.Range{Start: 1, End: 5, HasEnd: true}, &value.Range{Start: 1}, false},
	{"records field-wise", clock(8, 30), clock(8, 30), true},
	{"records differ", clock(8, 30), clock(9, 30), false},
	{"cases differ", &value.Record{T: nothing}, &value.Record{T: item, Fields: []value.Value{num(1), num(1)}}, false},
	{"applied records erase arguments", &value.Record{T: &types.AppliedRecord{Rec: hourly}}, &value.Record{T: hourly}, true},
	// TYPES.md §7.5 non-transitivity gap (a Louis-call): equal by identity, hashed by structure.
	{"entries by identity", entry("open", "Open"), entry("open", "Changed"), true},
	{"entries by key", entry("open", "Open"), entry("taken", "Open"), false},
	{"entry ignores identity against a plain record", entry("open", "Open"), &value.Record{T: status, Fields: []value.Value{str("Open")}}, true},
	// TYPES.md §7.5 non-transitivity gap (a Louis-call): a ref hashes by identity, an entry by structure.
	{"ref and entry", &value.Ref{T: refStatus, Key: value.Key{S: "open"}}, entry("open", "Open"), true},
	{"refs", &value.Ref{T: &types.Refined{Of: refStatus}, Key: value.Key{S: "open"}}, &value.Ref{T: refStatus, Key: value.Key{S: "open"}}, true},
	{"refs differ", &value.Ref{T: refStatus, Key: value.Key{S: "open"}}, &value.Ref{T: refStatus, Key: value.Key{S: "done"}}, false},
	{"lists in order", list(num(1), num(2)), list(num(1), num(2)), true},
	{"lists differ in order", list(num(1), num(2)), list(num(2), num(1)), false},
	{"lists differ in length", list(num(1)), list(num(1), num(2)), false},
	{"maps ignore order", pairs(str("a"), num(1), str("b"), num(2)), pairs(str("b"), num(2), str("a"), num(1)), true},
	{"maps differ in a value", pairs(str("a"), num(1)), pairs(str("a"), num(2)), false},
	{"maps differ in keys", pairs(str("a"), num(1)), pairs(str("b"), num(1)), false},
	{"maps differ in size", pairs(str("a"), num(1)), pairs(), false},
	{"tables keep order", table(entry("open", "O"), entry("done", "D")), table(entry("open", "O"), entry("done", "D")), true},
	{"tables differ in order", table(entry("open", "O"), entry("done", "D")), table(entry("done", "D"), entry("open", "O")), false},
	{"tables differ in size", table(entry("open", "O")), table(), false},
	{"symbols by name", &value.Symbol{Name: "monster"}, &value.Symbol{Name: "monster"}, true},
	{"a symbol is not a string", &value.Symbol{Name: "pvp"}, str("pvp"), false},
	{"pairs", &value.Pair{A: num(0), B: str("x")}, &value.Pair{A: num(0), B: str("x")}, true},
	{"kinds never mix", num(1), &value.Float{V: 1, T: types.FloatType}, false},
}

// TYPES.md §7.5: equality at evaluation.
func TestEqual(t *testing.T) {
	for _, c := range equalCases {
		if got := value.Equal(c.a, c.b); got != c.want {
			t.Errorf("%s: Equal = %v, want %v", c.name, got, c.want)
		}
		if got := value.Equal(c.b, c.a); got != c.want {
			t.Errorf("%s: Equal is not symmetric", c.name)
		}
	}
}

// TYPES.md §9.2: a map reads an entry by value equality of its key.
func TestMapGet(t *testing.T) {
	m := pairs(str("a"), num(1))
	if v, ok := m.Get(str("a")); !ok || !value.Equal(v, num(1)) {
		t.Errorf("Get(a) = %v, %v", v, ok)
	}
	if _, ok := m.Get(str("z")); ok {
		t.Errorf("Get(z) found a missing key")
	}
}

// A value type of another package (the evaluator's function values) is equal only to itself.
func TestEqualForeign(t *testing.T) {
	f := foreign{}
	if !value.Equal(f, f) || value.Equal(f, num(1)) {
		t.Errorf("a foreign value is equal only to itself")
	}
}

type foreign struct{}

func (foreign) Type() types.Type { return &types.FuncType{Result: types.IntType} }

func (foreign) Prov() *value.Prov { return nil }

func (foreign) CanonText() string { return "" }
