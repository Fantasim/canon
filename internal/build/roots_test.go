package build_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

const (
	rootsProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    src: \"../Source\"\n  }\n}\n"
	rootsLocal   = "project acme {\n  roots {\n    src: \"../work/Source\"\n  }\n}\n"
	rootsSource  = "/// A.\npackage a\n"
)

// API.md O3 (DECISIONS 332): an absent required root outside the
// project, and an error of project.local.canon, stop Open with the project's findings.
func TestOpenRootErrors(t *testing.T) {
	for _, c := range []struct {
		name  string
		fsys  mapFS
		codes []diag.Code
	}{
		{"absent", mapFS{"p/project.canon": file(rootsProject)}, []diag.Code{diag.E1013.Def().Code}},
		{"local absent", mapFS{"p/project.canon": file(rootsProject), "p/project.local.canon": file(rootsLocal), "Source/x": file("")},
			[]diag.Code{diag.E1013.Def().Code}},
		{"local key", mapFS{"p/project.canon": file(rootsProject), "Source/x": file(""),
			"p/project.local.canon": file("project acme {\n  canon: \"0.1\"\n}\n")}, []diag.Code{diag.E1014.Def().Code}},
		{"local syntax", mapFS{"p/project.canon": file(rootsProject), "Source/x": file(""),
			"p/project.local.canon": file("project acme {\n")}, nil},
	} {
		_, err := build.Open(c.fsys, "/p", build.Options{})
		var oe *build.OpenError
		if !errors.As(err, &oe) || !errors.Is(err, project.ErrInvalid) || len(oe.Findings.List) == 0 {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := codesOf(oe.Findings.List); c.codes != nil && !slices.Equal(got, c.codes) {
			t.Errorf("%s: codes %v, want %v", c.name, got, c.codes)
		}
	}
}

func codesOf(list []diag.Finding) []diag.Code {
	out := make([]diag.Code, len(list))
	for i, f := range list {
		out[i] = f.Code
	}
	return out
}

// API.md O2, S3 (DECISIONS 332): project.local.canon places a root and is never a
// source; it is in the revision and in Inputs when it exists, and editing it changes the revision.
func TestLocalRootFile(t *testing.T) {
	ctx := context.Background()
	fsys := mapFS{
		"p/project.canon": file(rootsProject), "p/project.local.canon": file(rootsLocal),
		"work/Source/x": file(""), "p/a/a.canon": file(rootsSource),
		"p/a/project.local.canon": file("/// B.\npackage a\n"), // elsewhere, an ordinary source (GRAMMAR.md §5.2)
	}
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	units, err := p.Packages(ctx)
	if err != nil || len(units.Units) != 1 || len(units.Units[0].Files) != 2 {
		t.Fatalf("Packages: %v, %+v", err, units)
	}
	if abs, ok := p.Abs("@src/x"); !ok || abs != "/work/Source/x" {
		t.Errorf("Abs(@src/x) = %s, %v", abs, ok)
	}
	if res, err := p.Check(ctx, nil); err != nil || res.Summary.Errors != 0 {
		t.Errorf("Check: %v, %+v", err, res)
	}
	reads, err := p.Inputs()
	if err != nil || !slices.ContainsFunc(reads, func(r build.Read) bool { return r.Display == project.LocalFileName }) {
		t.Errorf("Inputs: %v, %v", err, reads)
	}
	assertInputsRevision(t, p, fsys)
	before, _ := p.Revision(ctx)
	fsys["p/project.local.canon"] = file(rootsLocal + "// moved\n")
	if after, _ := p.Revision(ctx); after == before {
		t.Error("editing project.local.canon left the revision unchanged")
	}
}

// assertInputsRevision checks that the existing Inputs give p's Revision (API.md S3).
func assertInputsRevision(t *testing.T, p *build.Project, fsys mapFS) {
	t.Helper()
	reads, err := p.Inputs()
	if err != nil {
		t.Fatal(err)
	}
	var lines []build.Listed
	for _, r := range reads {
		if data, err := fsys.ReadFile(r.Abs); err == nil {
			lines = append(lines, build.Listed{Display: r.Display, Sum: sha256.Sum256(data)})
		}
	}
	if want, err := p.Revision(context.Background()); err != nil || build.RevisionOf(lines) != want {
		t.Errorf("RevisionOf(Inputs) = %s, Revision %s (%v)", build.RevisionOf(lines), want, err)
	}
}

// API.md O2 (DECISIONS 332): Options.Roots wins over project.local.canon, which wins
// over project.canon, root by root.
func TestRootPrecedence(t *testing.T) {
	const twoRoots = "project acme {\n  canon: \"0.1\"\n  roots {\n    src: \"../Source\"\n    web: \"../web\"\n  }\n}\n"
	fsys := mapFS{
		"p/project.canon":       file(twoRoots),
		"p/project.local.canon": file("project acme {\n  roots {\n    src: \"../work/Source\"\n    web: \"/elsewhere/web\"\n  }\n}\n"),
		"work/Source/x":         file(""), "opt/web/x": file(""),
	}
	p, err := build.Open(fsys, "/p", build.Options{Roots: map[string]string{"web": "/opt/web"}})
	if err != nil {
		t.Fatal(err)
	}
	//canon:unordered each path is resolved on its own
	for written, want := range map[string]string{"@src/x": "/work/Source/x", "@web/x": "/opt/web/x"} {
		if abs, ok := p.Abs(written); !ok || abs != want {
			t.Errorf("Abs(%s) = %s, %v, want %s", written, abs, ok, want)
		}
	}
}
