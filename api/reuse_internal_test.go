package canon

import (
	"bytes"
	"context"
	"maps"
	"slices"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// reuseLaw is a reading a imports b, c imports a, u stands apart: an analysis of a and its
// imports and the every-package one differ in what they load, not in what they give a.
func reuseLaw() map[string][]byte {
	return map[string][]byte{
		"/law/project.canon": []byte("project acme {\n  canon: \"0.1\"\n}\n"),
		"/law/b/b.canon": []byte("/// B.\npackage b\n\n/// An item.\nrecord Item {\n  /// Its name.\n  name: String\n}\n\n" +
			"/// Items.\nlet items: table Item = {\n  sword { name: \"Sword\" }\n  shield { name: \"Shield\" }\n}\n"),
		"/law/a/a.canon": []byte("/// A.\npackage a\n\nimport b\n\n/// A move.\nvariant Move {\n  /// Walks.\n  walk {\n" +
			"    /// How far.\n    far: Int = 1\n  }\n  /// Runs.\n  run {\n    /// How far.\n    far: Int = 1\n" +
			"    /// How fast.\n    fast: Int = 2\n  }\n}\n\n/// A use.\nrecord Use {\n  /// The item.\n  item: ref b.items\n" +
			"  /// How many.\n  count: Int\n  /// How it moves.\n  move: Move\n}\n\n/// Uses.\nlet uses: table Use = {\n" +
			"  first { item: sword, count: 20, move: walk { far: 3 } }\n  second { item: shield, count: 3, move: run { far: 1 } }\n}\n\n" +
			"view Use {\n  title \"{id}: {count}\"\n}\n\nemit view { out: \"a.view.json\" }\n"),
		"/law/c/c.canon": []byte("/// C.\npackage c\n\nimport a\n\n/// More uses.\nlet more: [a.Use] = [{ item: sword, count: 1, move: walk { far: 2 } }]\n"),
		"/law/u/u.canon": []byte("/// U.\npackage u\n\n/// K.\nlet k: Int = 1\n"),
	}
}

// checkLog records each phase 2 a project runs and the files it loads.
type checkLog struct {
	mu   sync.Mutex
	runs [][]string
}

func (l *checkLog) check(ctx context.Context, proj *project.Project, files []*syntax.File, bags map[string]*diag.Bag) *check.Program {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Src.Path
	}
	l.mu.Lock()
	l.runs = append(l.runs, names)
	l.mu.Unlock()
	return check.Check(ctx, proj, files, bags, eval.NewFolder(bags, eval.Options{}))
}

func (l *checkLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.runs)
}

// loaded reports a run since the first from that loaded file.
func (l *checkLog) loaded(from int, file string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.ContainsFunc(l.runs[from:], func(run []string) bool { return slices.Contains(run, file) })
}

// logged opens /law on fsys with layers active, each phase 2 recorded.
func logged(t *testing.T, fsys *writeFS, layers ...string) (*Project, *checkLog) {
	t.Helper()
	l := &checkLog{}
	b, err := build.Open(fsys, "/law", build.Options{Checker: l.check, Layers: layers})
	if err != nil {
		t.Fatal(err)
	}
	p := &Project{root: "/law", b: b, layers: layers}
	t.Cleanup(func() { _ = p.Close() })
	return p, l
}

// opened opens /law on a copy of files.
func opened(t *testing.T, files map[string][]byte) *Project {
	t.Helper()
	p, err := Open("/law", Options{FS: newWriteFS(maps.Clone(files), nil)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// API.md W12, V13: starting a watch checks every package as Value and Evaluate read them, one
// analysis for all three.
func TestWatchSeedIsValuesAnalysis(t *testing.T) {
	p, l := logged(t, newWriteFS(reuseLaw(), nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := p.Watch(ctx, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	seeded := l.count()
	if _, err := p.Value(ctx, "a:uses.first.count"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Evaluate(ctx, EvalRequest{Path: "a:uses.first"}); err != nil {
		t.Fatal(err)
	}
	if seeded != 1 || l.count() != seeded {
		t.Errorf("W12, V13: %d checks to seed the watch, %d after Value and Evaluate, want 1", seeded, l.count())
	}
}

// viewBytes is pkg's view model on p.
func viewBytes(t *testing.T, p *Project, pkg string) []byte {
	t.Helper()
	m, err := p.ViewModel(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	return m.JSON()
}

// keepWhole has p keep its every-package analysis, as Value does, and returns it.
func keepWhole(t *testing.T, p *Project) *build.Analysis {
	t.Helper()
	s, err := p.read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	a, err := analyze(context.Background(), s, nil)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// orderLaw is a's checked record with a warning instance and an emit, and b reading a's value
// with emits of its own: b's instance-check, emit-rule findings must not follow what was kept.
func orderLaw() map[string][]byte {
	return map[string][]byte{
		"/law/project.canon": []byte("project acme {\n  canon: \"0.1\"\n}\n"),
		"/law/a/a.canon": []byte("/// A.\npackage a\n\n/// R.\nrecord R {\n  /// N.\n  n: Int\n\n  warn n > 5 else \"small\"\n}\n\n" +
			"/// The r.\nlet r: R = { n: 1 }\n\nemit go { out: \"@gen/x\" }\n"),
		"/law/b/b.canon": []byte("/// B.\npackage b\n\nimport a\n\n/// X.\nlet x: a.R = a.r\n\nemit go { out: \"@gen/x\" }\n\n" +
			"emit view { out: \"b.view.json\" }\n"),
	}
}

// API.md R9, S8, VIEWMODEL.md J15: a view model is the analysis of the package and its imports,
// the same bytes whether or not every package's analysis was kept before it.
func TestViewModelIgnoresWholeAnalysis(t *testing.T) {
	for _, c := range []struct {
		files map[string][]byte
		pkg   string
	}{{reuseLaw(), "a"}, {orderLaw(), "b"}} {
		files, pkg := c.files, c.pkg
		want := viewBytes(t, opened(t, files), pkg)
		kept := opened(t, files)
		keepWhole(t, kept)
		if got := viewBytes(t, kept, pkg); !bytes.Equal(got, want) {
			t.Errorf("R9: %s's model follows the kept analysis:\n%s\nwant\n%s", pkg, got, want)
		}
	}
}
