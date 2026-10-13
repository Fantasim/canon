package build_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

const (
	orphanProject = "project acme {\n  canon: \"0.1\"\n}\n"
	pinnedProject = "project acme {\n  canon: \"0.1\"\n  compiler: \"0.1.5\"\n}\n"
	stray         = "record R { a: Int\n"
)

func orphanTree() mapFS {
	return mapFS{
		"p/project.canon": file(orphanProject),
		"p/a/a.canon":     file("package a\n\nconst X = 1\n"),
		"p/stray/s.canon": file(stray),
		"p/top.canon":     file("const Y = 2\n"),
		"p/b/b.canon":     file("const Z = 3\n"),
		"p/b/c.canon":     file("package a\n\nconst = 4\n"), // a package line: its own package's finding
	}
}

// CLI.md §2.2, DECISIONS 353: package-less files count against their directory or the root, as no package.
func TestOrphanBags(t *testing.T) {
	p, err := build.Open(orphanTree(), "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, c := range []struct {
		name     string
		selector []string
		errors   int // E1127 and the like: the findings of a bag, a package-less file has two
		packages int
	}{
		{"all", nil, 6, 1},
		{"package a, whose files hold the package line's finding", []string{"a"}, 2, 1},
		{"name", []string{"stray"}, 2, 0},
		{"below", []string{"stray..."}, 2, 0},
		{"directory", []string{"./stray"}, 2, 0},
		{"file", []string{"stray/s.canon"}, 2, 0},
		{"root file", []string{"top.canon"}, 1, 0},
		{"a package and an orphan", []string{"a", "stray"}, 4, 1},
		{"directory of a package and an orphan file", []string{"./b"}, 3, 1},
	} {
		res, err := p.Check(ctx, c.selector)
		if err != nil || res.Summary.Errors != c.errors || res.Summary.Packages != c.packages {
			t.Errorf("%s: %v, %+v", c.name, err, res.Summary)
		}
	}
	if _, err := p.Check(ctx, []string{"strays"}); !errors.Is(err, project.ErrUnknownPackage) {
		t.Errorf("an unknown name: %v", err)
	}
	units, err := p.Packages(ctx)
	if err != nil || len(units.Units) != 1 || units.Units[0].Name != "a" {
		t.Errorf("Packages: %v, %+v", err, units)
	}
}

// DECISIONS 353: a build of a package other than the orphan's is clean.
func TestOrphanBuildScoped(t *testing.T) {
	fsys := orphanTree()
	delete(fsys, "p/b/c.canon")
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"a"}})
	if err != nil || res.Summary.Errors != 0 || res.Summary.Packages != 1 {
		t.Errorf("build a: %v, %+v", err, res)
	}
}

// DECISIONS 354: the compiler is the link-time version, else dev; a pin that differs is E1015 in the
// project's own bag, Open still succeeding; a dev compiler is W1016 instead.
func TestCompilerPin(t *testing.T) {
	defer func(old string) { build.CompilerVersion = old }(build.CompilerVersion)
	tree := mapFS{"p/project.canon": file(pinnedProject), "p/a/a.canon": file("package a\n")}
	for _, c := range []struct {
		running          string
		errors, warnings int
	}{
		{"0.1.5", 0, 0},
		{"0.1.4", 1, 0},
		{"", 0, 1},
	} {
		build.CompilerVersion = c.running
		p, err := build.Open(tree, "/p", build.Options{})
		if err != nil {
			t.Fatalf("%q: Open: %v", c.running, err)
		}
		res, err := p.Check(context.Background(), nil)
		if err != nil || res.Summary.Errors != c.errors || res.Summary.Warnings != c.warnings || res.Summary.Packages != 1 {
			t.Errorf("%q: %v, %+v", c.running, err, res.Summary)
		}
	}
}

// DECISIONS 354: the build manifest carries the same version string the compiler reports.
func TestManifestCompiler(t *testing.T) {
	defer func(old string) { build.CompilerVersion = old }(build.CompilerVersion)
	for _, stamped := range []string{"0.1.5", ""} {
		build.CompilerVersion = stamped
		p, err := build.Open(mapFS{"p/project.canon": file(orphanProject), "p/a/a.canon": file("package a\n")}, "/p", build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Build(context.Background(), build.BuildOptions{})
		if err != nil || !strings.Contains(string(res.Manifest), "\ncompiler "+build.Compiler()+"\n") {
			t.Errorf("%q: %v\n%s", stamped, err, res.Manifest)
		}
	}
}

// DECISIONS 353: an orphan bag's findings sort among the packages' by the bag's name; a package-less
// file in a package's directory is selected by its own path, and a bare ... names nothing.
func TestOrphanOrder(t *testing.T) {
	p, err := build.Open(mapFS{
		"p/project.canon": file(orphanProject),
		"p/c/c.canon":     file("package c\n\nconst = 3\n"),
		"p/b/b.canon":     file("const Z = 2\n"),
		"p/a/a.canon":     file("package a\n\nconst = 1\n"),
		"p/a/loose.canon": file("const Q = 1\n"),
		"p/top.canon":     file("const T = 1\n"),
	}, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	res, err := p.Check(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, f := range res.List {
		order = append(order, f.Package)
	}
	if got := strings.Join(slices.Compact(order), " "); got != ". a b c" {
		t.Errorf("findings by package: %q", got)
	}
	whole, _ := p.Check(ctx, []string{"a"})
	loose, err := p.Check(ctx, []string{"a/loose.canon"})
	if err != nil || loose.Summary.Packages != 1 || loose.Summary.Errors != whole.Summary.Errors {
		t.Errorf("a loose file's path: %v, %+v", err, loose.Summary)
	}
	if _, err := p.Check(ctx, []string{"..."}); !errors.Is(err, project.ErrUnknownPackage) {
		t.Errorf("a bare ...: %v", err)
	}
	if root, err := p.Check(ctx, []string{"."}); err != nil || root.Summary.Errors != 1 || root.Summary.Packages != 0 {
		t.Errorf("the root directory: %v, %+v", err, root.Summary)
	}
}
