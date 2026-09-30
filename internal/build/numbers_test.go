package build_test

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

// numbersFS reads data/x.json through two roots over one directory, as a Float and, second, as
// the given type.
func numbersFS(second string) mapFS {
	return mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n\n  roots {\n    a: \"data\"\n    b: \"data\"\n  }\n}\n"),
		"law/d/d.canon":     file("package d\n\nlet wide: Float = load(\"@a/x.json\", at: \"x\")\nlet again: " + second + " = load(\"@b/x.json\", at: \"x\")\n"),
		"law/data/x.json":   file("{\"x\": 1.0}\n"),
	}
}

// flipFS serves law/data/x.json's bytes once, then other: a file changing between two loads.
type flipFS struct {
	mapFS
	other []byte
	mu    sync.Mutex
	reads int
}

func (f *flipFS) ReadFile(name string) ([]byte, error) {
	if rel(name) != "law/data/x.json" {
		return f.mapFS.ReadFile(name)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reads++; f.reads > 1 {
		return f.other, nil
	}
	return f.mapFS.ReadFile(name)
}

// xText is the text NumberTexts gives the token of x.json at display, "" for none.
func xText(t *testing.T, fsys project.FS, display string) string {
	t.Helper()
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"d"})
	if err != nil {
		t.Fatal(err)
	}
	content, texts := a.NumberTexts(display)
	for span, text := range texts { //canon:unordered one number
		if string(content[span.Start:span.End]) == "1.0" {
			return text
		}
	}
	return ""
}

// FORMATTER.md 14.1, API.md M9 (log-2026-09-29 M4 B11-r3, B11-r5): typed numbers are judged per
// file, so two roots over one directory read one file: a token read as Float and Float32 through
// them is kept, by NumberTexts (M9) and by JSONSources (fmt) alike; read as one type, canonical.
func TestNumbersPerFile(t *testing.T) {
	for _, c := range []struct {
		second, want string
	}{
		{"Float32", "1.0"},
		{"Float", "1"},
	} {
		for _, display := range []string{"@a/x.json", "@b/x.json"} {
			if got := xText(t, numbersFS(c.second), display); got != c.want {
				t.Errorf("NumberTexts(%s) with %s: %q, want %q", display, c.second, got, c.want)
			}
		}
		p, err := build.Open(numbersFS(c.second), "/law", build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		srcs, err := p.JSONSources(context.Background(), []string{"d/d.canon"})
		if err != nil {
			t.Fatal(err)
		}
		kept := !slices.ContainsFunc(srcs, func(s build.JSONSource) bool { return len(s.Numbers) > 0 })
		if kept != (c.want == "1.0") {
			t.Errorf("JSONSources with %s: %+v", c.second, srcs)
		}
	}
}

// FORMATTER.md 14.1, API.md M9 (log-2026-09-29 M4 B11-r5): two loads that read different contents
// of one file give no typed reading, in NumberTexts and JSONSources alike: numbers stay as written.
func TestNumbersMixedContents(t *testing.T) {
	other := []byte("{\"x\": 2.0}\n")
	if got := xText(t, &flipFS{mapFS: numbersFS("Float"), other: other}, "@a/x.json"); got != "" {
		t.Errorf("NumberTexts: %q, want none", got)
	}
	fsys := &flipFS{mapFS: numbersFS("Float"), other: other}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	srcs, err := p.JSONSources(context.Background(), []string{"d/d.canon"})
	if err != nil {
		t.Fatal(err)
	}
	if fsys.reads < 2 || len(srcs) == 0 {
		t.Fatalf("x.json read %d times, %d sources: want two contents", fsys.reads, len(srcs))
	}
	for _, s := range srcs {
		if s.Content != nil || s.Numbers != nil {
			t.Errorf("%s: %q %+v, want no reading", s.Display, s.Content, s.Numbers)
		}
	}
}
