package lint_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/lint"
)

const (
	projectSource = "project acme {\n  canon: \"0.1\"\n}\n"
	manyFields    = 1001 // one past the findings a package keeps (API.md F7)
	keptFindings  = 1000
)

// runOn lints the packages named in selectors of the project fsys holds under /p.
func runOn(t *testing.T, fsys mapFS, selectors ...string) (*build.Analysis, *lint.Result) {
	t.Helper()
	fsys["p/project.canon"] = file(projectSource)
	l := analyzed(t, fsys, selectors)
	return l.a, l.res
}

// API.md L1: lint returns the findings of the selected packages and no other.
func TestRunSelection(t *testing.T) {
	fsys := mapFS{
		"p/a/a.canon": file("package a\n\nrecord R {\n  dwA: Int\n}\n"),
		"p/b/b.canon": file("package b\n\nrecord R {\n  dwB: Int\n}\n"),
	}
	for _, c := range []struct {
		selectors []string
		want      []string
	}{
		{[]string{"a"}, []string{"a"}},
		{[]string{"b"}, []string{"b"}},
		{nil, []string{"a", "b"}},
	} {
		_, res := runOn(t, fsys, c.selectors...)
		var got []string
		for _, f := range res.Findings {
			got = append(got, f.Package)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("select %v: findings of %v, want %v", c.selectors, got, c.want)
		}
	}
}

// API.md L1: a broken declaration is never judged, and the rest of its package still is.
func TestRunBrokenDeclaration(t *testing.T) {
	fsys := mapFS{"p/a/a.canon": file("package a\n\nrecord Broken {\n  dwA: Nowhere\n}\n\nrecord Sound {\n  dwB: Int\n}\n")}
	_, res := runOn(t, fsys)
	if len(res.Findings) != 1 || !strings.Contains(res.Findings[0].Message, "dwB") {
		t.Fatalf("findings = %v, want the one of dwB", res.Findings)
	}
}

// API.md L11: two calls on one analysis return the same result, the fixes in finding order.
func TestRunDeterministic(t *testing.T) {
	fsys := mapFS{
		"p/a/a.canon": file("package a\n\nrecord R {\n  dwA: Int\n  szB: String\n}\n"),
		"p/b/b.canon": file("package b\n\nrecord R {\n  dwC: Int\n}\n"),
	}
	l := analyzed(t, withProject(fsys), nil)
	first := l.res
	second, err := lint.Run(context.Background(), l.a, edit.NewSnapshot(l.a))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("two calls differ:\n%v\n%v", first, second)
	}
	if len(first.Fixes) != len(first.Findings) {
		t.Fatalf("%d fixes for %d findings", len(first.Fixes), len(first.Findings))
	}
	for i, f := range first.Fixes {
		if f.Span != first.Findings[i].Span {
			t.Errorf("fix %d is at %v, its finding at %v", i, f.Span, first.Findings[i].Span)
		}
	}
}

func withProject(fsys mapFS) mapFS {
	fsys["p/project.canon"] = file(projectSource)
	return fsys
}

// API.md L1 and F7: a package keeps its first findings in F2 order, and the summary counts the rest.
func TestRunTruncates(t *testing.T) {
	var src strings.Builder
	src.WriteString("package a\n\nrecord R {\n")
	for i := range manyFields {
		fmt.Fprintf(&src, "  dwF%dx: Int\n", i)
	}
	src.WriteString("}\n")
	_, res := runOn(t, mapFS{"p/a/a.canon": file(src.String())})
	if len(res.Findings) != keptFindings || res.Summary.Warnings != manyFields {
		t.Fatalf("kept %d findings, counted %d warnings", len(res.Findings), res.Summary.Warnings)
	}
	if want := []diag.Truncation{{Package: "a", Warnings: manyFields - keptFindings}}; !reflect.DeepEqual(res.Summary.Truncated, want) {
		t.Errorf("truncated = %v, want %v", res.Summary.Truncated, want)
	}
}

// API.md L1 and F7: a positive Options.MaxFindings sets how many findings a package keeps, the rest are counted; any other value keeps the default, as check does.
func TestRunMaxFindings(t *testing.T) {
	src := "package a\n\nrecord R {\n  dwA: Int\n  dwB: Int\n  dwC: Int\n  dwD: Int\n  dwE: Int\n}\n"
	for _, c := range []struct {
		limit, kept int
		truncated   []diag.Truncation
	}{
		{2, 2, []diag.Truncation{{Package: "a", Warnings: 3}}},
		{0, 5, nil},
		{-1, 5, nil},
	} {
		l := analyzedWith(t, withProject(mapFS{"p/a/a.canon": file(src)}), nil, build.Options{MaxFindings: c.limit})
		if len(l.res.Findings) != c.kept || l.res.Summary.Warnings != 5 {
			t.Errorf("limit %d: kept %d findings, counted %d warnings; want %d and 5", c.limit, len(l.res.Findings), l.res.Summary.Warnings, c.kept)
		}
		if !reflect.DeepEqual(l.res.Summary.Truncated, c.truncated) {
			t.Errorf("limit %d: truncated = %v, want %v", c.limit, l.res.Summary.Truncated, c.truncated)
		}
	}
}

// API.md L1: a stopped context stops the lint with its error.
func TestRunCancelled(t *testing.T) {
	fsys := withProject(mapFS{"p/a/a.canon": file("package a\n\nrecord R {\n  dwA: Int\n}\n")})
	l := analyzed(t, fsys, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := lint.Run(ctx, l.a, edit.NewSnapshot(l.a)); !errors.Is(err, context.Canceled) {
		t.Errorf("Run on a stopped context = %v, want context.Canceled", err)
	}
}
