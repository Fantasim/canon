package check_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
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

// CODEGEN.md §2.1, DECISIONS 424: the path an out resolves to includes its root's own path from project.canon; a bad character there is E8009 outChars, naming that path.
func TestOutCharsRootPath(t *testing.T) {
	p := project.New("acme", project.Version{Major: 0, Minor: 1})
	p.Roots = []project.Root{{Name: "gen", Path: "gen"}, {Name: "odd", Path: "od\"d??="}}
	for _, tc := range []struct{ out, resolved string }{
		{out: "@gen/a"},
		{out: "@odd/a", resolved: "od\"d??=/a"}, // the project-relative path the message names
	} {
		set := &source.FileSet{}
		src, err := set.Add("a/a.canon", "/a/a.canon", []byte("package a\n\nemit cpp { out: \""+tc.out+"\", namespace: \"a\" }\n"))
		if err != nil {
			t.Fatal(err)
		}
		bags := check.Bags{"a": diag.NewBag(set, "a")}
		f := syntax.Parse(src, syntax.FileSource, bags["a"])
		check.Check(context.Background(), p, []*syntax.File{f}, bags, literalFolder{})
		found := bags["a"].Findings()
		switch {
		case tc.resolved == "" && len(found) != 0:
			t.Errorf("out %q: findings %v, want none", tc.out, found)
		case tc.resolved != "" && (len(found) != 1 || found[0].Code != diag.E8009.Def().Code || !strings.Contains(found[0].Message, tc.resolved)):
			t.Errorf("out %q: findings %v, want one outChars finding naming %q", tc.out, found, tc.resolved)
		}
	}
}

// CODEGEN.md §2.1, DECISIONS 424: without a project nothing resolves, so an out is judged as written: a backslash is E8009 outChars.
func TestOutCharsWithoutProject(t *testing.T) {
	set := &source.FileSet{}
	src, err := set.Add("a/a.canon", "/a/a.canon", []byte("package a\n\nemit cpp { out: \"gen\\\\a\", namespace: \"a\" }\n"))
	if err != nil {
		t.Fatal(err)
	}
	bags := check.Bags{"a": diag.NewBag(set, "a")}
	f := syntax.Parse(src, syntax.FileSource, bags["a"])
	check.Check(context.Background(), nil, []*syntax.File{f}, bags, literalFolder{})
	if got, want := errorCodes(bags["a"].Findings()), []diag.Code{diag.E8009.Def().Code}; !slices.Equal(got, want) {
		t.Errorf("codes %v, want %v", got, want)
	}
}
