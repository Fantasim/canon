package workspace

import (
	"context"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// ownersFS holds r, which lists every JSON file below items/, and s, which reads nothing there.
func ownersFS() fstest.MapFS {
	return fstest.MapFS{
		"law/project.canon":  {Data: []byte("project a {\n  canon: \"0.1\"\n}\n")},
		"law/r/r.canon":      {Data: []byte("/// R.\npackage r\n\n/// All.\nlet all: [Int] = load.dir(\"../items/**/*.json\")\n")},
		"law/s/s.canon":      {Data: []byte("/// S.\npackage s\n\n/// N.\nlet n: Int = 1\n")},
		"law/items/a/1.json": {Data: []byte("1\n")},
	}
}

// API.md E17 (log-2026-09-29 M4 P14-r3): a file created in a directory that did not exist changes
// the listing above it too, so a package whose load.dir lists that one owns what the edit writes.
func TestOwnersNewDirectory(t *testing.T) {
	b, err := build.Open(&clockFS{MapFS: ownersFS()}, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	defer p.Close()
	ctx := context.Background()
	s, err := p.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.analyze(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		path string
		want []string
	}{
		{"items/new/x.json", []string{"r"}},
		{"items/a/2.json", []string{"r"}},
		{"elsewhere/x.json", nil},
	} {
		plan := &edit.Plan{Changes: []edit.Change{{Kind: edit.ChangeCreated, Path: c.path, After: []byte("2\n")}}}
		if got, _ := s.owners(a, plan); !slices.Equal(got, c.want) {
			t.Errorf("creating %s: owners %v, want %v", c.path, got, c.want)
		}
	}
}
