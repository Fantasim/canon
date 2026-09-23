package edit_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
)

func field(name string) edit.Seg { return edit.Seg{Kind: edit.SegField, Name: name} }

func word(w string) edit.Seg {
	return edit.Seg{Kind: edit.SegKey, Key: edit.KeyLit{Kind: edit.KeyWord, Text: w, Raw: w}}
}

func integer(n int64, raw string) edit.Seg {
	return edit.Seg{Kind: edit.SegKey, Key: edit.KeyLit{Kind: edit.KeyInt, Int: n, Raw: raw}}
}

func quoted(text, raw string) edit.Seg {
	return edit.Seg{Kind: edit.SegKey, Key: edit.KeyLit{Kind: edit.KeyString, Text: text, Raw: raw}}
}

func position(n int) edit.Seg { return edit.Seg{Kind: edit.SegPos, Pos: n} }

// validPaths are the paths of API.md §6 and the corners of its grammar.
var validPaths = []struct {
	in   string
	want edit.Path
}{
	{"teamboard:statuses.open.next[0]", edit.Path{Package: "teamboard", Root: "statuses", Segs: []edit.Seg{field("open"), field("next"), integer(0, "0")}}},
	{"resource.farm:farm.modelTypes[3].levels[1].productionItem", edit.Path{Package: "resource.farm", Root: "farm",
		Segs: []edit.Seg{field("modelTypes"), integer(3, "3"), field("levels"), integer(1, "1"), field("productionItem")}}},
	{"resource.adventurequest:adventureQuests.styles[daily].taskWeights", edit.Path{Package: "resource.adventurequest", Root: "adventureQuests",
		Segs: []edit.Seg{field("styles"), word("daily"), field("taskWeights")}}},
	{`adventureQuests.styles["daily"]`, edit.Path{Root: "adventureQuests", Segs: []edit.Seg{field("styles"), quoted("daily", `"daily"`)}}},
	{"pipeline:potions[II_POT_HEAL_L].cooldown", edit.Path{Package: "pipeline", Root: "potions", Segs: []edit.Seg{word("II_POT_HEAL_L"), field("cooldown")}}},
	{"farm.modelTypes[#3]", edit.Path{Root: "farm", Segs: []edit.Seg{field("modelTypes"), position(3)}}},
	{"farm.modelTypes[#0][#12]", edit.Path{Root: "farm", Segs: []edit.Seg{field("modelTypes"), position(0), position(12)}}},
	{"Element.FIRE", edit.Path{Root: "Element", Segs: []edit.Seg{field("FIRE")}}},
	{"resource.vocab:Element.FIRE", edit.Path{Package: "resource.vocab", Root: "Element", Segs: []edit.Seg{field("FIRE")}}},
	{"a.b.c", edit.Path{Root: "a", Segs: []edit.Seg{field("b"), field("c")}}},
	{"m[-5][-0][0]", edit.Path{Root: "m", Segs: []edit.Seg{integer(-5, "-5"), integer(0, "-0"), integer(0, "0")}}},
	{"m[-9223372036854775808][9223372036854775807]", edit.Path{Root: "m",
		Segs: []edit.Seg{integer(-9223372036854775808, "-9223372036854775808"), integer(9223372036854775807, "9223372036854775807")}}},
	{`m["a b:c]."]`, edit.Path{Root: "m", Segs: []edit.Seg{quoted("a b:c].", `"a b:c]."`)}}},
	{`m["\u0061\n"]`, edit.Path{Root: "m", Segs: []edit.Seg{quoted("a\n", `"\u0061\n"`)}}},
	{`m[""]`, edit.Path{Root: "m", Segs: []edit.Seg{quoted("", `""`)}}},
	{"_x.__._0[_1]", edit.Path{Root: "_x", Segs: []edit.Seg{field("__"), field("_0"), word("_1")}}},
	{"record.check[package]", edit.Path{Root: "record", Segs: []edit.Seg{field("check"), word("package")}}},
}

// invalidPaths break API.md §6.1: whitespace, empty parts, signs, leading zeros, ranges.
var invalidPaths = []string{
	"", "_", ".a", "a.", "a[", "a[]", "a[#]", "a[#-1]", "a[#01]", "a[01]", "a[-]", "a[-01]", "a[1",
	"a b", "a..b", "a.0", "a._", ":a", "a:", "p:", "a:b:c", "a.b:", `a["x]`, `a["\x"]`, `a["\ud800"]`,
	"a[99999999999999999999]", "a[#99999999999999999999]", "a[9223372036854775808]",
	"a[-9223372036854775809]", "é", "a.é", "a.b[x]y", "a[x y]", "1a", "a[x]]", "a[[x]]", "a[1.5]",
	"a[#1", "a[x", "a[+1]", "a[0x1]", "a\t", " a", "a[\"\x01\"]", "p.:a", "p..q:a", "a[#+1]",
}

// API.md §6.1 (API-02): paths parse into their segments and print back as written.
func TestParse(t *testing.T) {
	for _, c := range validPaths {
		got, err := edit.Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
		}
		if s := got.String(); s != c.in {
			t.Errorf("Parse(%q).String() = %q", c.in, s)
		}
	}
}

// API.md §6.1, P4: every other string is ErrBadPath.
func TestParseRefuses(t *testing.T) {
	for _, in := range invalidPaths {
		if p, err := edit.Parse(in); !errors.Is(err, edit.ErrBadPath) {
			t.Errorf("Parse(%q) = %+v, %v; want ErrBadPath", in, p, err)
		}
	}
	if _, err := edit.Parse(`a["\x"]`); !errors.Is(err, diag.ErrJSONString) {
		t.Errorf("a bad JSON string key: %v, want it to wrap ErrJSONString", err)
	}
}

// API.md §15: a refusal is a *SyntaxError with its byte and reason, both in its text.
func TestSyntaxError(t *testing.T) {
	cases := []struct {
		in     string
		offset int
		text   string
	}{
		{"a[01]", 2, "invalid path: invalid digits at byte 2"},
		{"a.b[x", 5, "invalid path: expected ] at byte 5"},
		{`m["\x"]`, 2, "invalid path: invalid JSON string: byte 1: invalid escape at byte 2"},
	}
	for _, c := range cases {
		_, err := edit.Parse(c.in)
		var se *edit.SyntaxError
		if !errors.As(err, &se) || se.Offset != c.offset || se.Reason == nil || err.Error() != c.text {
			t.Errorf("Parse(%q) = %v (%+v), want offset %d and %q", c.in, err, se, c.offset, c.text)
		}
	}
}

// A path built in code prints each key in the form its kind names.
func TestStringOfBuiltPath(t *testing.T) {
	p := edit.Path{Package: "p.q", Root: "r", Segs: []edit.Seg{
		{Kind: edit.SegKey, Key: edit.KeyLit{Kind: edit.KeyString, Text: "a\"b"}},
		{Kind: edit.SegKey, Key: edit.KeyLit{Kind: edit.KeyInt, Int: -3}},
		{Kind: edit.SegKey, Key: edit.KeyLit{Kind: edit.KeyWord, Text: "w"}},
		{Kind: edit.SegPos, Pos: 2},
		{Kind: edit.SegField, Name: "f"},
	}}
	if got, want := p.String(), `p.q:r["a\"b"][-3][w][#2].f`; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// IMPLEMENTATION-PLAN.md §7.7: Parse/String round trip; a refusal is always ErrBadPath.
func FuzzParse(f *testing.F) {
	for _, c := range validPaths {
		f.Add(c.in)
	}
	for _, in := range invalidPaths {
		f.Add(in)
	}
	f.Fuzz(func(t *testing.T, in string) {
		p, err := edit.Parse(in)
		if err != nil {
			if !errors.Is(err, edit.ErrBadPath) {
				t.Fatalf("Parse(%q): %v does not wrap ErrBadPath", in, err)
			}
			return
		}
		if s := p.String(); s != in {
			t.Fatalf("Parse(%q).String() = %q", in, s)
		}
		again, err := edit.Parse(p.String())
		if err != nil || !reflect.DeepEqual(again, p) {
			t.Fatalf("Parse(%q) again = %+v, %v; want %+v", in, again, err, p)
		}
	})
}
