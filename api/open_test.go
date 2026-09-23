package canon_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

func project(files map[string]string) canon.Options {
	m := map[string][]byte{}
	//canon:unordered each file is stored under its own name
	for name, text := range files {
		m["/law/"+name] = []byte(text)
	}
	return canon.Options{FS: newMemFS(m)}
}

// Rule O1: no project.canon above the directory is ErrNoProject, with its E1003 finding.
func TestFindProjectNone(t *testing.T) {
	_, err := canon.FindProject(t.TempDir())
	var perr *canon.ProjectError
	if !errors.Is(err, canon.ErrNoProject) || !errors.As(err, &perr) || len(perr.Findings) != 1 ||
		perr.Findings[0].Code != string(diag.E1003.Def().Code) || perr.Findings[0].File != "" {
		t.Errorf("FindProject: %v", err)
	}
}

// Rules O2, O3, API.md §2.1: Open's errors, each with the sentinel of API.md §15.
func TestOpenErrors(t *testing.T) {
	for _, c := range []struct {
		files map[string]string
		roots map[string]string
		want  error
		code  diag.Code
	}{
		{map[string]string{"project.canon": "project a {\n  canon: \"0.3\"\n}\n"}, nil, canon.ErrUnsupportedVersion, diag.E1001.Def().Code},
		{map[string]string{"project.canon": "project a {\n  canon: \"0.1\"\n  x: 1\n}\n"}, nil, canon.ErrProject, diag.E1002.Def().Code},
		{map[string]string{"project.canon": "project a {\n  canon: \"0.1\"\n}\n"}, map[string]string{"r": "x"}, canon.ErrProject, diag.E7003.Def().Code},
	} {
		opts := project(c.files)
		opts.Roots = c.roots
		_, err := canon.Open("/law", opts)
		var perr *canon.ProjectError
		if !errors.Is(err, c.want) || !errors.As(err, &perr) || perr.Findings[0].Code != string(c.code) {
			t.Errorf("Open: %v", err)
		}
	}
	_, err := canon.Open("/law", project(map[string]string{"Project.canon": "project a {\n  canon: \"0.1\"\n}\n"}))
	var perr *canon.ProjectError
	if !errors.Is(err, canon.ErrNoProject) || !errors.As(err, &perr) || perr.Findings[0].Code != string(diag.E1003.Def().Code) {
		t.Errorf("Open without project.canon: %v", err)
	}
}

// Rules R1, R3, O4, O6, S3: selector and layer errors, the revision, and Close.
func TestCheckErrors(t *testing.T) {
	opts := project(map[string]string{
		"project.canon": "project a {\n  canon: \"0.1\"\n}\n",
		"x/x.canon":     "package x\n\nconst = 1\n",
	})
	opts.Layers = []string{"louis"}
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	before := p.Revision()
	ctx := context.Background()
	if _, err := p.Check(ctx, "y"); !errors.Is(err, canon.ErrUnknownPackage) || err.Error() != "unknown package: y" {
		t.Errorf("unknown package: %v", err)
	}
	if _, err := p.Check(ctx); !errors.Is(err, canon.ErrUnknownLayer) || err.Error() != "unknown layer: louis" {
		t.Errorf("unknown layer: %v", err)
	}
	if _, err := p.Packages(ctx); !errors.Is(err, canon.ErrUnknownLayer) {
		t.Errorf("unknown layer: %v", err)
	}
	opts.Layers = nil
	if p, err = canon.Open("/law", opts); err != nil {
		t.Fatal(err)
	}
	pkgs, err := p.Packages(ctx)
	if err != nil || len(pkgs) != 1 || pkgs[0].Files[0] != "x/x.canon" || p.Revision() != before {
		t.Errorf("Packages: %v, %+v, %s", err, pkgs, p.Revision())
	}
	if p.Close() != nil || p.Close() != nil {
		t.Error("Close is not idempotent")
	}
	if _, err := p.Check(ctx); !errors.Is(err, canon.ErrClosed) {
		t.Errorf("after Close: %v", err)
	}
	if _, err := p.Packages(ctx); !errors.Is(err, canon.ErrClosed) {
		t.Errorf("after Close: %v", err)
	}
}

// Rules R2, F2: a check's findings, located and sorted.
func TestCheckFindings(t *testing.T) {
	p, err := canon.Open("/law", project(map[string]string{
		"project.canon": "project a {\n  canon: \"0.1\"\n}\n",
		"x/x.canon":     "package x\n\nconst = 1\nconst = 2\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), "x")
	if err != nil || !res.HasErrors() || len(res.Findings) != 2 || res.Findings[0].Line != 3 ||
		res.Findings[1].Line != 4 || res.Findings[0].Package != "x" || res.Summary.Packages != 1 {
		t.Errorf("Check: %v, %+v", err, res)
	}
}

// Rules S1, S3, O3: Revision refreshes, and a call after project.canon broke fails like Open.
func TestRefresh(t *testing.T) {
	opts := project(map[string]string{"project.canon": "project a {\n  canon: \"0.1\"\n}\n", "x/x.canon": "package x\n"})
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	before := p.Revision()
	if err := opts.FS.WriteFile("/law/x/x.canon", []byte("package x\n\nconst X = 1\n")); err != nil {
		t.Fatal(err)
	}
	after := p.Revision()
	if after == before || !strings.HasPrefix(string(after), "r1:") {
		t.Errorf("revision %s then %s", before, after)
	}
	if err := opts.FS.WriteFile("/law/project.canon", []byte("project a {\n}\n")); err != nil {
		t.Fatal(err)
	}
	var perr *canon.ProjectError
	if _, err := p.Check(context.Background()); !errors.Is(err, canon.ErrProject) || !errors.As(err, &perr) {
		t.Errorf("Check: %v", err)
	}
	if broken := p.Revision(); broken == after {
		t.Errorf("revision unchanged after project.canon broke: %s", broken)
	}
}
