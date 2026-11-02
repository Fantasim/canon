package canon_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// lintOrderLaw holds two packages whose files sort in the other order than their names: a.b's
// file, a/b/b.canon, comes before a's, a/z.canon (API.md F2).
var lintOrderLaw = map[string][]byte{
	"/law/project.canon": []byte("project acme {\n  canon: \"0.1\"\n}\n"),
	"/law/a/z.canon":     []byte("package a\n\nrecord R {\n  dwA: Int\n}\n"),
	"/law/a/b/b.canon":   []byte("package a.b\n\nrecord R {\n  szB: String\n}\n"),
}

func openLint(t *testing.T, files map[string][]byte) *canon.Project {
	t.Helper()
	p, err := canon.Open("/law", canon.Options{FS: newMemFS(files), Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// API.md L11, F2: the findings of every package are in F2 order, and the fixes follow them,
// whatever order the packages were linted in.
func TestLintFixesFollowFindings(t *testing.T) {
	res, err := openLint(t, lintOrderLaw).Lint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var files, fixes []string
	for _, f := range res.Findings {
		files = append(files, f.File)
	}
	for _, f := range res.Fixes {
		fixes = append(fixes, f.File)
		if f.Edit.Base != res.Revision || len(f.Edit.Ops) != 1 || f.Edit.Ops[0].Kind != canon.OpRenameName {
			t.Errorf("fix %+v: want one renameName based on %s", f, res.Revision)
		}
	}
	want := []string{"a/b/b.canon", "a/z.canon"}
	if !reflect.DeepEqual(files, want) || !reflect.DeepEqual(fixes, want) {
		t.Errorf("findings in %v, fixes in %v, want both %v", files, fixes, want)
	}
	if res.Summary.Warnings != len(want) || res.Summary.Errors != 0 || res.Summary.Packages != len(want) {
		t.Errorf("summary %+v, want %d warnings in %d packages", res.Summary, len(want), len(want))
	}
}

// API.md L1: Lint reports W0001-W0008 only; a project's own findings stay Check's, and an
// unknown package is ErrUnknownPackage, as for every read.
func TestLintOnlyLints(t *testing.T) {
	files := map[string][]byte{
		"/law/project.canon": lintOrderLaw["/law/project.canon"],
		"/law/a/a.canon":     []byte("package a\n\nrecord R {\n  dwA: Nowhere\n}\n\nrecord S {\n  dwB: Int\n}\n"),
	}
	p := openLint(t, files)
	res, err := p.Lint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Line != 8 || res.Findings[0].Severity != canon.SeverityWarning {
		t.Errorf("findings %+v, want the one lint warning of the sound record", res.Findings)
	}
	if res.Summary.Errors != 0 || len(res.Fixes) != 0 {
		t.Errorf("summary %+v with %d fixes, want no error and no fix in a broken package (E32)", res.Summary, len(res.Fixes))
	}
	if _, err := p.Lint(context.Background(), "nowhere"); !errors.Is(err, canon.ErrUnknownPackage) {
		t.Errorf("Lint of an unknown package = %v, want ErrUnknownPackage", err)
	}
}

// API.md L11, S8: concurrent calls on one snapshot return the same result.
func TestLintConcurrent(t *testing.T) {
	p := openLint(t, lintOrderLaw)
	const calls = 4
	results := make([]*canon.LintResult, calls)
	errs := make([]error, calls)
	var wg sync.WaitGroup
	for i := range calls {
		wg.Go(func() { results[i], errs[i] = p.Lint(context.Background()) })
	}
	wg.Wait()
	for i := range calls {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if !reflect.DeepEqual(results[i], results[0]) {
			t.Errorf("call %d = %+v, want %+v", i, results[i], results[0])
		}
	}
}

// API.md S11: a cancelled context makes Lint return its error.
func TestLintCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openLint(t, lintOrderLaw).Lint(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Lint cancelled = %v, want context.Canceled", err)
	}
}
