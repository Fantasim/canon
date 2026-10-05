package wire_test

import (
	"context"
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

func text(t *testing.T, v value.Value) string {
	t.Helper()
	b, err := wire.Text(v)
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	return string(b)
}

func shopBadge() (*types.RecordType, *value.Record) {
	badge := record("shop", "Badge", field("title", types.StringType), field("count", types.IntType),
		field("tags", &types.ListType{Elem: types.StringType}))
	return badge, rec(badge, str("Café 🎉"), num(2), list(types.StringType))
}

func shopReward() (*types.VariantType, *value.Record) {
	item := &types.CaseType{Name: "item", Fields: []*types.Field{field("id", types.StringType), field("count", types.IntType)}}
	gold := &types.CaseType{Name: "gold", Fields: []*types.Field{field("amount", types.IntType)}}
	vt := variantOf("shop", "Reward", "kind", item, gold)
	return vt, rec(item, str("sword"), num(1))
}

// WIRE.md §8.5, DECISIONS 308: the byte samples of the section.
func TestTextSamples(t *testing.T) {
	_, badge := shopBadge()
	_, reward := shopReward()
	tiers := &value.Map{T: &types.MapType{Key: types.IntType, Value: types.StringType},
		Keys: []value.Value{num(10), num(2)}, Vals: []value.Value{str("high"), str("low")}}
	for _, tc := range []struct {
		name string
		v    value.Value
		want string
	}{
		{"badge.json", badge, "{\n  \"title\": \"Café 🎉\",\n  \"count\": 2,\n  \"tags\": []\n}\n"},
		{"tiers.json", tiers, "{\n  \"10\": \"high\",\n  \"2\": \"low\"\n}\n"},
		{"reward.json", reward, "{\n  \"kind\": \"item\",\n  \"id\": \"sword\",\n  \"count\": 1\n}\n"},
		{"limit.json", none(opt(types.IntType)), "null\n"},
	} {
		if got := text(t, tc.v); got != tc.want {
			t.Errorf("%s:\n%q\nwant:\n%q", tc.name, got, tc.want)
		}
	}
}

// WIRE.md §7.2, §7.3, §7.4, §5.3, §5.8, §8.5: scalars, escapes, keys, codes, arrays.
func TestTextScalars(t *testing.T) {
	codesEnum := codes(enum("p", "C", "a", "b"), 1, 7)
	codesEnum.WireCodes = true
	intMap := &value.Map{T: &types.MapType{Key: types.IntType, Value: types.BoolType},
		Keys: []value.Value{num(-1), num(5)}, Vals: []value.Value{boolean(true), boolean(false)}}
	for _, tc := range []struct {
		name string
		v    value.Value
		want string
	}{
		{"accent", str("é"), "\"é\"\n"},
		{"emoji", str("😀"), "\"😀\"\n"},
		{"u2028", str("\u2028"), "\"\u2028\"\n"},
		{"escape", str("a\té/\"\x1b\\"), "\"a\\té/\\\"\\u001b\\\\\"\n"},
		{"1e21", flt(1e21), "1e+21\n"},
		{"negzero", flt(math.Copysign(0, -1)), "0\n"},
		{"maxint", num(math.MaxInt64), "9223372036854775807\n"},
		{"intmap", intMap, "{\n  \"-1\": true,\n  \"5\": false\n}\n"},
		{"codes", member(codesEnum, 1), "7\n"},
		{"none", none(opt(types.IntType)), "null\n"},
		{"emptyobj", &value.Map{T: &types.MapType{Key: types.StringType, Value: types.IntType}}, "{}\n"},
		{"float32", &value.Float{V: float64(float32(0.1)), T: types.Float32Type}, "0.1\n"},
		{"array", list(types.IntType, num(1), num(2)), "[\n  1,\n  2\n]\n"},
		{"nested", list(listOf(types.StringType), list(types.StringType, str("a")), list(types.StringType)), "[\n  [\n    \"a\"\n  ],\n  []\n]\n"},
	} {
		if got := text(t, tc.v); got != tc.want {
			t.Errorf("%s: %q want %q", tc.name, got, tc.want)
		}
	}
}

// WIRE.md §5.12, §8.5: a nested table keeps `$retired`, is keyed by id, and has no `$id`.
func TestTextTableRetired(t *testing.T) {
	et := record("flow", "Status", field("label", types.StringType))
	tt := &types.TableType{Elem: et}
	open := rec(et, str("Open"))
	open.Ident = &value.Identity{Key: value.Key{S: "open"}}
	taken := rec(et, str("Taken"))
	taken.Ident = &value.Identity{Key: value.Key{S: "taken"}, Retired: true}
	got := text(t, &value.Table{T: tt, Entries: []*value.Record{open, taken}})
	want := "{\n  \"open\": {\n    \"label\": \"Open\"\n  },\n  \"taken\": {\n    \"$retired\": true,\n    \"label\": \"Taken\"\n  }\n}\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if strings.Contains(got, "$id") || strings.Contains(got, "$schema") || strings.Contains(got, "$fns") {
		t.Errorf("document keys in %q", got)
	}
}

// WIRE.md §5.9, DECISIONS 308: a value with no wire form is refused.
func TestTextNoWire(t *testing.T) {
	if _, err := wire.Text(flt(math.NaN())); err == nil {
		t.Error("NaN written")
	}
}

// roundTripText writes v with Text, parses it and decodes it at typ.
func roundTripText(t testing.TB, v value.Value, typ types.Type) {
	t.Helper()
	b, err := wire.Text(v)
	if err != nil {
		t.Fatalf("Text %s: %v", v.CanonText(), err)
	}
	fs := &source.FileSet{}
	f, _ := fs.Add("r.json", "/p/r.json", b)
	bag := diag.NewBag(fs, "p")
	root, err := jsonsrc.Parse(f, bag)
	if err != nil {
		t.Fatalf("parse %s: %v", b, err)
	}
	dec := wire.Decoder{Bag: bag, Pkg: "p"}
	got, ok, err := dec.Decode(context.Background(), wire.Selection{Node: root}, typ)
	if !ok || err != nil {
		t.Fatalf("decode %s: %v %v", b, err, short(fs, bag.Findings()))
	}
	if !sameValue(got, v) {
		t.Fatalf("decode(Text(v)) differs:\n%s\n%s\nfrom %s", got.CanonText(), v.CanonText(), b)
	}
}

// textDepth bounds the drawn types.
const textDepth = 3

// drawText draws a random type of the depth and a value of it from the two seeds.
func drawText(a, b uint64, depth int) (types.Type, value.Value) {
	codesEnum := codes(enum("p", "C", "a", "b", "c"), 1, 7, 42)
	codesEnum.WireCodes = true
	g := &gtype{r: rand.New(rand.NewPCG(a, b)), enums: []*types.EnumType{tone, codesEnum, side}}
	typ, draw := g.of(depth)
	return typ, draw()
}

// WIRE.md §8.5, DECISIONS 308: `load` of the file at the result type gives the value back.
func TestTextRoundTrip(t *testing.T) {
	for i := range uint64(2000) {
		typ, v := drawText(i, i+1, textDepth)
		roundTripText(t, v, typ)
	}
	bt, badge := shopBadge()
	roundTripText(t, badge, bt)
	vt, reward := shopReward()
	roundTripText(t, reward, vt)
}

// FuzzTextRoundTrip: decode(Text(v)) at the type of v equals v (WIRE.md §8.5).
func FuzzTextRoundTrip(f *testing.F) {
	for _, s := range [][2]uint64{{0, 1}, {3, 4}, {42, 7}, {1 << 40, 99}} {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, a, b uint64) {
		typ, v := drawText(a, b, textDepth)
		roundTripText(t, v, typ)
	})
}
