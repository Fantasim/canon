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

// API.md E17 (DECISIONS 330): a package declaring an asset type whose root holds a name the edit
// creates or removes owns it; a name modified, or outside the root, is not its.
func TestOwnersAssetRoot(t *testing.T) {
	files := ownersFS()
	files["law/g/g.canon"] = &fstest.MapFile{Data: []byte("/// G.\npackage g\n\n/// An icon.\ntype Icon = asset(\"../icons\", ext: [png])\n")}
	files["law/icons/a.png"] = &fstest.MapFile{Data: []byte("png")}
	b, err := build.Open(&clockFS{MapFS: files}, "/law", build.Options{})
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
	for _, c := range []struct {
		kind edit.ChangeKind
		path string
		want []string
	}{
		{edit.ChangeCreated, "icons/sub/b.png", []string{"g"}},
		{edit.ChangeDeleted, "icons/a.png", []string{"g"}},
		{edit.ChangeModified, "icons/a.png", nil},
		{edit.ChangeCreated, "iconsx/b.png", nil},
	} {
		plan := &edit.Plan{Changes: []edit.Change{{Kind: c.kind, Path: c.path, After: []byte("png")}}}
		if got, _, err := s.owners(ctx, plan); err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%v %s: owners %v (%v), want %v", c.kind, c.path, got, err, c.want)
		}
	}
}

// API.md E17 (DECISIONS 330): a load.dir base directory that does not exist yet is in its package's
// static read set, so an edit creating it, and a file in it, affects that package.
func TestOwnersMissingGlobBase(t *testing.T) {
	files := ownersFS()
	files["law/f/f.canon"] = &fstest.MapFile{Data: []byte("/// F.\npackage f\n\n/// All.\nlet all: [Int] = load.dir(\"../fresh/*.json\")\n")}
	b, err := build.Open(&clockFS{MapFS: files}, "/law", build.Options{})
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
	plan := &edit.Plan{Changes: []edit.Change{{Kind: edit.ChangeCreated, Path: "fresh/x.json", After: []byte("2\n")}}}
	if got, _, err := s.owners(ctx, plan); err != nil || !slices.Equal(got, []string{"f"}) {
		t.Errorf("creating fresh/x.json: owners %v (%v), want f", got, err)
	}
}

// API.md E17 (log-2026-09-29 M4 P14-r3, DECISIONS 330): a file created in a directory that did not
// exist changes the listing above it too, so a package whose load.dir lists that one owns what the
// edit writes, found from its static read set, no package analysed.
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
	for _, c := range []struct {
		path string
		want []string
	}{
		{"items/new/x.json", []string{"r"}},
		{"items/a/2.json", []string{"r"}},
		{"elsewhere/x.json", nil},
	} {
		plan := &edit.Plan{Changes: []edit.Change{{Kind: edit.ChangeCreated, Path: c.path, After: []byte("2\n")}}}
		if got, _, err := s.owners(ctx, plan); err != nil || !slices.Equal(got, c.want) {
			t.Errorf("creating %s: owners %v, want %v", c.path, got, c.want)
		}
	}
}
