package project_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"golang.org/x/tools/txtar"
)

const (
	libFile    = "lib/deep/x.canon"   // clean
	uiFile     = "shared/ui/ui.canon" // a syntax error, in its package's bag
	readers    = 8
	rounds     = 25
	libChanged = "package lib\n\nconst Y = 2\n"
	uiChanged  = "package shared.ui\n\nconst = 2\n"
	libPackage = "lib"
	uiPackage  = "shared.ui"
)

// pkgTree is a parsed file and the package of the unit holding it.
type pkgTree struct {
	pkg  string
	file *syntax.File
}

// warmProject is a project read through a Reuse and a persistent file set, snapshot by snapshot.
type warmProject struct {
	fsys  memFS
	names []string
	set   *source.FileSet
	reuse *project.Reuse
}

func newWarmProject(t *testing.T) *warmProject {
	t.Helper()
	fsys := newMemFS(txtar.Parse([]byte(tree)))
	names, err := project.Scan(fsys, projectDir)
	if err != nil {
		t.Fatal(err)
	}
	set := &source.FileSet{}
	return &warmProject{fsys: fsys, names: names, set: set, reuse: project.NewReuse(set)}
}

// readIn is one snapshot of fsys: a reader with new bags over the persistent set and store.
func (w *warmProject) readIn(fsys memFS, names []string) ([]*project.Unit, map[string]*diag.Bag, error) {
	bags := map[string]*diag.Bag{}
	var mu sync.Mutex
	bagOf := func(pkg string) *diag.Bag {
		mu.Lock()
		defer mu.Unlock()
		if bags[pkg] == nil {
			bags[pkg] = diag.NewBag(w.set, pkg)
		}
		return bags[pkg]
	}
	r := &project.Reader{FS: fsys, Dir: projectDir, Set: w.set, BagOf: bagOf, Reuse: w.reuse}
	units, err := r.Parse(context.Background(), names)
	return units, bags, err
}

func (w *warmProject) read(t *testing.T, names []string) ([]*project.Unit, map[string]*diag.Bag) {
	t.Helper()
	units, bags, err := w.readIn(w.fsys, names)
	if err != nil {
		t.Fatal(err)
	}
	return units, bags
}

// coldIn is the findings of fsys read without the hook, into a set of its own.
func coldIn(fsys memFS, names []string) (string, error) {
	set := &source.FileSet{}
	bags := map[string]*diag.Bag{}
	bagOf := func(pkg string) *diag.Bag {
		if bags[pkg] == nil {
			bags[pkg] = diag.NewBag(set, pkg)
		}
		return bags[pkg]
	}
	r := &project.Reader{FS: fsys, Dir: projectDir, Set: set, BagOf: bagOf}
	if _, err := r.Parse(context.Background(), names); err != nil {
		return "", err
	}
	return renderFindings(set, bags)
}

func (w *warmProject) cold(t *testing.T, names []string) string {
	t.Helper()
	out, err := coldIn(w.fsys, names)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// renderFindings is the text and JSON forms of the findings of every bag, in package order.
func renderFindings(set *source.FileSet, bags map[string]*diag.Bag) (string, error) {
	var buf bytes.Buffer
	for _, pkg := range slices.Sorted(maps.Keys(bags)) {
		bag := bags[pkg]
		fmt.Fprintf(&buf, "== %q\n", pkg)
		opt := diag.RenderOptions{Summary: bag.Summary(), Golden: true}
		if err := diag.Render(&buf, set, bag.Findings(), opt); err != nil {
			return "", err
		}
		for _, l := range diag.Locate(set, bag.Findings()) {
			buf.Write(l.AppendJSON(nil))
			buf.WriteByte('\n')
		}
	}
	return buf.String(), nil
}

func findingsText(t *testing.T, set *source.FileSet, bags map[string]*diag.Bag) string {
	t.Helper()
	out, err := renderFindings(set, bags)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// memPath is the key of a project file in a memFS.
func memPath(name string) string { return path.Join(projectDir[1:], name) }

func trees(units []*project.Unit) []pkgTree {
	var out []pkgTree
	for _, u := range units {
		for _, f := range u.Files {
			out = append(out, pkgTree{pkg: u.Name, file: f})
		}
	}
	return out
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: unchanged files, erroneous ones too, come back as the same tree.
func TestReuseReturnsSameTrees(t *testing.T) {
	w := newWarmProject(t)
	first, _ := w.read(t, w.names)
	want := trees(first)
	second, _ := w.read(t, w.names)
	got := trees(second)
	if len(got) != len(want) || len(got) == 0 {
		t.Fatalf("%d trees, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].file != want[i].file {
			t.Errorf("tree %d was parsed again", i)
		}
	}
	if w.set.File(source.FileID(len(w.names)+1)) != nil {
		t.Error("a reused read added files to the set")
	}
}

// NFR-02: the findings of a reused read, text and JSON, are those of a cold read.
func TestReuseReplaysFindings(t *testing.T) {
	w := newWarmProject(t)
	_, firstBags := w.read(t, w.names)
	_, warmBags := w.read(t, w.names)
	want := w.cold(t, w.names)
	if !strings.Contains(want, uiFile) {
		t.Fatal("the cold read has no findings")
	}
	if got := findingsText(t, w.set, firstBags); got != want {
		t.Errorf("first read:\n%s\nwant:\n%s", got, want)
	}
	if got := findingsText(t, w.set, warmBags); got != want {
		t.Errorf("reused read:\n%s\nwant:\n%s", got, want)
	}
}

// NFR-02: after an edit only the changed file is parsed again, and its findings are fresh.
func TestReuseParsesChangedFileOnly(t *testing.T) {
	w := newWarmProject(t)
	first, _ := w.read(t, w.names)
	before := trees(first)
	w.fsys.m[memPath(libFile)] = &fstest.MapFile{Data: []byte(libChanged)}
	w.fsys.m[memPath(uiFile)] = &fstest.MapFile{Data: []byte(uiChanged)}
	second, warmBags := w.read(t, w.names)
	after := trees(second)
	var changed []string
	for i := range before {
		if before[i].file != after[i].file {
			changed = append(changed, after[i].pkg)
		}
	}
	slices.Sort(changed)
	if want := []string{libPackage, uiPackage}; !slices.Equal(changed, want) {
		t.Errorf("parsed again: %q, want %q", changed, want)
	}
	if got, want := findingsText(t, w.set, warmBags), w.cold(t, w.names); got != want {
		t.Errorf("findings after the edit:\n%s\nwant:\n%s", got, want)
	}
}

// NFR-02: a deleted file leaves the project and its entry is dropped; this pins a memory policy, not correctness.
func TestReuseDoesNotResurrect(t *testing.T) {
	w := newWarmProject(t)
	first, _ := w.read(t, w.names)
	old := trees(first)
	without := slices.DeleteFunc(slices.Clone(w.names), func(n string) bool { return n == libFile })
	units, _ := w.read(t, without)
	if got := len(trees(units)); got != len(old)-1 {
		t.Fatalf("%d trees after the deletion, want %d", got, len(old)-1)
	}
	back, _ := w.read(t, w.names)
	for _, f := range trees(back) {
		if f.pkg == libPackage && slices.ContainsFunc(old, func(o pkgTree) bool { return o.file == f.file }) {
			t.Error("the deleted file came back as its old tree")
		}
	}
}

// NFR-02: readers of one project share a Reuse and a set under -race, each getting whole findings.
func TestReuseConcurrent(t *testing.T) {
	w := newWarmProject(t)
	w.read(t, w.names)
	want := w.cold(t, w.names)
	var wg sync.WaitGroup
	errs := make([]error, readers)
	got := make([]string, readers)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, bags, err := w.readIn(w.fsys, w.names)
			if err != nil {
				errs[i] = err
				return
			}
			got[i], errs[i] = renderFindings(w.set, bags)
		}()
	}
	wg.Wait()
	for i, g := range got {
		if errs[i] != nil || g != want {
			t.Errorf("reader %d: %v\n%s\nwant:\n%s", i, errs[i], g, want)
		}
	}
}

// variant is one reader of the shared store: a file system and the names it reads from it.
type variant struct {
	fsys  memFS
	names []string
	want  string // the findings of a cold read of it
}

// NFR-02: Parse calls of different name sets and contents on one Reuse, one pruning while another
// stores or looks up, each still return the trees of what they read and the cold findings.
func TestReuseConcurrentNameSets(t *testing.T) {
	w := newWarmProject(t)
	edited := newMemFS(txtar.Parse([]byte(tree)))
	edited.m[memPath(libFile)] = &fstest.MapFile{Data: []byte(libChanged)}
	edited.m[memPath(uiFile)] = &fstest.MapFile{Data: []byte(uiChanged)}
	notLib := slices.DeleteFunc(slices.Clone(w.names), func(n string) bool { return n == libFile })
	notUI := slices.DeleteFunc(slices.Clone(w.names), func(n string) bool { return n == uiFile })
	variants := []*variant{
		{fsys: w.fsys, names: w.names}, {fsys: edited, names: w.names}, {fsys: w.fsys, names: notLib},
		{fsys: edited, names: notUI}, {fsys: w.fsys, names: []string{libFile}}, {fsys: edited, names: []string{uiFile, libFile}},
	}
	for _, v := range variants {
		var err error
		if v.want, err = coldIn(v.fsys, v.names); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make([][]string, len(variants))
	for i, v := range variants {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				errs[i] = append(errs[i], w.check(v)...)
			}
		}()
	}
	wg.Wait()
	for i := range errs {
		if len(errs[i]) > 0 {
			t.Errorf("reader %d: %s", i, strings.Join(errs[i], "; "))
		}
	}
}

// check reads v through the shared store and returns what differs from what it read: a tree of
// other content than its file's, or findings other than a cold read's.
func (w *warmProject) check(v *variant) []string {
	units, bags, err := w.readIn(v.fsys, v.names)
	if err != nil {
		return []string{err.Error()}
	}
	var out []string
	for _, tr := range trees(units) {
		data, err := v.fsys.ReadFile(path.Join(projectDir, tr.file.Src.Path))
		if err != nil || sha256.Sum256(data) != sha256.Sum256(tr.file.Src.Content) {
			out = append(out, tr.file.Src.Path+" is not the content read")
		}
	}
	got, err := renderFindings(w.set, bags)
	if err != nil || got != v.want {
		out = append(out, "findings differ from a cold read")
	}
	return out
}

// NFR-02: a store made for another file set would hand out dangling spans; the reader refuses.
func TestReuseOfOtherSet(t *testing.T) {
	w := newWarmProject(t)
	r := &project.Reader{FS: w.fsys, Dir: projectDir, Set: &source.FileSet{}, Reuse: w.reuse}
	if _, err := r.Parse(context.Background(), w.names); !errors.Is(err, project.ErrReuseSet) {
		t.Errorf("Parse = %v, want ErrReuseSet", err)
	}
}
