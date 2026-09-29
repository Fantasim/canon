package jsonsrc_test

import (
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// potion is FORMATTER.md §14.2's data/II_POT_HEAL_S.json.
const potion = `{
  "dwID": "II_POT_HEAL_S",
  "szName": "IDS_PROPITEM_TXT_POT_S",
  "nHeal": 500,
  "dwCooldownMs": 3000
}
`

// value is the tree of a JSON text, the value of an edit.
func value(t *testing.T, text string) *jsonsrc.Node {
	t.Helper()
	return mustParse(t, text).root
}

// FORMATTER.md §14.2: a new key goes after the nearest earlier declared key, else before.
func TestPlace(t *testing.T) {
	obj := value(t, potion)
	declared := []string{"dwID", "szName", "nHeal", "dwCooldownMs", "nStack", "szIcon"}
	tests := []struct {
		declared []string
		key      string
		want     int
	}{
		{declared, "nStack", 4},
		{[]string{"x", "szName", "dwID"}, "x", 1},
		{[]string{"x", "y"}, "x", 4},
		{declared, "unknown", 4},
		{[]string{"a", "dwID", "b", "szName"}, "b", 1},
		{[]string{"nHeal", "b", "dwID"}, "b", 3},
		// LOD-09: fields dwID, szName, stats.hp, nHeal, stats.mp; the prefix stands where stats.hp does.
		{[]string{"dwID", "szName", "stats", "nHeal", "dwCooldownMs"}, "stats", 2},
	}
	for _, tt := range tests {
		if got := jsonsrc.Place(obj, tt.declared, tt.key); got != tt.want {
			t.Errorf("%v %q: %d, want %d", tt.declared, tt.key, got, tt.want)
		}
	}
}

// RFC 6901, LOD-02: a pointer names a node; an index is decimal without a leading zero.
func TestFind(t *testing.T) {
	root := value(t, `{"a/b": [1, {"m~n": true}], "": 0}`)
	tests := []struct {
		ptr, want string
	}{
		{"", "{"}, {"/a~1b/1/m~0n", "true"}, {"/a~1b/0", "1"}, {"/", "0"},
		{"/a~1b/01", ""}, {"/a~1b/2", ""}, {"/x", ""}, {"a", ""}, {"/a~1b/-1", ""}, {"/a~1b/0/x", ""},
	}
	for _, tt := range tests {
		n := root.Find(tt.ptr)
		got := ""
		if n != nil {
			got = n.Text
			if n.Kind == jsonsrc.Object {
				got = "{"
			}
		}
		if got != tt.want {
			t.Errorf("%q: %q, want %q", tt.ptr, got, tt.want)
		}
	}
}

// jsonEditCase is one Rewrite of a JSON source.
type jsonEditCase struct {
	rule, src, want string
	edits           func(t *testing.T) []jsonsrc.Edit
}

var jsonEditCases = []jsonEditCase{
	{
		rule: "FORMATTER.md §14.2, LOD-09: a missing @json(path:) object is created where its first field stands",
		src:  potion,
		want: "{\n  \"dwID\": \"II_POT_HEAL_S\",\n  \"szName\": \"IDS_PROPITEM_TXT_POT_S\",\n  \"stats\": {\n    \"hp\": 5\n  },\n" +
			"  \"nHeal\": 500,\n  \"dwCooldownMs\": 3000\n}\n",
		edits: func(t *testing.T) []jsonsrc.Edit {
			at := jsonsrc.Place(value(t, potion), []string{"dwID", "szName", "stats", "nHeal", "dwCooldownMs"}, "stats")
			return []jsonsrc.Edit{{Kind: jsonsrc.Insert, Key: "stats", At: at, Value: value(t, `{"hp": 5}`)}}
		},
	},
	{
		rule: "FORMATTER.md §14.2: nStack follows dwCooldownMs in Potion's declaration order",
		src:  potion,
		want: "{\n  \"dwID\": \"II_POT_HEAL_S\",\n  \"szName\": \"IDS_PROPITEM_TXT_POT_S\",\n  \"nHeal\": 500,\n" +
			"  \"dwCooldownMs\": 3000,\n  \"nStack\": 20\n}\n",
		edits: func(t *testing.T) []jsonsrc.Edit {
			return []jsonsrc.Edit{{Kind: jsonsrc.Insert, Key: "nStack", At: 4, Value: value(t, "20")}}
		},
	},
	{
		rule: "FORMATTER.md §14.2, API.md E10: a first key takes the line after \"{\", with its \",\"",
		src:  potion,
		want: "{\n  \"$retired\": true,\n  \"dwID\": \"II_POT_HEAL_S\",\n  \"szName\": \"IDS_PROPITEM_TXT_POT_S\",\n" +
			"  \"nHeal\": 500,\n  \"dwCooldownMs\": 3000\n}\n",
		edits: func(t *testing.T) []jsonsrc.Edit {
			return []jsonsrc.Edit{{Kind: jsonsrc.Insert, Key: "$retired", Value: value(t, "true")}}
		},
	},
	{
		rule: "FORMATTER.md §14.2: a set value is printed in canonical layout at its depth; the rest is kept",
		src:  "{\n  \"a\": 1.50,\n  \"b\": [\n    1\n  ],\n  \"c\": 3\n}\n",
		want: "{\n  \"a\": 1.50,\n  \"b\": {\n    \"x\": [\n      2\n    ]\n  },\n  \"c\": 4\n}\n",
		edits: func(t *testing.T) []jsonsrc.Edit {
			return []jsonsrc.Edit{
				{Kind: jsonsrc.Set, Pointer: "/b", Value: value(t, `{"x": [2]}`)},
				{Kind: jsonsrc.Set, Pointer: "/c", Value: value(t, "4")},
			}
		},
	},
	{
		rule: "FORMATTER.md §14.2: elements and members removed with the \",\" of the neighbouring line",
		src:  "{\n  \"a\": [\n    1,\n    2,\n    3\n  ],\n  \"b\": 1,\n  \"c\": {\n    \"x\": 1\n  }\n}\n",
		want: "{\n  \"a\": [\n    2\n  ],\n  \"c\": {}\n}\n",
		edits: func(t *testing.T) []jsonsrc.Edit {
			return []jsonsrc.Edit{
				{Kind: jsonsrc.Remove, Pointer: "/a/0"},
				{Kind: jsonsrc.Remove, Pointer: "/a/1"},
				{Kind: jsonsrc.Remove, Pointer: "/b"},
				{Kind: jsonsrc.Remove, Pointer: "/c/x"},
			}
		},
	},
	{
		rule: "FORMATTER.md §14.2: an element inserted in an array, and in an empty one, which breaks",
		src:  "{\n  \"a\": [\n    1,\n    3\n  ],\n  \"b\": []\n}\n",
		want: "{\n  \"a\": [\n    1,\n    2,\n    3\n  ],\n  \"b\": [\n    {\n      \"k\": \"v\"\n    }\n  ]\n}\n",
		edits: func(t *testing.T) []jsonsrc.Edit {
			return []jsonsrc.Edit{
				{Kind: jsonsrc.Insert, Pointer: "/a", At: 1, Value: value(t, "2")},
				{Kind: jsonsrc.Insert, Pointer: "/b", Value: value(t, `{"k": "v"}`)},
			}
		},
	},
	{
		rule: "FORMATTER.md §14.2: a unit inside a container written on one line escalates to it, and up",
		src:  "{\n  \"a\": {\"b\": [1, 2], \"c\": 1.0},\n  \"d\": 1.0\n}\n",
		want: "{\n  \"a\": {\n    \"b\": [\n      1,\n      5\n    ],\n    \"c\": 1.0\n  },\n  \"d\": 1.0\n}\n",
		edits: func(t *testing.T) []jsonsrc.Edit {
			return []jsonsrc.Edit{{Kind: jsonsrc.Set, Pointer: "/a/b/1", Value: value(t, "5")}}
		},
	},
	{
		rule: "FORMATTER.md §14.2: a file never normalized is re-printed whole when its root is on one line",
		src:  `{"a": 1, "b": 2}`,
		want: "{\n  \"a\": 1\n}",
		edits: func(t *testing.T) []jsonsrc.Edit {
			return []jsonsrc.Edit{{Kind: jsonsrc.Remove, Pointer: "/b"}}
		},
	},
}

// FORMATTER.md §14.2: edits keep every byte but the members and elements they change.
func TestRewrite(t *testing.T) {
	for _, c := range jsonEditCases {
		got, err := jsonsrc.Rewrite([]byte(c.src), c.edits(t))
		if err != nil || string(got) != c.want {
			t.Errorf("%s:\n got %q, %v\nwant %q", c.rule, got, err, c.want)
		}
	}
}

// cycle is an array holding itself, twice: a caller's value Rewrite must refuse, not print.
func cycle() *jsonsrc.Node {
	n := &jsonsrc.Node{Kind: jsonsrc.Array}
	n.Elems = []*jsonsrc.Node{n, n}
	return n
}

// FORMATTER.md §14.1, §14.2, log-2026-09-29 M4 U1r: a bad edit or value, invalid JSON or a repeated key changes nothing.
func TestRewriteRefuses(t *testing.T) {
	one := value(t, "1")
	tests := []struct {
		src  string
		edit jsonsrc.Edit
		want error
	}{
		{potion, jsonsrc.Edit{Kind: jsonsrc.Remove + 1}, jsonsrc.ErrEdit},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Set, Pointer: "/x", Value: one}, jsonsrc.ErrEdit},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Set, Pointer: "/nHeal"}, jsonsrc.ErrEdit},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Insert, Pointer: "/nHeal", Value: one}, jsonsrc.ErrEdit},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Insert, Key: "x", At: 5, Value: one}, jsonsrc.ErrEdit},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Insert, Key: "nHeal", Value: one}, jsonsrc.ErrEdit},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Remove}, jsonsrc.ErrEdit},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Remove, Pointer: "/x"}, jsonsrc.ErrEdit},
		{`{"a": 1,}`, jsonsrc.Edit{Kind: jsonsrc.Remove, Pointer: "/a"}, jsonsrc.ErrSyntax},
		{`{"a": 1, "a": 2}`, jsonsrc.Edit{Kind: jsonsrc.Remove, Pointer: "/a"}, jsonsrc.ErrDuplicateKey},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Set, Pointer: "/nHeal", Value: &jsonsrc.Node{Kind: jsonsrc.Object + 1}}, jsonsrc.ErrEdit},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Set, Pointer: "/nHeal", Value: &jsonsrc.Node{Kind: jsonsrc.Array, Elems: []*jsonsrc.Node{nil}}}, jsonsrc.ErrEdit},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Insert, Key: "x", Value: &jsonsrc.Node{Kind: jsonsrc.Object, Members: []jsonsrc.Member{{Key: "y"}}}}, jsonsrc.ErrEdit},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Set, Pointer: "/nHeal", Value: cycle()}, jsonsrc.ErrEdit},
		{potion, jsonsrc.Edit{Kind: jsonsrc.Set, Pointer: "/nHeal", Value: &jsonsrc.Node{Kind: jsonsrc.Number, Text: "x"}}, jsonsrc.ErrEdit},
	}
	for _, tt := range tests {
		if out, err := jsonsrc.Rewrite([]byte(tt.src), []jsonsrc.Edit{tt.edit}); !errors.Is(err, tt.want) || out != nil {
			t.Errorf("%+v: got %q, %v; want %v", tt.edit, out, err, tt.want)
		}
	}
}
