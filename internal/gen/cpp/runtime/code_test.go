package cppruntime

import "testing"

// CODEGEN.md §2.7, DECISIONS 423 (pending): Code blanks every comment of both forms, a `//` comment with a trailing backslash included, and keeps string and character literals whole.
func TestCode(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, text, want string }{
		{"line comment", "a // b\nc", "a  c"},
		{"trailing backslash", "a // b \\\nc", "a  c"},
		{"block comment", "a /** b\n * std::vector \\\n */ c", "a   c"},
		{"literal keeps comment marks", `a "// b /* c" 'x' d`, `a "// b /* c" 'x' d`},
		{"unterminated block", "a /* b", "a  "},
		{"empty", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := Code(c.text); got != c.want {
				t.Errorf("Code(%q) = %q, want %q", c.text, got, c.want)
			}
		})
	}
}
