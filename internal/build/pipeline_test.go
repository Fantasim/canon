package build_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
)

const examplesDir = "../../examples"

// openExamples opens examples/ from disk with every root redirected (examples/_fixtures/README.md):
// the read roots to their fixtures, the written ones to out. Its builds run in Check mode.
func openExamples(t *testing.T, out string) *build.Project {
	t.Helper()
	return openExamplesLayered(t, out, nil)
}

// openExamplesLayered is openExamples with layers active (EVALUATION.md §9.1).
func openExamplesLayered(t *testing.T, out string, layers []string) *build.Project {
	t.Helper()
	dir, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	roots := map[string]string{"resource": "_fixtures/resource", "client": "_fixtures/client"}
	for _, name := range []string{"source", "services", "sovcommon", "web", "parity", "generated"} {
		roots[name] = filepath.ToSlash(filepath.Join(out, name))
	}
	p, err := build.Open(project.OS(), filepath.ToSlash(dir), build.Options{Roots: roots, Layers: layers})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func render(t *testing.T, f build.Findings) string {
	t.Helper()
	var buf bytes.Buffer
	if err := diag.Render(&buf, f.Files, f.List, diag.RenderOptions{Summary: f.Summary, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func teamboardGolden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(examplesDir, "teamboard", "expected", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

var goAndJSON = []ir.Target{ir.TargetGo, ir.TargetJSON}

// IMPLEMENTATION-PLAN §6 M1 items 2 and 5, LOCK.md §9.1: the outputs are listed, not compared.
func TestTeamboard(t *testing.T) {
	p := openExamples(t, t.TempDir())
	checked, err := p.Check(context.Background(), []string{"teamboard"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := render(t, checked.Findings), teamboardGolden(t, "findings.txt"); got != want {
		t.Errorf("findings:\n%s\nwant\n%s", got, want)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"teamboard"}, Targets: goAndJSON, Check: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Stale || len(res.Locks) != 1 || res.Locks[0].Path != "teamboard/canon.lock" || res.Locks[0].Status != build.StatusStale ||
		string(res.Locks[0].Content) != teamboardGolden(t, "canon.lock") || len(res.Locks[0].Lines) != strings.Count(teamboardGolden(t, "canon.lock"), "\n")-1 {
		t.Errorf("locks: %+v", res.Locks)
	}
	var paths []string
	for _, o := range res.Outputs {
		paths = append(paths, o.Path)
		if o.Package != "teamboard" || o.Status != build.StatusStale || !strings.HasPrefix(o.Abs, "/") {
			t.Errorf("output %+v", o)
		}
	}
	for _, want := range []string{"@sovcommon/teamboard/teamboard.gen.go", "@sovcommon/teamboard/rt/rt.go", "@sovcommon/teamboard/data/statuses.json"} {
		if !slices.Contains(paths, want) {
			t.Errorf("no output %s in %v", want, paths)
		}
	}
	if !slices.IsSorted(paths) {
		t.Errorf("outputs not in byte order: %v", paths)
	}
}

// CLI.md §3.3, EVALUATION.md §1: Analyze reports what Build reports, and writes nothing.
func TestAnalyzeMatchesBuild(t *testing.T) {
	p := openExamples(t, t.TempDir())
	sel := []string{"teamboard", "sovcommon..."}
	analyzed, err := p.Analyze(context.Background(), sel)
	if err != nil {
		t.Fatal(err)
	}
	a := analyzed.Result()
	b, err := p.Build(context.Background(), build.BuildOptions{Packages: sel, Targets: goAndJSON, Check: true})
	if err != nil {
		t.Fatal(err)
	}
	if render(t, a.Findings) != render(t, b.Findings) || !slices.Equal(a.Packages, b.Packages) || a.Revision != b.Revision {
		t.Errorf("analyze %v, build %v", a.Packages, b.Packages)
	}
}

// IMPLEMENTATION-PLAN §7.5: two builds give the same bytes.
func TestDeterminism(t *testing.T) {
	var runs [2]*build.BuildResult
	for i := range runs {
		p := openExamples(t, t.TempDir())
		res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"teamboard", "sovcommon..."}, Targets: goAndJSON, Check: true})
		if err != nil {
			t.Fatal(err)
		}
		runs[i] = res
	}
	if render(t, runs[0].Findings) != render(t, runs[1].Findings) || !reflect.DeepEqual(runs[0].Locks, runs[1].Locks) {
		t.Error("findings or locks differ")
	}
	if len(runs[0].Outputs) != len(runs[1].Outputs) || len(runs[0].Outputs) == 0 {
		t.Fatalf("%d and %d outputs", len(runs[0].Outputs), len(runs[1].Outputs))
	}
	for i, o := range runs[0].Outputs {
		if q := runs[1].Outputs[i]; o.Path != q.Path || !bytes.Equal(o.Content, q.Content) {
			t.Errorf("%s differs from %s", o.Path, q.Path)
		}
	}
}
