package canon

import (
	"context"
	"strings"
	"testing"
)

// API.md §2.1 Lang, WIRE.md §10: Options.Lang reaches the build, whose manifest names it; none is the source language.
func TestOptionsLangInManifest(t *testing.T) {
	src := "/// A.\npackage a\n\n/// V.\nlet v: Int = 1\n"
	for _, c := range []struct{ lang, line string }{{"", "lang en\n"}, {"fr", "lang fr\n"}} {
		fsys := newWriteFS(map[string][]byte{"/law/project.canon": []byte(revTestProject), "/law/a/a.canon": []byte(src)}, nil)
		p, err := Open("/law", Options{FS: fsys, Lang: c.lang})
		if err != nil {
			t.Fatal(err)
		}
		a, err := p.b.Analyze(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(a.Manifest()); !strings.Contains(got, c.line) {
			t.Errorf("Lang %q: no line %q in\n%s", c.lang, c.line, got)
		}
	}
}
