package build_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

const okProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    res: \"res\"\n  }\n}\n"

// API.md O2, O3: Open fails on project.canon's errors and on a bad override, with findings;
// without project.canon it wraps ErrNoProject.
func TestOpenErrors(t *testing.T) {
	for _, c := range []struct {
		name, text string
		roots      map[string]string
		want       error
		findings   int
	}{
		{"invalid", "project acme {\n  canon: 1\n}\n", nil, project.ErrInvalid, 1},
		{"unsupported", "project acme {\n  canon: \"0.7\"\n}\n", nil, project.ErrUnsupportedVersion, 1},
		{"syntax", "project acme {\n  canon: \"0.1\"\n", nil, project.ErrInvalid, 1},
		{"override", okProject, map[string]string{"re": "x", "res": "y"}, project.ErrInvalid, 1},
	} {
		_, err := build.Open(mapFS{"law/project.canon": file(c.text)}, "/law", build.Options{Roots: c.roots})
		var oe *build.OpenError
		if !errors.As(err, &oe) || !errors.Is(err, c.want) || len(oe.Findings.List) != c.findings ||
			oe.Findings.Summary.Errors != c.findings || oe.Findings.Summary.Packages != 0 || oe.Error() != c.want.Error() {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	_, err := build.Open(mapFS{"law/Project.canon": file(okProject)}, "/law", build.Options{})
	var oe *build.OpenError
	if !errors.As(err, &oe) || !errors.Is(err, project.ErrNoProject) || len(oe.Findings.List) != 1 {
		t.Errorf("no project.canon: %v", err)
	}
}

// API.md S1, S3: every call reads project.canon anew, a new error stopping it as it stops Open;
// the revision follows the disk, project.canon broken or gone.
func TestRefresh(t *testing.T) {
	ctx := context.Background()
	fsys := mapFS{"law/project.canon": file(okProject), "law/a/a.canon": file("package a\n")}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	before, err := p.Revision(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fsys["law/a/a.canon"] = file("package a\n\nconst X = 1\n")
	after, err := p.Revision(ctx)
	if err != nil || after == before {
		t.Errorf("revision did not change: %v", err)
	}
	if res, err := p.Check(ctx, nil); err != nil || res.Revision != after {
		t.Errorf("Check revision %v, %v", res, err)
	}
	fsys["law/project.canon"] = file("project acme {\n  canon: \"0.5\"\n}\n")
	var oe *build.OpenError
	if _, err := p.Check(ctx, nil); !errors.As(err, &oe) || !errors.Is(err, project.ErrUnsupportedVersion) {
		t.Errorf("Check after project.canon broke: %v", err)
	}
	broken, err := p.Revision(ctx)
	if err != nil || broken == after {
		t.Errorf("Revision after project.canon broke: %s, %v", broken, err)
	}
	delete(fsys, "law/a/a.canon")
	if gone, err := p.Revision(ctx); err != nil || gone == broken {
		t.Errorf("Revision after a.canon went: %s, %v", gone, err)
	}
	delete(fsys, "law/project.canon")
	if none, err := p.Revision(ctx); err != nil || none == broken || !strings.HasPrefix(none, "r1:") {
		t.Errorf("Revision without project.canon: %s, %v", none, err)
	}
}

func openTree(t *testing.T, opt build.Options) *build.Project {
	t.Helper()
	fsys := mapFS{
		"law/project.canon":         file("/// Doc.\n\nproject acme {\n  canon: \"0.1\"\n  studio: studio\n}\n"),
		"law/a/a.canon":             file("package a\n\nconst = 1\nconst = 2\n"),
		"law/a/knights.layer.canon": file("package a\nlayer knights\n"),
		"law/b/b.canon":             file("package b\n"),
	}
	p, err := build.Open(fsys, "/law", opt)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// API.md R1, R2, F7, F8, O4, S3: selection, the project's own findings (W1001, E1012) in every
// result, truncation, layers and the revision.
func TestCheck(t *testing.T) {
	ctx := context.Background()
	p := openTree(t, build.Options{MaxFindings: 1, Layers: []string{"knights"}})
	res, err := p.Check(ctx, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	s := res.Summary
	if len(res.List) != 3 || s.Errors != 3 || s.Warnings != 1 || s.Packages != 1 || len(s.Truncated) != 1 ||
		s.Truncated[0].Package != "a" || strings.Join(res.Packages, ",") != "a" {
		t.Errorf("check a: %d findings, summary %+v", len(res.List), s)
	}
	if !regexp.MustCompile(`^r1:[0-9a-f]{64}$`).MatchString(res.Revision) {
		t.Errorf("revision %q", res.Revision)
	}
	again, err := p.Check(ctx, nil)
	if err != nil || again.Revision != res.Revision || again.Summary.Packages != 2 {
		t.Errorf("check all: %v, %+v", err, again)
	}
	if _, err := p.Check(ctx, []string{"c"}); !errors.Is(err, project.ErrUnknownPackage) {
		t.Errorf("unknown package: %v", err)
	}
	if _, err := p.Check(ctx, []string{"b"}); !errors.Is(err, build.ErrUnknownLayer) {
		t.Errorf("layer of an unloaded package: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := p.Check(cancelled, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
}

// API.md §5.5, O4: every package, with its imports and layers; an unknown layer fails.
func TestPackages(t *testing.T) {
	units, err := openTree(t, build.Options{Layers: []string{"knights"}}).Packages(context.Background())
	if err != nil || len(units.Units) != 2 || units.Units[0].Layers[0] != "knights" || units.Revision == "" {
		t.Errorf("Packages: %v, %+v", err, units)
	}
	if _, err := openTree(t, build.Options{Layers: []string{"louis"}}).Packages(context.Background()); !errors.Is(err, build.ErrUnknownLayer) {
		t.Errorf("unknown layer: %v", err)
	}
}
