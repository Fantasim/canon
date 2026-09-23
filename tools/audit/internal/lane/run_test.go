package lane

import (
	"slices"
	"testing"
)

func TestParseDirectives(t *testing.T) {
	src := []byte("a\n// sovaudit:ignore fn-length -- table test\nb // sovaudit:ignore magic-number,magic-string -- wire format\n" +
		"<!-- sovaudit:ignore dead-link -- generated demo -->\n// sovaudit:ignore fn-length\n// sovaudit:ignore-file magic-string -- rulebook table\n")
	ds := parseDirectives("x.go", src)
	tests := []struct {
		line  int
		rules []string
		valid bool
		whole bool
	}{
		{2, []string{"fn-length"}, true, false},
		{3, []string{"magic-number", "magic-string"}, true, false},
		{4, []string{"dead-link"}, true, false},
		{5, []string{"fn-length"}, false, false},
		{6, []string{"magic-string"}, true, true},
	}
	if len(ds) != len(tests) {
		t.Fatalf("got %d directives", len(ds))
	}
	for i, tt := range tests {
		d := ds[i]
		if d.Line != tt.line || !slices.Equal(d.Rules, tt.rules) || d.Valid() != tt.valid || d.Whole != tt.whole {
			t.Errorf("directive %d = %+v", i, d)
		}
	}
	if ds[2].Reason != "generated demo" {
		t.Errorf("html reason = %q", ds[2].Reason)
	}
}

func TestDirectiveMentionIsNotADirective(t *testing.T) {
	src := []byte("// Directive is one `sovaudit:ignore` found in a file.\n// every sovaudit:ignore / sovaudit:ignore-file here\n")
	if ds := parseDirectives("x.go", src); len(ds) != 0 {
		t.Fatalf("prose mentions parsed as directives: %+v", ds)
	}
}

func TestGoDirectiveMustOpenTheComment(t *testing.T) {
	src := []byte("// the marker: `// sovaudit:ignore <rules> -- <reason>`\n// sovaudit:ignore fn-length -- table test\n")
	ds := parseWith(reGoDirective, "x.go", src)
	if len(ds) != 1 || ds[0].Line != 2 {
		t.Fatalf("got %+v, want only line 2", ds)
	}
}

func TestMarkdownDirectiveMustOpenItsLine(t *testing.T) {
	src := []byte("Quote the Go form: `// sovaudit:ignore <rule> -- <reason>`.\n<!-- sovaudit:ignore dead-link -- demo link -->\n")
	ds := parseWith(reMdDirective, "x.md", src)
	if len(ds) != 1 || ds[0].Line != 2 || !ds[0].Valid() {
		t.Fatalf("got %+v, want only the HTML comment on line 2", ds)
	}
}
