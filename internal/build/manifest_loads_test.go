package build_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"golang.org/x/tools/txtar"
)

const outsideProject = "project acme {\n  canon: \"0.1\"\n\n  roots {\n    resource: \"../res\"\n  }\n}\n"

// WIRE.md §10: the glob line hashes the matches the run used; a file added after Analyze is not in its manifest.
func TestManifestGlobAsUsed(t *testing.T) {
	fsys, layers := manifestCaseFS(t)
	opt := build.Options{Layers: layers}
	want := checkManifest(t, fsys, opt)
	a := analyzed(t, fsys, opt)
	fsys["p/res/items/c.json"] = file(`{"name": "c", "icon": "a.png"}`)
	if got := string(a.Manifest()); got != want {
		t.Errorf("a file added after Analyze changed its manifest:\n%s", got)
	}
}

// WIRE.md §10, §2.3: load, load.dir, load.csv, load.text and load.defines files under a root outside the project, by their written displays.
func TestManifestLoadForms(t *testing.T) {
	a, err := txtar.ParseFile("testdata/incremental/loadforms.txtar")
	if err != nil {
		t.Fatal(err)
	}
	fsys := mapFS{"w/p/project.canon": file(outsideProject)}
	for _, f := range a.Files {
		if rest, ok := strings.CutPrefix(f.Name, "resource/"); ok {
			fsys["w/res/"+rest] = file(string(f.Data))
		} else if f.Name != "project.canon" {
			fsys["w/p/"+f.Name] = file(string(f.Data))
		}
	}
	p, err := build.Open(fsys, "/w/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	checked, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := string(checked.Manifest())
	for _, line := range []string{
		"root resource ../res\n", " @resource/codes.h\n", " @resource/rows.csv\n", " @resource/note.txt\n",
		" @resource/cfg.json\n", " @resource/items/i1.json\n", " @resource/items/*.json\n",
	} {
		if !strings.Contains(got, line) {
			t.Errorf("no %q in\n%s", line, got)
		}
	}
	if strings.Contains(got, " codes.h\n") || strings.Contains(got, " rows.csv\n") {
		t.Errorf("a loaded file lost its written display:\n%s", got)
	}
}
