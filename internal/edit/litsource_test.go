package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// API.md V1 for Source: a Canon literal with contextual names only, typed as the expected type.
func TestTypedSource(t *testing.T) {
	checkTyped(t, []typeCase{
		{"n", edit.Source("-3"), "-3"},
		{"f", edit.Source("2"), "2"},
		{"f", edit.Source("-1.25e2"), "-125"},
		{"g", edit.Source("0.1"), "0.1"},
		{"d", edit.Source("1m30s"), "1m30s"},
		{"on", edit.Source("true"), "true"},
		{"s", edit.Source(`"a\tb"`), "a\tb"},
		{"s", edit.Source(`r"a{b}"`), "a{b}"},
		{"s", edit.Source(`"a{s}"`), ""}, // an interpolation is not a literal
		{"c", edit.Source("green"), "green"},
		{"c", edit.Source("Color.green"), ""}, // not a contextual name
		{"c", edit.Source("blue"), ""},
		{"o", edit.Source("none"), "none"},
		{"o", edit.Source("(4)"), "4"},
		{"p", edit.Source("a"), "a"},
		{"p", edit.Source(`"a"`), "a"},
		{"row", edit.Source("7"), "7"},
		{"row", edit.Source("a"), ""},
		{"sh", edit.Source("circle { r: 2 }"), "circle{r: 2, label: }"},
		{"sh", edit.Source("dot"), "dot"},
		{"sh", edit.Source("circle"), "circle{r: , label: }"}, // E3302 is the re-check's (V3)
		{"ps", edit.Source(`[{ name: "q" }, { name: "r", w: 1 }]`), `[Part{name: "q", w: }, Part{name: "r", w: 1}]`},
		{"byC", edit.Source("{ red: 1, green: 2 }"), "{red: 1, green: 2}"},
		{"byS", edit.Source(`{ "k": 1 }`), `{"k": 1}`},
		{"byS", edit.Source("{ k: 1 }"), ""}, // a bare name keys only an enum or a ref (SPEC §6.1)
		{"byN", edit.Source(`{ 3: "x" }`), `{3: "x"}`},
		{"byP", edit.Source("{ a: 1 }"), "{a: 1}"},
		{"tag", edit.Source(`"none"`), "none"},
		{"tag", edit.Source("red"), "red"},
		{"part", edit.Source(`{ name: "q", name: "r" }`), ""},
		{"part", edit.Source(`Part { name: "q" }`), ""}, // a type name is not contextual
		{"part", edit.Source(`{ ...other }`), ""},
		{"n", edit.Source("1 + 2"), ""},
		{"n", edit.Source("1\nlet w = 2"), ""},
		{"n", edit.Source(""), ""},
		{"n", edit.Source("99999999999999999999"), ""},
	})
}

// API.md V1 for FromJSON: the wire form, decoded by WIRE.md's rules for the type; `null` is
// not a value of a required type.
func TestTypedFromJSON(t *testing.T) {
	checkTyped(t, []typeCase{
		{"n", edit.FromJSON("3"), "3"},
		{"c", edit.FromJSON(`"GREEN"`), "green"},
		{"c", edit.FromJSON(`"green"`), ""},
		{"o", edit.FromJSON("null"), "none"},
		{"sh", edit.FromJSON(`{"kind": "circle", "r": 2}`), "circle{r: 2, label: }"},
		{"part", edit.FromJSON(`{"name": "q"}`), "Part{name: \"q\", w: }"},
		{"part", edit.FromJSON(`{"name": "q", "size": 1}`), ""},
		{"byC", edit.FromJSON(`{"GREEN": 2}`), "{green: 2}"},
		{"n", edit.FromJSON(`"3"`), ""},
		{"n", edit.FromJSON(`3 4`), ""},
		{"cell", edit.FromJSON(`{"xs": [1, 2]}`), "Cell{on: , n: , f: , g: , s: , d: , c: , o: , p: , row: , sh: , ps: , xs: [1, 2], byC: , byS: , byN: , byP: , tag: , part: , bc: , node: }"},
	})
}

// API.md E25: a key of an edit's JSON form is read as a path key: for an enum, a string is a
// Canon name first, then a wire value (P1, P2); an integer is an integer key.
func TestTypedPathKeys(t *testing.T) {
	ts := fieldTypes(t)
	cases := []struct {
		key  edit.Lit
		kt   string // the field whose map key type the key is typed as
		want string
	}{
		{edit.PathKey("green"), "byC", "green"},
		{edit.PathKey("GREEN"), "byC", "green"},
		{edit.PathKey("blue"), "byC", ""},
		{edit.PathKey("k"), "byS", "k"},
		{edit.PathKey("3"), "byN", ""},
		{edit.IntKey(3), "byN", "3"},
		{edit.PathKey("a"), "byP", "a"},
		{edit.Key("a"), "byP", "a"},
		{edit.Key("green"), "byC", ""}, // API.md V4
		{edit.Member("green"), "byC", "green"},
		{edit.Source(`"k"`), "byS", "k"},
	}
	for _, c := range cases {
		kt := keyTypeOf(t, ts[c.kt])
		v, err := edit.Typer{Host: noHost{}}.Key(context.Background(), c.key, kt)
		switch {
		case c.want == "" && !errors.Is(err, edit.ErrBadValue):
			t.Errorf("Key(%#v, %s) = %v, %v, want ErrBadValue", c.key, kt, v, err)
		case c.want != "" && (err != nil || v.CanonText() != c.want):
			t.Errorf("Key(%#v, %s) = %v, %v, want %s", c.key, kt, v, err, c.want)
		}
	}
}

// API.md V1: a ValueError names the expected type, what was given and where inside the value.
func TestValueErrorText(t *testing.T) {
	ts := fieldTypes(t)
	cases := []struct {
		field string
		lit   edit.Lit
		want  string
	}{
		{"n", edit.Str("3"), `value does not fit the type: expected Int8, got Str("3")`},
		{"ps", edit.List{edit.Obj{"name": edit.Int(1)}}, "value does not fit the type: expected String, got Int(1): at [0].name"},
		{"byC", edit.Source("{ red: true }"), "value does not fit the type: expected Int, got true: at [red]"},
		{"d", edit.Dur(1), "value does not fit the type: expected Duration, got Dur(1ns): holds a fraction of a millisecond"},
		{"part", edit.Obj{"size": edit.Int(1)}, "value does not fit the type: expected v.Part, got Obj: no such field: size"},
	}
	for _, c := range cases {
		_, err := edit.Typer{Host: noHost{}}.Value(context.Background(), c.lit, ts[c.field])
		var ve *edit.ValueError
		if !errors.As(err, &ve) || err.Error() != c.want {
			t.Errorf("%s %#v: %v, want %s", c.field, c.lit, err, c.want)
		}
	}
}

// API.md V2, V3 (log-2026-09-29 M4 U4a): in a FromJSON, a missing required field (E3302), a
// range (E3201), a Float32 overflow (E3202) and two keys alike are the re-check's: kept. A float
// past Float64 has no value, and a key repeated in one object is not JSON (E7104): ValueErrors.
func TestTypedFromJSONKeepsValueFindings(t *testing.T) {
	checkTyped(t, []typeCase{
		{"part", edit.FromJSON(`{}`), "Part{name: , w: }"},
		{"n", edit.FromJSON(`1000`), "1000"},
		{"ps", edit.FromJSON(`[{"w": 1}]`), "[Part{name: , w: 1}]"},
		{"byN", edit.FromJSON(`{"0": "a", "-0": "b"}`), `{0: "a", 0: "b"}`}, // E3317, like a duplicate key
		{"f", edit.FromJSON(`1e400`), ""},
		{"byC", edit.FromJSON(`{"GREEN": 1, "GREEN": 2}`), ""},
	})
	v, err := edit.Typer{Host: noHost{}}.Value(context.Background(), edit.FromJSON(`1e39`), fieldTypes(t)["g"])
	if f, ok := v.(*value.Float); err != nil || !ok || f.V != 1e39 {
		t.Errorf("1e39 as Float32 = %v, %v, want 1e39 kept whole", v, err)
	}
}

// log-2026-09-29 M4 U4a: a string literal is a ref's key only for a String key type; an
// enum-keyed ref takes a Member (ACCEPTED), never a string, whatever its text.
func TestTypedStringIsNoEnumKey(t *testing.T) {
	checkTyped(t, []typeCase{
		{"bc", edit.Source(`"red"`), ""},
		{"bc", edit.Source(`"GREEN"`), ""},
		{"bc", edit.Source(`"nope"`), ""},
		{"bc", edit.Source("red"), "red"},
		{"bc", edit.Member("green"), "green"},
		{"bc", edit.Key("red"), ""},
	})
	ts := fieldTypes(t)
	shape, _ := ts["sh"].Base().(*types.VariantType)
	kindKey := &types.Field{Name: "k", Type: &types.VariantKindType{Variant: shape}}
	byKind := &types.RefType{Target: &types.Collection{Kind: types.CollLet, Pkg: "v", Name: "byKinds", KeyedBy: kindKey}}
	for _, text := range []string{`"dot"`, `"circle"`, `"nope"`} {
		if _, err := (edit.Typer{Host: noHost{}}).Value(context.Background(), edit.Source(text), byKind); !errors.Is(err, edit.ErrBadValue) {
			t.Errorf("%s for %s: %v, want ErrBadValue", text, byKind, err)
		}
	}
}

// keyTypeOf is the key type of the map type t.
func keyTypeOf(t *testing.T, mt types.Type) types.Type {
	t.Helper()
	m, ok := mt.Base().(*types.MapType)
	if !ok {
		t.Fatalf("%s is not a map type", mt)
	}
	return m.Key
}
