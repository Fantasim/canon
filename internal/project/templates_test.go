package project_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

const templatesProject = `project acme {
  canon: "0.1"
  roots {
    web: "../web"
    admin: "../admin"
  }
  out_templates {
    ts: { web: "web/{dotted}.ts" }
    go: {
      web: "{dir}"
      admin: "internal/gen/{dir}"
    }
    cpp: { admin: "gen/cpp" }
    view: { admin: "views/{dir}/{dotted}.view.json" }
  }
}
`

func loadTemplates(t *testing.T, text string) *project.Project {
	t.Helper()
	set := &source.FileSet{}
	src, err := set.Add(project.FileName, "/p/project.canon", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(set, "")
	p, err := project.Load(src, bag)
	if err != nil {
		t.Fatalf("Load: %v, %d errors", err, bag.ErrorCount())
	}
	return p
}

// GRAMMAR.md §7.1: templates are stored by (target, root), interpolated strings kept.
func TestTemplatesSorted(t *testing.T) {
	p := loadTemplates(t, templatesProject)
	var got []string
	for _, tpl := range p.Templates {
		got = append(got, tpl.Target+"/"+tpl.Root)
	}
	want := []string{"cpp/admin", "go/admin", "go/web", "ts/web", "view/admin"}
	if !slices.Equal(got, want) {
		t.Errorf("Templates = %v, want %v", got, want)
	}
}

// GRAMMAR.md §7.1: {dir} is the package path with /, {dotted} as written, after @<root>/.
func TestTemplateExpand(t *testing.T) {
	p := loadTemplates(t, templatesProject)
	cases := []struct {
		target, root, pkg string
		want              string
		ok                bool
	}{
		{"go", "admin", "items.vocab", "@admin/internal/gen/items/vocab", true},
		{"go", "web", "items.vocab", "@web/items/vocab", true},
		{"go", "admin", "items", "@admin/internal/gen/items", true},
		{"ts", "web", "items.vocab", "@web/web/items.vocab.ts", true},
		{"cpp", "admin", "items.vocab", "@admin/gen/cpp", true},
		{"view", "admin", "a.b.c", "@admin/views/a/b/c/a.b.c.view.json", true},
		{"cpp", "web", "items", "", false},
		{"json", "admin", "items", "", false},
		{"go", "nowhere", "items", "", false},
	}
	for _, c := range cases {
		got, ok := p.Expand(c.target, c.root, c.pkg)
		if got != c.want || ok != c.ok {
			t.Errorf("Expand(%s, %s, %s) = %q, %v, want %q, %v", c.target, c.root, c.pkg, got, ok, c.want, c.ok)
		}
	}
}
