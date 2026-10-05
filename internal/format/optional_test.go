package format_test

import "testing"

// FORMATTER.md §10, DECISIONS 319: a redundant `= none` goes, comments stay, a fixed point.
func TestOptionalDefaultRemoved(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"removed", "record R {\n  a: Int? = none\n}\n", "record R {\n  a: Int?\n}\n"},
		{"trailing comment", "record R {\n  a: Int? = none // c\n}\n", "record R {\n  a: Int? // c\n}\n"},
		{"comment between", "record R {\n  a: Int? = /* c */ none\n}\n", "record R {\n  a: Int? /* c */\n}\n"},
		{"parameter kept", "fn f(x: Int? = none) -> Int { return 1 }\n", "fn f(x: Int? = none) -> Int { return 1 }\n"},
		{"alias kept", "record R {\n  a: Opt = none\n}\n", "record R {\n  a: Opt = none\n}\n"},
		{"non-optional kept", "record R {\n  a: Int = none\n}\n", "record R {\n  a: Int = none\n}\n"},
		{"other default kept", "record R {\n  a: Int? = 1\n}\n", "record R {\n  a: Int? = 1\n}\n"},
	}
	for _, c := range cases {
		in := "package p\n\n" + c.in
		got, err := formatText(t, "a/a.canon", []byte(in))
		if err != nil || string(got) != "package p\n\n"+c.want {
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
			continue
		}
		again, err := formatText(t, "a/a.canon", got)
		if err != nil || string(again) != string(got) {
			t.Errorf("%s: not a fixed point: %q, %v", c.name, again, err)
		}
	}
}
