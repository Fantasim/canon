package build_test

import (
	"context"
	"io/fs"
	"maps"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"golang.org/x/tools/txtar"
)

const (
	manifestFile = "manifest.txt"
	manifestCase = "testdata/manifest/inputs.txtar"
	shuffleSeed  = 11
)

// manifestFS is a manifest case's project, its expected output and options left out.
func manifestFS(a *txtar.Archive) mapFS {
	fsys := archiveFS(a)
	delete(fsys, "p/"+manifestFile)
	return fsys
}

// manifestCaseFS reads the manifest case's project.
func manifestCaseFS(t *testing.T) (mapFS, []string) {
	t.Helper()
	a, err := txtar.ParseFile(manifestCase)
	if err != nil {
		t.Fatal(err)
	}
	return manifestFS(a), fields(a, layersFile)
}

// analyzed is a check of fsys's project, findings or not.
func analyzed(t *testing.T, fsys project.FS, opt build.Options) *build.Analysis {
	t.Helper()
	p, err := build.Open(fsys, "/p", opt)
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// checkManifest is the manifest of a check of fsys's project, which must hold no error.
func checkManifest(t *testing.T, fsys project.FS, opt build.Options) string {
	t.Helper()
	a := analyzed(t, fsys, opt)
	if res := a.Result(); res.Summary.Errors != 0 {
		t.Fatalf("check: %s", render(t, res.Findings))
	}
	return string(a.Manifest())
}

// WIRE.md §10 (LOD-11), §2.3: the exact manifests of check, build --target json and test.
func TestManifestGolden(t *testing.T) {
	golden.Run(t, "testdata/manifest/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		fsys, opt := manifestFS(c.Archive), build.Options{Layers: fields(c.Archive, layersFile)}
		var b strings.Builder
		b.WriteString(checkManifest(t, fsys, opt))
		p, err := build.Open(fsys, "/p", opt)
		if err != nil {
			t.Fatal(err)
		}
		built, err := p.Build(context.Background(), build.BuildOptions{Targets: []ir.Target{ir.TargetJSON}, Check: true})
		if err != nil {
			t.Fatal(err)
		}
		b.Write(built.Manifest)
		tested, err := p.Test(context.Background(), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(tested.Manifest)
		return []byte(b.String())
	}, golden.Expected(manifestFile))
}

// API.md O7, WIRE.md §10: Layers and Roots are part of the manifest; Lang names the language.
func TestManifestOptions(t *testing.T) {
	fsys, layers := manifestCaseFS(t)
	fsys["p/copy/items/a.json"] = file(`{"name": "a", "icon": "a.png"}`)
	fsys["p/copy/items/b.json"] = file(`{"name": "b", "icon": "a.png"}`)
	fsys["p/copy/icons/a.png"] = file("")
	fsys["gen/.keep"] = file("") // SPEC §3.1: the root moved outside the project exists
	//canon:unordered each file copied alone: the same bytes under another directory
	for name, f := range maps.Clone(fsys) {
		if rest, ok := strings.CutPrefix(name, "p/res/"); ok {
			fsys["p/same/"+rest] = f
		}
	}
	base := checkManifest(t, fsys, build.Options{Layers: layers})
	for _, c := range []struct {
		name string
		opt  build.Options
		line string // a line only the changed manifest has
		only bool   // that line is the only difference
	}{
		{"no layer", build.Options{}, "lang en\n", false},
		{"root moved", build.Options{Layers: layers, Roots: map[string]string{"res": "copy"}}, "root res copy\n", false},
		{"root moved to the same bytes", build.Options{Layers: layers, Roots: map[string]string{"res": "same"}}, "root res same\n", true},
		{"emit root moved", build.Options{Layers: layers, Roots: map[string]string{"out": "../gen"}}, "root out ../gen\n", true},
		{"lang", build.Options{Layers: layers, Lang: "fr"}, "lang fr\n", true},
	} {
		got := checkManifest(t, fsys, c.opt)
		if got == base {
			t.Errorf("%s: the manifest did not change", c.name)
		}
		if !strings.Contains(got, c.line) {
			t.Errorf("%s: no line %q in\n%s", c.name, c.line, got)
		}
		if diff := lineDiff(base, got); c.only && len(diff) != 1 {
			t.Errorf("%s: lines differ beyond %q: %q", c.name, c.line, diff)
		}
	}
	if !strings.Contains(base, "layer dev\nlang en\n") || !strings.Contains(base, "root out out\nroot res res\n") {
		t.Errorf("the layer, the source language or the roots are not listed:\n%s", base)
	}
}

// lineDiff is the lines of got that want lacks.
func lineDiff(want, got string) []string {
	var out []string
	for _, l := range strings.Split(got, "\n") {
		if !slices.Contains(strings.Split(want, "\n"), l) {
			out = append(out, l)
		}
	}
	return out
}

// WIRE.md §10: a file matching a glob changes its glob line; a file no run reads changes nothing.
func TestManifestInputs(t *testing.T) {
	fsys, layers := manifestCaseFS(t)
	opt := build.Options{Layers: layers}
	base := checkManifest(t, fsys, opt)
	globLine := regexp.MustCompile(`(?m)^glob [0-9a-f]{64} @res/items/\*\.json$`).FindString(base)
	if globLine == "" {
		t.Fatalf("no glob line in\n%s", base)
	}
	for _, c := range []struct {
		name    string
		do      func(mapFS)
		changed bool
	}{
		{"unread file touched", func(m mapFS) { m["p/res/unread.json"] = file("[1]") }, false},
		{"unmatched file added", func(m mapFS) { m["p/res/items/more.txt"] = file("x") }, false},
		{"unlisted icon added", func(m mapFS) { m["p/res/icons/unused/d.png"] = file("") }, false},
		{"matching file added", func(m mapFS) { m["p/res/items/c.json"] = file(`{"name": "c", "icon": "a.png"}`) }, true},
		{"icon added", func(m mapFS) { m["p/res/icons/e.png"] = file("") }, true},
		{"loaded file edited", func(m mapFS) { m["p/items/data/conf.json"] = file(`{"limit": 5}`) }, true},
		{"asset name becomes a folder", func(m mapFS) { // E3701 then: the listing must tell
			delete(m, "p/res/icons/a.png")
			m["p/res/icons/a.png/x"] = file("")
		}, true},
	} {
		m := maps.Clone(fsys)
		c.do(m)
		got := string(analyzed(t, m, opt).Manifest())
		if (got != base) != c.changed {
			t.Errorf("%s: changed %t, want %t:\n%s", c.name, got != base, c.changed, got)
		}
	}
	m := maps.Clone(fsys)
	m["p/res/items/c.json"] = file(`{"name": "c", "icon": "a.png"}`)
	if strings.Contains(checkManifest(t, m, opt), globLine) {
		t.Errorf("a matching file added left the glob line %q", globLine)
	}
}

// shuffledFS lists every directory in an order of its own (DOCTRINE §5).
type shuffledFS struct {
	mapFS
	rnd *rand.Rand
}

func (s shuffledFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := s.mapFS.ReadDir(name)
	s.rnd.Shuffle(len(entries), func(i, j int) { entries[i], entries[j] = entries[j], entries[i] })
	return entries, err
}

// WIRE.md §10, DOCTRINE §5: the manifest's bytes do not depend on the order a listing comes in.
func TestManifestDeterministic(t *testing.T) {
	fsys, layers := manifestCaseFS(t)
	opt := build.Options{Layers: layers}
	want := checkManifest(t, fsys, opt)
	for i := range 4 {
		got := checkManifest(t, shuffledFS{mapFS: fsys, rnd: rand.New(rand.NewPCG(shuffleSeed, uint64(i)))}, opt)
		if got != want {
			t.Fatalf("shuffle %d: the manifest differs:\n%s\nwant\n%s", i, got, want)
		}
	}
	lines := strings.Split(strings.TrimSuffix(want, "\n"), "\n")
	for _, kind := range []string{"file ", "glob ", "list "} {
		var paths []string
		for _, l := range lines {
			if strings.HasPrefix(l, kind) {
				paths = append(paths, l[strings.LastIndexByte(l, ' ')+1:])
			}
		}
		if len(paths) == 0 || !slices.IsSorted(paths) {
			t.Errorf("%s lines missing or not sorted by path: %q", kind, paths)
		}
	}
}
