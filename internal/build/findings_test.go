package build_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"golang.org/x/tools/txtar"
)

const (
	findingsFile = "findings.txt"
	buildFile    = "build.txt"
	layersFile   = "layers"     // the active layers, in order (EVALUATION.md §9.1)
	adoptFile    = "adopt"      // display paths the build may take over (API.md B2)
	selectFile   = "select"     // the selectors (API.md R1); none selects every package
	checkOnly    = "check-only" // present: the case is analysed, never built
)

// archiveFS is a case's files as a project under /p, its expected output and options left out.
func archiveFS(a *txtar.Archive) mapFS {
	fsys := mapFS{}
	for _, f := range a.Files {
		switch f.Name {
		case findingsFile, buildFile, layersFile, adoptFile, selectFile, checkOnly:
		default:
			fsys["p/"+f.Name] = file(string(f.Data))
		}
	}
	return fsys
}

func fields(a *txtar.Archive, name string) []string {
	for _, f := range a.Files {
		if f.Name == name {
			return strings.Fields(string(f.Data))
		}
	}
	return nil
}

func buildCase(t *testing.T, a *txtar.Archive) *build.BuildResult {
	t.Helper()
	p, err := build.Open(archiveFS(a), "/p", build.Options{Layers: fields(a, layersFile)})
	if err != nil {
		t.Fatal(err)
	}
	selectors := fields(a, selectFile)
	if _, only := archived(a, checkOnly); only {
		return checkCase(t, p, selectors)
	}
	opt := build.BuildOptions{Packages: selectors, Adopt: fields(a, adoptFile)}
	res, err := p.Build(context.Background(), opt)
	var oe *build.OpenError
	if errors.As(err, &oe) { // a refusal with findings: E1901 (EVALUATION.md §9.1)
		return &build.BuildResult{Result: build.Result{Findings: oe.Findings}}
	}
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// checkCase is check-only's build.BuildResult: phases 1-7 alone (CLI.md §3.3), no write.
func checkCase(t *testing.T, p *build.Project, selectors []string) *build.BuildResult {
	t.Helper()
	res, err := p.Check(context.Background(), selectors)
	var oe *build.OpenError
	if errors.As(err, &oe) {
		return &build.BuildResult{Result: build.Result{Findings: oe.Findings}}
	}
	if err != nil {
		t.Fatal(err)
	}
	return &build.BuildResult{Result: *res}
}

// IMPLEMENTATION-PLAN.md §7.2: each case builds a project and prints the findings of build's codes.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		return []byte(render(t, buildCase(t, c.Archive).Findings))
	}, golden.Expected(findingsFile))
}

// CLI.md §3.4, EVALUATION.md §1: each case prints a build's findings, outputs and locks.
func TestBuilds(t *testing.T) {
	golden.Run(t, "testdata/builds/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		res := buildCase(t, c.Archive)
		var b strings.Builder
		b.WriteString(render(t, res.Findings))
		for _, o := range res.Outputs {
			fmt.Fprintf(&b, "output %s %d\n", o.Path, o.Status)
		}
		for _, l := range res.Locks {
			fmt.Fprintf(&b, "lock %s %d %q\n", l.Path, l.Status, l.Lines)
		}
		return []byte(b.String())
	}, golden.Expected(buildFile))
}
