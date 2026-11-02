package syntax_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// GRAMMAR.md §7.1: only the value of out_templates takes interpolations, the exemption resets after it.
func TestProjectTemplatesInterp(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []int // the 1-based lines of the refused strings
		torn bool  // the text is broken: other findings are expected
	}{
		{"under out_templates", "project p {\n  out_templates {\n    go: { admin: \"gen/{dir}\" }\n  }\n}\n", nil, false},
		{"under roots", "project p {\n  roots {\n    admin: \"../{x}\"\n  }\n}\n", []int{3}, false},
		{"top-level value", "project p {\n  out_templates: \"{dir}\"\n}\n", nil, false},
		{"key after", "project p {\n  out_templates {\n    go: { admin: \"{dir}\" }\n  }\n  canon: \"{x}\"\n}\n", []int{5}, false},
		{"key before", "project p {\n  canon: \"{x}\"\n  out_templates {\n    go: { admin: \"{dir}\" }\n  }\n}\n", []int{2}, false},
		{"nested key", "project p {\n  out_templates {\n    go: { \"{k}\": \"{dir}\" }\n  }\n}\n", []int{3}, false},
		{"other key named alike", "project p {\n  out_templates_x: \"{x}\"\n}\n", []int{2}, false},
		{"unclosed out_templates", "project p {\n  out_templates {\n    go: { a: \"{dir}\"\n  compiler: \"1.{x}\"\n}\n", []int{3, 4}, true},
		{"unclosed after a closed target", "project p {\n  out_templates {\n    go: { a: \"{dir}\" }\n  compiler: \"1.{x}\"\n}\n", []int{3, 4}, true},
		{"unclosed list", "project p {\n  out_templates: [\"{dir}\"\n  compiler: \"1.{x}\"\n}\n", []int{2}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set := &source.FileSet{}
			bag := diag.NewBag(set, "p")
			src, err := set.Add(projectFile, "/"+projectFile, []byte(c.text))
			if err != nil {
				t.Fatal(err)
			}
			syntax.Parse(src, syntax.FileProject, bag)
			var got []int
			for _, f := range bag.Findings() {
				if f.Code != diag.E1132.Def().Code {
					if c.torn {
						continue
					}
					t.Errorf("unexpected finding %s: %s", f.Code, f.Message)
					continue
				}
				line, _ := src.Position(f.Span.Start)
				got = append(got, line)
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("refused strings on lines %v, want %v", got, c.want)
			}
		})
	}
}
