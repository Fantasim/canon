package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// rootAt is a root declared at offset at of project.canon.
func rootAt(name, dir string, at source.Pos) project.Root {
	return project.Root{Name: name, Path: dir, Span: source.Span{Start: at, End: at + 1}}
}

// CODEGEN.md §2.8, DECISIONS 229, 269.
func TestOwningRoot(t *testing.T) {
	p := project.New("acme", project.Version{Major: 0, Minor: 1})
	p.Roots = []project.Root{ // name order, as project.Load leaves them
		rootAt("alias", "../../shared", 40),
		rootAt("gen", "gen", 50),
		rootAt("services", "../..", 10),
		rootAt("shared", "../../shared", 30),
		rootAt("web", "../../shared/web", 20),
	}
	for _, c := range []struct{ dir, want string }{
		{"../../shared/web/src", "web"},
		{"../../shared/web", "web"},
		{"../../shared/x", "shared"}, // shared declared before alias, same directory
		{"../../shared", "shared"},
		{"../../other", "services"},
		{"gen/go/a", "gen"},
		{"a/out", ""},      // no root holds it: the project
		{"../outside", ""}, // above the project, under no root
		{"general/x", ""},  // a string prefix of gen, not a child of it
		{"../../../x", ""}, // walks out of services
		{".", ""},          // the project directory itself
	} {
		if got := check.OwningRoot(p, c.dir); got != c.want {
			t.Errorf("OwningRoot(%q) = %q, want %q", c.dir, got, c.want)
		}
	}
	if got := check.RootLabel(p, ""); got != "project acme" {
		t.Errorf("RootLabel of the project = %q", got)
	}
	if got := check.RootLabel(p, "web"); got != "web" {
		t.Errorf("RootLabel of a root = %q", got)
	}
}

// CODEGEN.md §2.8 (E8007, owning roots): a directory that only shares a root's path as a string prefix, or walks back out through "..", is not under that root.
func TestWithin(t *testing.T) {
	for _, c := range []struct {
		dir, root, wantRel string
		wantOK             bool
	}{
		{"services/pipeline", "services", "pipeline", true},
		{"services", "services", "", true},
		{"services/../other", "services", "../other", false},
		{"servicesx/a", "services", "servicesx/a", false},
		{"..", ".", "..", false},
		{"../other", ".", "../other", false},
		{"other", ".", "other", true},
		{"../../../x", "../..", "../x", false},
		{"../../x", "../..", "x", true},
	} {
		if rel, ok := check.Within(c.dir, c.root); rel != c.wantRel || ok != c.wantOK {
			t.Errorf("Within(%q, %q) = %q, %v; want %q, %v", c.dir, c.root, rel, ok, c.wantRel, c.wantOK)
		}
	}
}
