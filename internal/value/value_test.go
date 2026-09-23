package value_test

import (
	"math"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

var (
	lit       = &value.Prov{Kind: value.ProvLiteral, Span: source.Span{File: 1, Start: 4, End: 9}}
	tone      = &types.EnumType{Pkg: "teamboard", Name: "Tone", Members: []*types.Member{{Name: "info", Wire: "info"}, {Name: "series_1", Wire: "series-1", Index: 1}}}
	reward    = variant(&types.CaseType{Name: "nothing"}, &types.CaseType{Name: "item", Index: 1, Fields: []*types.Field{{Name: "define"}, {Name: "count", Index: 1}}})
	nothing   = reward.Cases[0]
	item      = reward.Cases[1]
	timeOfDay = &types.RecordType{Pkg: "sovcommon.time", Name: "TimeOfDay", Fields: []*types.Field{{Name: "hour"}, {Name: "minute", Index: 1}, {Name: "apiKey", Index: 2, Input: &types.Input{Env: "KEY"}}}}
	status    = &types.RecordType{Pkg: "teamboard", Name: "Status", Fields: []*types.Field{{Name: "label"}}}
	statuses  = &types.Collection{Kind: types.CollLet, Pkg: "teamboard", Name: "statuses", Elem: status}
	refStatus = &types.RefType{Target: statuses}
	hourly    = &types.RecordType{Pkg: "events", Name: "HourlyTarget", Params: []*types.Param{{Name: "e"}}}
)

func variant(cases ...*types.CaseType) *types.VariantType {
	v := &types.VariantType{Pkg: "events", Name: "Reward", Cases: cases}
	for _, c := range cases {
		c.Variant = v
	}
	return v
}

func str(s string) *value.Str { return &value.Str{V: s, T: types.StringType, P: lit} }

func num(n int64) *value.Int { return &value.Int{V: n, T: types.IntType} }

func entry(key, label string) *value.Record {
	return &value.Record{T: status, Fields: []value.Value{str(label)}, Ident: &value.Identity{Coll: statuses, Key: value.Key{S: key}}}
}

func clock(h, m int64) *value.Record {
	return &value.Record{T: timeOfDay, Fields: []value.Value{num(h), num(m), nil}, Set: []bool{true, true, false}}
}

var textCases = []struct {
	v    value.Value
	kind types.Kind
	text string
}{
	{&value.Bool{V: true}, types.Bool, "true"},
	{&value.Int{V: -42, T: types.Int16Type}, types.Int, "-42"},
	{&value.Float{V: 0.1, T: types.FloatType}, types.Float, "0.1"},
	{&value.Float{V: float64(float32(0.1)), T: types.Float32Type}, types.Float, "0.1"},
	{&value.Float{V: 1e21, T: &types.Refined{Of: types.FloatType}}, types.Float, "1e+21"},
	{str("Heal"), types.String, "Heal"},
	{&value.Dur{Ms: 5_400_000}, types.Duration, "1h30m"},
	{&value.Member{Enum: tone, Index: 1}, types.Enum, "series_1"},
	{&value.CaseKind{T: &types.VariantKindType{Variant: reward}, Index: 1}, types.VariantKind, "item"},
	{&value.None{T: &types.OptionalType{Elem: types.IntType}}, types.Optional, "none"},
	{&value.Symbol{Name: "II_GEN_MAT_MOONSTONE", T: &types.TypeAppType{Fn: &types.TypeFunc{Name: "Param"}}}, types.TypeApp, "II_GEN_MAT_MOONSTONE"},
	{&value.Range{Start: 0, End: 11, HasEnd: true}, types.Range, "0..11"},
	{&value.Range{Start: 3}, types.Range, "3.."},
	{&value.Ref{T: refStatus, Key: value.Key{S: "open"}}, types.Ref, "open"},
	{&value.Ref{T: refStatus, Key: value.Key{I: 3, IsInt: true}}, types.Ref, "3"},
	{&value.List{T: &types.ListType{Elem: types.StringType}, Elems: []value.Value{str("a\"b"), num(2)}}, types.List, `["a\"b", 2]`},
	{&value.List{T: &types.ListType{Elem: types.IntType}}, types.List, "[]"},
	{&value.Map{T: &types.MapType{Key: types.StringType, Value: types.IntType}, Keys: []value.Value{str("Cap"), &value.Member{Enum: tone}}, Vals: []value.Value{num(20), str("x")}}, types.Map, `{"Cap": 20, info: "x"}`},
	{clock(8, 30), types.Record, "TimeOfDay{hour: 8, minute: 30}"},
	{&value.Record{T: nothing}, types.Case, "nothing"},
	{&value.Record{T: item, Fields: []value.Value{&value.Ref{T: refStatus, Key: value.Key{S: "II_GEN_GOLD"}}, num(1)}}, types.Case, "item{define: II_GEN_GOLD, count: 1}"},
	{&value.Record{T: types.DefineType, Fields: []value.Value{num(5)}}, types.Define, "Define{value: 5}"},
	{&value.Record{T: &types.AppliedRecord{Rec: hourly}}, types.Record, "HourlyTarget{}"},
	{&value.Table{T: &types.TableType{Elem: status}, Entries: []*value.Record{entry("open", "Open")}}, types.Table, `{open: Status{label: "Open"}}`},
	{&value.Pair{T: &types.PairType{A: types.IntType, B: refStatus}, A: num(0), B: &value.Ref{T: refStatus, Key: value.Key{S: "open"}}}, types.Pair, "(0, open)"},
}

// STD-06: the canonical text form of every kind of value, and the type each one carries.
func TestCanonText(t *testing.T) {
	for _, c := range textCases {
		if got := c.v.CanonText(); got != c.text {
			t.Errorf("CanonText() = %q, want %q", got, c.text)
		}
		if got := c.v.Type().Kind(); got != c.kind {
			t.Errorf("%s: Type().Kind() = %d, want %d", c.text, got, c.kind)
		}
		var arg diag.ValueArg = c.v
		if arg.CanonText() != c.text || c.v.Prov() != nil && c.v.Prov() != lit {
			t.Errorf("%s: a value is a diag.ValueArg and keeps its provenance", c.text)
		}
	}
}

// EVL-07: provenance carries its origin, the omitting literal of a default and a bounded stack.
func TestProv(t *testing.T) {
	def := &value.Prov{Kind: value.ProvDefault, Via: lit}
	computed := &value.Prov{Kind: value.ProvComputed, Stack: []diag.Frame{{Fn: "healFor"}}, MoreFrames: 3}
	loaded := &value.Prov{Kind: value.ProvJSON, Pointer: "/modelTypes/3"}
	kinds := []value.ProvKind{value.ProvLiteral, value.ProvJSON, value.ProvCSV, value.ProvDefines, value.ProvText, value.ProvDefault, value.ProvSpread, value.ProvComputed, value.ProvLayer}
	for i, k := range kinds {
		if int(k) != i {
			t.Errorf("provenance kinds follow API.md OriginKind order")
		}
	}
	layered := &value.Prov{Kind: value.ProvLayer, Layer: "local", Via: &value.Prov{Kind: value.ProvSpread}}
	if def.Via != lit || len(computed.Stack) > diag.MaxStackFrames || loaded.Pointer == "" || layered.Layer != "local" {
		t.Errorf("provenance fields")
	}
}

// API-02: a key prints as written, an integer key in decimal.
func TestKeyText(t *testing.T) {
	if got := (value.Key{S: "open"}).Text(); got != "open" {
		t.Errorf("Key.Text() = %q", got)
	}
	if got := (value.Key{I: math.MinInt64, IsInt: true}).Text(); got != "-9223372036854775808" {
		t.Errorf("Key.Text() = %q", got)
	}
}
