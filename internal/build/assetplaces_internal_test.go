package build

import (
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

const (
	placesIcons   = "icons"
	placesPkg     = "a"
	placesMissing = "missing.png"
	placesTwo     = "two.png"
	placesOne     = "one.png"
	placesUno     = "uno.png"
	placesOther   = "Other"
	placesNested  = "Icons/x.png"
)

// placesStep changes the memo case's icons on disk, and how many E3701 the analysis then reports.
type placesStep struct {
	name    string
	do      func(m roFS)
	missing int
}

// TYPES.md §13.4, API.md S5, IMPLEMENTATION-PLAN §7.6: an asset change flips E3701 warm as cold.
func TestAssetsAskedAgainOnReplay(t *testing.T) {
	z := archiveAnalyzer(t, memoCase)
	m := z.fs.base.(roFS)
	icon := func(name string) string { return strings.TrimPrefix(path.Join(archiveRoot, placesIcons, name), "/") }
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	for _, st := range []placesStep{
		{"unchanged", func(roFS) {}, 1},
		{"missing added", func(m roFS) { m[icon(placesMissing)] = srcFile("") }, 0},
		{"two removed", func(m roFS) { delete(m, icon(placesTwo)) }, 2},
		{"one renamed", func(m roFS) { m[icon(placesUno)] = m[icon(placesOne)]; delete(m, icon(placesOne)) }, 4},
		{"unchanged again", func(roFS) {}, 4},
	} {
		st.do(m)
		warm, cold = z.pair(t)
		same(t, st.name, warm, cold)
		if code := diag.E3701.Def().Code; countCode(warm, code) != st.missing {
			t.Errorf("%s: %d %s, want %d", st.name, countCode(warm, code), code, st.missing)
		}
		if v, _ := replaysOf(warm); v == 0 {
			t.Errorf("%s: stage B replayed no entry, so no asset was asked again", st.name)
		}
		if !listsDir(warm.Reads(placesPkg), path.Join(archiveRoot, placesIcons)) {
			t.Errorf("%s: the icons listing is not in %s's read set", st.name, placesPkg)
		}
	}
}

// countCode is how many findings of code a reports.
func countCode(a *Analysis, code diag.Code) int {
	n := 0
	for _, f := range a.Result().List {
		if f.Code == code {
			n++
		}
	}
	return n
}

// listsDir reports that reads holds the listing of the directory abs.
func listsDir(reads []Read, abs string) bool {
	for _, r := range reads {
		if r.Dir && r.Abs == abs {
			return true
		}
	}
	return false
}

// TYPES.md §13.4, WIRE.md §2.2: a run places a root and a folder once, then uses what it kept.
func TestAssetPlacesKeptPerRun(t *testing.T) {
	tree := ciDir{"assets": ciDir{"Icons": ciDir{"x.png": nil}, placesOther: ciDir{}}}
	p := &project.Project{Roots: []project.Root{{Name: "assets", Path: "assets"}}}
	layout, ok := project.NewLayout(p, "/", nil, diag.NewBag(nil, ""))
	if !ok {
		t.Fatal("layout")
	}
	a := &assets{fs: ciFS(tree), layout: layout, host: &evalHost{}, dirs: map[string]dirListing{}}
	if _, found := a.Exists(benchAssetRoot, benchAssetFrom, placesNested); !found {
		t.Fatal("not found before the placing is kept")
	}
	a.layout = nil
	if display, found := a.Exists(benchAssetRoot, benchAssetFrom, placesNested); !found || display != benchAssetRoot {
		t.Errorf("with the root kept: %q, %t; want %q, true", display, found, benchAssetRoot)
	}
	//canon:unordered one folder is kept
	for k, f := range a.places.folders {
		a.places.folders[k] = project.Path{Display: path.Join(path.Dir(f.Display), placesOther), Abs: path.Join(path.Dir(f.Abs), placesOther)}
	}
	if _, found := a.Exists(benchAssetRoot, benchAssetFrom, placesNested); found {
		t.Error("the kept folder was not the one walked into")
	}
}
