package build_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/syntax"
	viewrender "github.com/fantasim/canonlang/internal/views/render"
)

const readsProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    data: \"data\"\n  }\n}\n"

// readsFS is a project of three packages: a loads a file of its own directory, b imports a, and
// c loads a file through a root.
func readsFS() mapFS {
	return mapFS{
		"p/project.canon": file(readsProject),
		"p/a/a.canon":     file("/// A.\npackage a\n\n/// Xs.\nlet xs: [Int] = load(\"a.json\")\n"),
		"p/a/a.json":      file("[1, 2]\n"),
		"p/b/b.canon":     file("/// B.\npackage b\n\nimport a\n\n/// N.\nlet n: Int = 1 + 2\n"),
		"p/c/c.canon":     file("/// C.\npackage c\n\n/// Ys.\nlet ys: [Int] = load(\"@data/c.json\")\n"),
		"p/data/c.json":   file("[3]\n"),
	}
}

// recordingFS is a file system that is told what each load read (build.ReadRecorder).
type recordingFS struct {
	mapFS
	mu    sync.Mutex
	reads []build.Read
}

var _ build.ReadRecorder = (*recordingFS)(nil)

func (r *recordingFS) RecordReads(reads []build.Read) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads = append(r.reads, reads...)
}

func displaysOf(reads []build.Read) []string {
	var out []string
	for _, r := range reads {
		if !r.Dir {
			out = append(out, r.Display)
		}
	}
	return out
}

// API.md S5: a package's read set is its own files, locks and loads and its imports', never an
// unrelated package's; project.canon and the place of project.local.canon count for every one.
func TestAnalysisReads(t *testing.T) {
	p, err := build.Open(readsFS(), "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil || a.Result().Summary.Errors != 0 {
		t.Fatalf("Analyze: %v, %+v", err, a.Result().List)
	}
	for _, c := range []struct {
		pkg  string
		want []string
	}{
		{"a", []string{"a/a.canon", "a/a.json", "a/canon.lock", "project.canon", "project.local.canon"}},
		{"b", []string{"a/a.canon", "a/a.json", "a/canon.lock", "b/b.canon", "b/canon.lock", "project.canon", "project.local.canon"}},
		{"c", []string{"@data/c.json", "c/c.canon", "c/canon.lock", "project.canon", "project.local.canon"}},
		{"nope", nil},
	} {
		if got := displaysOf(a.Reads(c.pkg)); !slices.Equal(got, c.want) {
			t.Errorf("Reads(%s) = %q, want %q", c.pkg, got, c.want)
		}
	}
	if r := a.Reads("c"); !slices.Contains(r, build.Read{Display: "@data/c.json", Abs: "/p/data/c.json"}) {
		t.Errorf("Reads(c) names c.json %v", r)
	}
	for _, dir := range []string{"a", "b"} { // a source added there after a base is a change
		if r := a.Reads("b"); !slices.Contains(r, build.Read{Display: dir, Abs: "/p/" + dir, Dir: true, Sources: true}) {
			t.Errorf("Reads(b) lists no directory %s: %v", dir, r)
		}
	}
}

// API.md S3: a file system that records reads is told each file a load read, with its display
// path, by Analyze and by Build alike.
func TestReadRecorder(t *testing.T) {
	for _, run := range []func(*build.Project) error{
		func(p *build.Project) error { _, err := p.Analyze(context.Background(), nil); return err },
		func(p *build.Project) error {
			_, err := p.Build(context.Background(), build.BuildOptions{Check: true})
			return err
		},
	} {
		fsys := &recordingFS{mapFS: readsFS()}
		p, err := build.Open(fsys, "/p", build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err := run(p); err != nil {
			t.Fatal(err)
		}
		want := []build.Read{{Display: "a/a.json", Abs: "/p/a/a.json"}, {Display: "@data/c.json", Abs: "/p/data/c.json"}}
		if !slices.Equal(fsys.reads, want) {
			t.Errorf("recorded %v, want %v", fsys.reads, want)
		}
	}
}

// API.md S3: Inputs and RevisionOf give the revision a build reads of the same files.
func TestInputsRevision(t *testing.T) {
	fsys := readsFS()
	fsys["p/a/canon.lock"] = file("canon-lock v1\n")
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	reads, err := p.Inputs()
	if err != nil {
		t.Fatal(err)
	}
	var lines []build.Listed
	for _, r := range reads {
		data, err := fsys.ReadFile(r.Abs)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		lines = append(lines, build.Listed{Display: r.Display, Sum: sha256.Sum256(data)})
	}
	want, err := p.Revision(context.Background())
	if err != nil || build.RevisionOf(lines) != want || len(lines) != 5 {
		t.Errorf("RevisionOf %d lines = %s, Revision %s (%v)", len(lines), build.RevisionOf(lines), want, err)
	}
	if unread := build.RevisionOf(append(lines, build.Listed{Display: "x", Unreadable: true})); unread == want {
		t.Error("an unreadable line leaves the revision unchanged")
	}
}

// API.md §3.4, WIRE.md §2.3: Over, Abs and Display.
func TestOverAbsDisplay(t *testing.T) {
	p, err := build.Open(readsFS(), "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	other := readsFS()
	delete(other, "p/c/c.canon")
	units, err := p.Over(other).Packages(context.Background())
	if err != nil || len(units.Units) != 2 || p.Over(other).FS() == nil {
		t.Errorf("Over: %v, %d units", err, len(units.Units))
	}
	for _, c := range []struct {
		file, abs string
		ok        bool
	}{
		{"a/a.canon", "/p/a/a.canon", true},
		{"@data/c.json", "/p/data/c.json", true},
		{"/elsewhere/x", "/elsewhere/x", true},
		{"../x", "/x", false},
		{"@nope/x", "", false},
	} {
		if abs, ok := p.Abs(c.file); abs != c.abs || ok != c.ok {
			t.Errorf("Abs(%s) = %s, %v", c.file, abs, ok)
		}
	}
	for _, c := range [][2]string{{"/p/a/a.canon", "a/a.canon"}, {"/p", "."}, {"/elsewhere/x", "x"}} {
		if got := p.Display(c[0]); got != c[1] {
			t.Errorf("Display(%s) = %s, want %s", c[0], got, c[1])
		}
	}
}

// analyzeSubtitle is package a of fsys analyzed alone, and the expression of its view's subtitle.
func analyzeSubtitle(t *testing.T, fsys mapFS) (*build.Analysis, syntax.Expr) {
	t.Helper()
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	var subtitle syntax.Expr
	for _, pkg := range a.Program().Packages {
		for _, f := range pkg.Files {
			syntax.Inspect(f, func(n syntax.Node) bool {
				if s, ok := n.(*syntax.ViewSubtitle); ok {
					subtitle = s.Text
				}
				return true
			})
		}
	}
	return a, subtitle
}

// API.md V11, V13: ViewEvaluator evaluates a view's template on the frozen analysis, no finding added.
func TestViewEvaluator(t *testing.T) {
	a, subtitle := analyzeSubtitle(t, mapFS{"p/project.canon": file(viewProject), "p/a/a.canon": file(viewA), "p/b/b.canon": file(viewB)})
	before := len(a.Bag("a").Findings())
	v, ok := a.ViewEvaluator().Eval(context.Background(), subtitle, nil, viewrender.Magic{})
	if !ok || v.CanonText() != "B" || len(a.Bag("a").Findings()) != before || a.ViewErr() != nil {
		t.Errorf("Eval = %q, %v; %d findings, %d before; %v", v.CanonText(), ok, len(a.Bag("a").Findings()), before, a.ViewErr())
	}
}

// API.md V11, X2: a template that fails is only false; an unsupported load it meets is kept as
// the analysis's ViewErr, never swallowed.
func TestViewEvaluatorKeepsFailures(t *testing.T) {
	failing := viewFS()
	failing["p/a/a.canon"] = file(strings.Replace(viewA, `subtitle "{b.label}"`, `subtitle "{b.unread}"`, 1))
	a, subtitle := analyzeSubtitle(t, failing)
	if _, ok := a.ViewEvaluator().Eval(context.Background(), subtitle, nil, viewrender.Magic{}); ok || a.ViewErr() != nil {
		t.Errorf("a failing template: ok %v, ViewErr %v", ok, a.ViewErr())
	}
	loading := viewFS()
	loading["p/b/b.canon"] = file(viewB + "\n/// Loaded.\nlet loaded: [Int] = load.dir(\"x.txt\")\n")
	loading["p/b/x.txt"] = file("1")
	loading["p/a/a.canon"] = file(strings.Replace(viewA, `subtitle "{b.label}"`, `subtitle "{b.loaded}"`, 1))
	a, subtitle = analyzeSubtitle(t, loading)
	if _, ok := a.ViewEvaluator().Eval(context.Background(), subtitle, nil, viewrender.Magic{}); ok || !errors.Is(a.ViewErr(), build.ErrLoad) {
		t.Errorf("an unsupported load: ok %v, ViewErr %v", ok, a.ViewErr())
	}
}
