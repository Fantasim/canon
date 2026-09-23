package jsonsrc_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// FORMATTER.md §14.1: the canonical layout, WIRE.md §7.3 strings, key order kept, idempotent.
func TestFormat(t *testing.T) {
	tests := []struct{ in, want string }{
		{"{}", "{}\n"},
		{" [ ] ", "[]\n"},
		{"null", "null\n"},
		{`"x"`, "\"x\"\n"},
		{
			`  {"b":1,"a":[true,false,null,{}],"c":{"d":[],"e":{"f":[0]}}} `,
			"{\n  \"b\": 1,\n  \"a\": [\n    true,\n    false,\n    null,\n    {}\n  ],\n  \"c\": {\n    \"d\": [],\n" +
				"    \"e\": {\n      \"f\": [\n        0\n      ]\n    }\n  }\n}\n",
		},
		{
			`["\u00e9\/\u001B<>&\u2028", "\ud83d\ude00", "\b\f\n\r\t\u0000\u007f\"\\"]`,
			"[\n  \"é/\\u001b<>&\u2028\",\n  \"😀\",\n  \"\\b\\f\\n\\r\\t\\u0000\x7f\\\"\\\\\"\n]\n",
		},
		{`{"\u0041\"": 1}`, "{\n  \"A\\\"\": 1\n}\n"},
		{"[1.0, 1E5, -0, 0.10, 1e-7]", "[\n  1.0,\n  1E5,\n  -0,\n  0.10,\n  1e-7\n]\n"},
		{"\xef\xbb\xbf{\r\n\t\"a\": 1\r\n}\r\n", "{\n  \"a\": 1\n}\n"},
	}
	for _, tt := range tests {
		got := string(jsonsrc.Format(mustParse(t, tt.in).root))
		if got != tt.want {
			t.Errorf("%q:\n got %q\nwant %q", tt.in, got, tt.want)
		}
		if again := string(jsonsrc.Format(mustParse(t, got).root)); again != got {
			t.Errorf("%q: not idempotent:\n%s", tt.in, again)
		}
	}
}

// FORMATTER.md §14.1 (numbers), DECISIONS 165: Format writes a number's Text as the caller set it.
func TestFormatNumberText(t *testing.T) {
	root := mustParse(t, `{"known": 1.50, "unknown": 1.50}`).root
	root.Members[0].Value.Text = "1.5"
	if got, want := string(jsonsrc.Format(root)), "{\n  \"known\": 1.5,\n  \"unknown\": 1.50\n}\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
