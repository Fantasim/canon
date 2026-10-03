package wire_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

var (
	tone    = enumWires("teamboard", "Tone", "info", "info", "series_1", "series-1")
	element = func() *types.EnumType {
		e := codes(enum("fpdemo", "Element", "FIRE", "WATER", "WIND"), 1, 2, 3)
		e.WireCodes, e.Members[2].Retired = true, true
		return e
	}()
	dupCodes = func() *types.EnumType {
		e := codes(enum("p", "Dup", "A", "B", "C"), 1, 1, 3)
		e.WireCodes = true
		return e
	}()
	flag   = codes(enum("fpdemo", "Flag", "tradable", "droppable", "soulbound"), 1, 2, 4)
	side   = enumWires("fpdemo", "Side", "left", "left", "right", "RIGHT")
	status = record("flow", "Status", field("label", types.StringType))
	flows  = &types.Collection{Kind: types.CollLet, Pkg: "flow", Name: "statuses", Elem: status}
)

// enumWires is an enum of (name, wire) pairs.
func enumWires(pkg, name string, pairs ...string) *types.EnumType {
	e := &types.EnumType{Pkg: pkg, Name: name}
	for i := 0; i < len(pairs); i += 2 {
		e.Members = append(e.Members, &types.Member{Name: pairs[i], Wire: pairs[i+1], Index: i / 2})
	}
	return e
}

// inField wraps f in a one-field record, so that f's unit and encoding apply.
func inField(f *types.Field) *types.RecordType { return record("p", "R", f) }

func withUnit(t types.Type, u types.Unit) *types.RecordType {
	f := field("x", t)
	f.Unit = u
	return inField(f)
}

func withEnc(t types.Type, e types.Enc) *types.RecordType {
	f := field("x", t)
	f.Enc = e
	return inField(f)
}

var scalarCases = []struct {
	rule string
	typ  types.Type
	json string
	want string
}{
	{"§5.1 Bool", types.BoolType, `true`, "true"},
	{"§5.1 Bool kind", types.BoolType, `1`, at(diag.E7110, "1:1", "")},
	{"§5.1 Int", types.IntType, `42`, "42"},
	{"§5.1 Int -0", types.IntType, `-0`, "0"},
	{"§5.1 Int fraction", types.IntType, `2.0`, at(diag.E7103, "1:1", "")},
	{"§5.1 Int exponent", types.IntType, `1e3`, at(diag.E7103, "1:1", "")},
	{"§5.1 Int negative fraction", types.IntType, `-1.5`, at(diag.E7103, "1:1", "")},
	{"§5.1 Int overflow", types.IntType, `9223372036854775808`, at(diag.E3201, "1:1", "")},
	{"§5.1 Int min", types.IntType, `-9223372036854775808`, "-9223372036854775808"},
	{"§5.1 UInt8 range", types.UInt8Type, `256`, at(diag.E3201, "1:1", "")},
	{"§5.1 Int8 range", types.Int8Type, `-129`, at(diag.E3201, "1:1", "")},
	{"§5.1 UInt64 range", types.UInt64Type, `-1`, at(diag.E3201, "1:1", "")},
	{"§5.1 Int string", types.IntType, `"5"`, at(diag.E7110, "1:1", "")},
	{"§5.1 Float", types.FloatType, `0.1`, "0.1"},
	{"§5.1 Float -0", types.FloatType, `-0.0`, "0"},
	{"§5.1 Float overflow", types.FloatType, `1e400`, at(diag.E3202, "1:1", "")},
	{"§5.1 Float underflow", types.FloatType, `1e-400`, "0"},
	{"§5.1 Float ties to even", types.FloatType, `9007199254740993`, "9007199254740992"},
	{"§5.1 Float32 once", types.Float32Type, `0.1`, "0.1"},
	{"§5.1 Float32 overflow", types.Float32Type, `3.5e38`, at(diag.E3202, "1:1", "")},
	{"§5.1 String", types.StringType, `"aé"`, "aé"},
	{"§5.1 String kind", types.StringType, `5`, at(diag.E7110, "1:1", "")},
	{"§5.1 Duration ms", withUnit(types.DurationType, types.UnitMs), `{"x": 8000}`, "R{x: 8s}"},
	{"§5.1 Duration 8000.0", withUnit(types.DurationType, types.UnitMs), `{"x": 8000.0}`, "R{x: 8s}"},
	{"§5.1 Duration 8e3", withUnit(types.DurationType, types.UnitMs), `{"x": 8e3}`, "R{x: 8s}"},
	{"§5.1 Duration 1500.5 ms", withUnit(types.DurationType, types.UnitMs), `{"x": 1500.5}`, at(diag.E3203, "1:7", "/x")},
	{"§5.1 Duration 1.5 m", withUnit(types.DurationType, types.UnitM), `{"x": 1.5}`, "R{x: 1m30s}"},
	{"§5.1 Duration 0.0005 s", withUnit(types.DurationType, types.UnitS), `{"x": 0.0005}`, at(diag.E3203, "1:7", "/x")},
	{"§5.1 Duration limit", withUnit(types.DurationType, types.UnitMs), `{"x": -9223372036854}`, "R{x: -106751d23h47m16s854ms}"},
	{"§5.1 Duration range", withUnit(types.DurationType, types.UnitMs), `{"x": 9223372036855}`, at(diag.E3201, "1:7", "/x")},
	{"§5.1 Duration days", withUnit(types.DurationType, types.UnitD), `{"x": 1e-8}`, at(diag.E3203, "1:7", "/x")},
	{"§5.1 Duration huge exponent", withUnit(types.DurationType, types.UnitMs), `{"x": 1e999999999999999999}`, at(diag.E3201, "1:7", "/x")},
	{"§5.1 Duration tiny exponent", withUnit(types.DurationType, types.UnitMs), `{"x": 1e-999999999999999999}`, at(diag.E3203, "1:7", "/x")},
	{"§5.1 Duration zero exponent", withUnit(types.DurationType, types.UnitH), `{"x": 0e-99999}`, "R{x: 0s}"},
	{"§5.1 Duration in a list", withUnit(listOf(types.DurationType), types.UnitS), `{"x": [1, 0.25]}`, "R{x: [1s, 250ms]}"},
	{"§5.2 int 0", withEnc(types.BoolType, types.EncInt), `{"x": 0}`, "R{x: false}"},
	{"§5.2 int 1", withEnc(types.BoolType, types.EncInt), `{"x": 1}`, "R{x: true}"},
	{"§5.2 int true", withEnc(types.BoolType, types.EncInt), `{"x": true}`, at(diag.E7110, "1:7", "/x")},
	{"§5.2 int 2", withEnc(types.BoolType, types.EncInt), `{"x": 2}`, at(diag.E7110, "1:7", "/x")},
	{"§5.2 int 1.0", withEnc(types.BoolType, types.EncInt), `{"x": 1.0}`, at(diag.E7110, "1:7", "/x")},
	{"§5.2 int array", withEnc(types.BoolType, types.EncInt), `{"x": []}`, at(diag.E7110, "1:7", "/x")},
	{"§5.3 enum wire", tone, `"series-1"`, "series_1"},
	{"§5.3 enum Canon name", tone, `"series_1"`, at(diag.E7111, "1:1", "")},
	{"§5.3 enum unknown", tone, `"zzz"`, at(diag.E7111, "1:1", "")},
	{"§5.3 enum kind", tone, `1`, at(diag.E7110, "1:1", "")},
	{"§5.3 codes", element, `1`, "FIRE"},
	{"§5.3 codes retired", element, `3`, "WIND"},
	{"§5.3 codes unknown", element, `4`, at(diag.E7111, "1:1", "")},
	{"§9.1 broken codes add no second finding", dupCodes, `2`, ""},
	{"§9.1 broken codes add no second finding in bits", withEnc(listOf(dupCodes), types.EncBits), `{"x": 8}`, ""},
	{"§5.3 codes fraction", element, `1.0`, at(diag.E7103, "1:1", "")},
	{"§5.3 codes kind", element, `"FIRE"`, at(diag.E7110, "1:1", "")},
	{"§5.3 bits", withEnc(listOf(flag), types.EncBits), `{"x": 5}`, "R{x: [tradable, soulbound]}"},
	{"§5.3 bits empty", withEnc(listOf(flag), types.EncBits), `{"x": 0}`, "R{x: []}"},
	{"§5.3 bits optional", withEnc(opt(listOf(flag)), types.EncBits), `{"x": 2}`, "R{x: [droppable]}"},
	{"§5.3 bits negative", withEnc(listOf(flag), types.EncBits), `{"x": -1}`, at(diag.E7110, "1:7", "/x")},
	{"§5.3 bits unknown", withEnc(listOf(flag), types.EncBits), `{"x": 13}`, at(diag.E7111, "1:7", "/x")},
	{"§5.3 bits past Int", withEnc(listOf(flag), types.EncBits), `{"x": 18446744073709551615}`, at(diag.E7111, "1:7", "/x")},
	{"§5.3 bits high", withEnc(listOf(flag), types.EncBits), `{"x": 9223372036854775807}`, at(diag.E7111, "1:7", "/x")},
	{"§5.3 bits exponent", withEnc(listOf(flag), types.EncBits), `{"x": 1e3}`, at(diag.E7103, "1:7", "/x")},
	{"§5.3 bits fraction", withEnc(listOf(flag), types.EncBits), `{"x": 1.5}`, at(diag.E7103, "1:7", "/x")},
	{"§5.3 bits kind", withEnc(listOf(flag), types.EncBits), `{"x": "5"}`, at(diag.E7110, "1:7", "/x")},
	{"§5.9 union literal", &types.LitUnionType{Of: side, Literals: []string{"both"}}, `"both"`, "both"},
	{"§5.9 union base", &types.LitUnionType{Of: side, Literals: []string{"both"}}, `"RIGHT"`, "right"},
	{"§5.9 literal wins", &types.LitUnionType{Of: types.StringType, Literals: []string{"default"}}, `"default"`, "default"},
	{"§5.9 table ref", &types.RefType{Target: flows}, `"open"`, "open"},
	{"§5.9 table ref kind", &types.RefType{Target: flows}, `5`, at(diag.E7110, "1:1", "")},
	{"§5.9 keyed ref", &types.RefType{Target: intKeyed}, `7`, "7"},
	{"§5.9 keyed ref kind", &types.RefType{Target: intKeyed}, `"7"`, at(diag.E7110, "1:1", "")},
	{"§5.9 enum-keyed ref", &types.RefType{Target: toneKeyed}, `"series-1"`, "series_1"},
	{"§5.4 optional null", opt(types.IntType), `null`, "none"},
	{"§5.4 null elements", listOf(opt(types.IntType)), `[1, null]`, "[1, none]"},
	{"§5.4 null element", listOf(types.IntType), `[1, null]`, at(diag.E3315, "1:5", "/1")},
	{"§5.4 null top level", types.IntType, `null`, at(diag.E3315, "1:1", "")},
	{"§5.9 plain Never", opt(types.NeverType), `1`, at(diag.E7110, "1:1", "")},
}

// WIRE.md §5.1, §5.3, §6.6: a huge integer is past every range without being parsed whole.
func TestDecodeHugeIntegers(t *testing.T) {
	digits := strings.Repeat("9", 4<<20)
	start := time.Now()
	got := decodeJSON(t, scalarsDec, `{"x": `+digits+`}`, withEnc(listOf(flag), types.EncBits)).text()
	if got != at(diag.E7111, "1:7", "/x") {
		t.Errorf("a 4 MB mask: %.80s", got)
	}
	row := record("p", "H", field("i", types.IntType), field("f", types.FloatType))
	hex := "0x" + strings.Repeat("F", 1<<20)
	got = decodeCSV(t, wire.Decoder{}, "i,f\n"+hex+","+hex+"\n", listOf(row)).text()
	if got != findings(at(diag.E3201, "2:1", ""), at(diag.E3202, "2:1048580", "")) {
		t.Errorf("1 MB hexadecimal cells: %.80s", got)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("huge integers took %v", elapsed)
	}
}

var (
	intNode    = record("p", "Node", field("id", types.IntType))
	intKeyed   = &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "nodes", Elem: intNode, KeyedBy: intNode.Fields[0]}
	toneRow    = record("p", "ToneRow", field("tone", tone))
	toneKeyed  = &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "tones", Elem: toneRow, KeyedBy: toneRow.Fields[0]}
	scalarsDec = wire.Decoder{}
)

// WIRE.md §5.1–§5.4, §5.9 and §7.2: each scalar reading, and the code of each refusal.
func TestDecodeScalars(t *testing.T) {
	for _, c := range scalarCases {
		if got := decodeJSON(t, scalarsDec, c.json, c.typ).text(); got != c.want {
			t.Errorf("%s: %s as %v = %q, want %q", c.rule, c.json, c.typ, got, c.want)
		}
	}
}

// WIRE.md §5.1: numbers are read from their token exactly, rounded once to their width.
func TestDecodeFloatWidths(t *testing.T) {
	got := decodeJSON(t, scalarsDec, `0.1`, types.Float32Type)
	if f, ok := got.v.(*value.Float); !ok || f.V != float64(float32(0.1)) || f.T != types.Float32Type {
		t.Errorf("Float32 0.1 = %#v", got.v)
	}
	got = decodeJSON(t, scalarsDec, `-0.0`, types.FloatType)
	if f := got.v.(*value.Float); math.Signbit(f.V) {
		t.Errorf("-0.0 reads as -0")
	}
	// 1.00000005960464477550 lies just above a float32 midpoint: double rounding through
	// float64 would land on 1.0.
	got = decodeJSON(t, scalarsDec, `1.00000005960464477550`, types.Float32Type)
	if f := got.v.(*value.Float); float32(f.V) != math.Nextafter32(1, 2) {
		t.Errorf("rounded twice: %v", f.V)
	}
}
