package canon

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

// scopeLaw is a project where a's xs comes from data/x.json; r names that file only in a function
// nothing calls, and holds an error; u holds a type error and v a failing check, both unrelated to
// a; v has a layer file of its own.
func scopeLaw() map[string][]byte {
	return map[string][]byte{
		"/law/project.canon": []byte("project acme {\n  canon: \"0.1\"\n}\n"),
		"/law/a/a.canon":     []byte("/// A.\npackage a\n\n/// Xs.\nlet xs: [Int] = load(\"../data/x.json\")\n\n/// N.\nlet n: Int = 1\n"),
		"/law/data/x.json":   []byte("[1, 2]\n"),
		"/law/r/r.canon": []byte("/// R.\npackage r\n\n/// Never called.\nfn never() -> [Int] {\n  load(\"../data/x.json\")\n}\n\n" +
			"/// Broken.\nlet broken: Int = \"s\"\n"),
		"/law/u/u.canon": []byte("/// U.\npackage u\n\n/// Broken.\nlet broken: Int = \"s\"\n"),
		"/law/v/v.canon": []byte("/// V.\npackage v\n\n/// A config.\nrecord Cfg {\n  /// Port.\n  port: Int\n}\n\n" +
			"/// The config.\nlet cfg: Cfg = { port: 1 }\n\ncheck cfg.port > 1 else \"port must exceed 1\"\n"),
		"/law/v/live.layer.canon": []byte("package v\nlayer live\n\namend cfg {\n  port: 3\n}\n"),
	}
}

// scopeOpen opens files under /law with layers active.
func scopeOpen(t *testing.T, files map[string][]byte, layers ...string) (*Project, *writeFS) {
	t.Helper()
	fsys := newWriteFS(files, nil)
	return scopeOn(t, fsys, layers...), fsys
}

// scopeOn opens /law on fsys with layers active, a project of its own.
func scopeOn(t *testing.T, fsys *writeFS, layers ...string) *Project {
	t.Helper()
	p, err := Open("/law", Options{FS: fsys, Layers: layers})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// packagesOf is the packages findings name, in order, each once.
func packagesOf(findings []Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Package)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// API.md E17a, E18, E19 (DECISIONS 330): an edit analyses its scope and affected packages alone:
// w, whose evaluation fails a run, never fails it, u's type error and v's failing check are not
// reported, while canon check fails on w and, without w, reports both.
func TestEditUnrelatedNotAnalysed(t *testing.T) {
	files := scopeLaw()
	files["/law/w/w.canon"] = []byte("/// W.\npackage w\n\n/// Texts.\nlet ts: [String] = load.dir(\"t/*.txt\")\n")
	files["/law/w/t/a.txt"] = []byte("a\n")
	p, fsys := scopeOpen(t, files)
	ctx := context.Background()
	res, err := p.Edit(ctx, Edit{Ops: []Op{Set("a:n", Int(2))}})
	if err != nil || !res.Applied {
		t.Fatalf("Edit: %v, %+v", err, res)
	}
	if got := packagesOf(res.Findings); len(got) != 0 {
		t.Errorf("the edit of a reports findings of %v", got)
	}
	if _, err := p.Check(ctx); err == nil {
		t.Error("canon check of the whole project evaluates w and succeeds")
	}
	_ = fsys.Remove("/law/w/w.canon")
	all, err := p.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := packagesOf(all.Findings); !slices.Equal(got, []string{"r", "u", "v"}) {
		t.Errorf("canon check reports %v, want r, u and v: the whole project's verdict", got)
	}
}

// API.md E18, E19 (DECISIONS 330): the findings of a package the edit does not affect, an error
// of a package an affected one imports included, are neither reported nor refuse the edit.
func TestEditImportErrorsNotReported(t *testing.T) {
	files := scopeLaw()
	files["/law/e/e.canon"] = []byte("/// E.\npackage e\n\nimport u\n\n/// N.\nlet n: Int = 1\n")
	p, _ := scopeOpen(t, files)
	res, err := p.Edit(context.Background(), Edit{Ops: []Op{Set("e:n", Int(2))}, DryRun: true})
	if err != nil || len(res.Findings) != 0 {
		t.Errorf("the edit of e, which imports the broken u: %v, %v", err, packagesOf(res.Findings))
	}
}

// API.md E17 (DECISIONS 330): a package whose load names a file the edit writes is affected, found
// from its sources, though that load is never evaluated: r is re-checked and its error reported.
func TestEditAffectsNeverEvaluatedLoad(t *testing.T) {
	p, _ := scopeOpen(t, scopeLaw())
	res, err := p.Edit(context.Background(), Edit{Ops: []Op{Set("a:xs[0]", Int(5))}, AllowErrors: true, DryRun: true, Normalize: true})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if len(res.Changes) != 1 || res.Changes[0].Path != "data/x.json" {
		t.Fatalf("the edit writes %+v, want data/x.json", res.Changes)
	}
	if got := packagesOf(res.Findings); !slices.Equal(got, []string{"r"}) {
		t.Errorf("the edit of data/x.json reports %v, want r's error alone", got)
	}
}

// API.md S3, S4 (DECISIONS 330): the revision covers the static read set of the whole project,
// whatever a call analysed, so two fresh projects of the same files give the same revision, and
// one accepts as current a base the other printed; a change to any loaded file changes it.
func TestRevisionAcrossProjects(t *testing.T) {
	files := scopeLaw()
	first, _ := scopeOpen(t, files)
	ctx := context.Background()
	before := first.Revision()
	if _, err := first.Check(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if after := first.Revision(); after != before {
		t.Errorf("checking a moved the revision from %s to %s", before, after)
	}
	second, fsys := scopeOpen(t, files)
	if got := second.Revision(); got != before {
		t.Fatalf("a second project reads %s, the first %s", got, before)
	}
	third := scopeOn(t, fsys)
	if _, err := third.Edit(ctx, Edit{Base: before, Ops: []Op{Set("a:n", Int(3))}, DryRun: true}); err != nil {
		t.Errorf("a fresh project refuses the base another printed: %v", err)
	}
	_ = fsys.WriteFile("/law/data/x.json", []byte("[7]\n"))
	if got := second.Revision(); got == before {
		t.Error("a loaded file changed and the revision did not")
	}
}

// API.md S3 (DECISIONS 143, 330): a lock place a load names that does not exist is listed
// unreadable, so the project's revision is the build's, and creating that lock changes it.
func TestRevisionNamedMissingLock(t *testing.T) {
	files := scopeLaw()
	files["/law/u/u.canon"] = []byte("/// U.\npackage u\n\n/// R's lock.\nlet t: String = load.text(\"../r/canon.lock\")\n")
	p, fsys := scopeOpen(t, files)
	ctx := context.Background()
	b, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := b.Revision(ctx)
	if got := p.Revision(); err != nil || string(got) != want {
		t.Errorf("the project's revision %s, the build's %s (%v)", got, want, err)
	}
	_ = fsys.WriteFile("/law/r/canon.lock", []byte("# canon.lock v1\n"))
	if again, err := b.Revision(ctx); err != nil || again == want || string(p.Revision()) != again {
		t.Errorf("after the lock appeared: build %s (%v), project %s, before %s", again, err, p.Revision(), want)
	}
}

// API.md S3, S4 (DECISIONS 330): a revision is a function of the files alone: after a root moves,
// one Project's revision is that of a fresh one over the same files.
func TestRevisionAfterRootMove(t *testing.T) {
	canonAt := func(dir string) []byte {
		return []byte("project acme {\n  canon: \"0.1\"\n\n  roots {\n    src: \"" + dir + "\"\n  }\n}\n")
	}
	files := map[string][]byte{
		"/law/project.canon": canonAt("../one"),
		"/law/a/a.canon":     []byte("/// A.\npackage a\n\n/// N.\nlet n: Int = load(\"@src/n.json\")\n"),
		"/one/n.json":        []byte("1\n"),
		"/two/n.json":        []byte("2\n"),
	}
	p, fsys := scopeOpen(t, files)
	ctx := context.Background()
	if _, err := p.Check(ctx); err != nil {
		t.Fatal(err)
	}
	before := p.Revision()
	_ = fsys.WriteFile("/law/project.canon", canonAt("../two"))
	if _, err := p.Check(ctx); err != nil {
		t.Fatal(err)
	}
	got, want := p.Revision(), scopeOn(t, fsys).Revision()
	if got != want || got == before {
		t.Errorf("after the move: %s, a fresh project %s, before %s", got, want, before)
	}
}

// API.md S4, S5 (DECISIONS 330): a revision is compared per package in the project that produced
// it: a change to a file only an unrelated package reads leaves an edit of a current, one to a
// file a reads makes it stale; a fresh project cannot compare per file, so any change is stale.
func TestStaleStaticPerPackage(t *testing.T) {
	files := scopeLaw()
	files["/law/u/u.json"] = []byte("[1]\n")
	files["/law/u/u.canon"] = []byte("/// U.\npackage u\n\n/// Ys.\nlet ys: [Int] = load(\"u.json\")\n")
	p, fsys := scopeOpen(t, files)
	ctx := context.Background()
	base := p.Revision()
	_ = fsys.WriteFile("/law/u/u.json", []byte("[2]\n"))
	if _, err := p.Edit(ctx, Edit{Base: base, Ops: []Op{Set("a:n", Int(3))}, DryRun: true}); err != nil {
		t.Errorf("a change u alone reads makes an edit of a stale: %v", err)
	}
	fresh := scopeOn(t, fsys)
	var stale *StaleError
	if _, err := fresh.Edit(ctx, Edit{Base: base, Ops: []Op{Set("a:n", Int(3))}, DryRun: true}); !errors.As(err, &stale) {
		t.Errorf("a fresh project accepts a base whose files changed: %v", err)
	}
	_ = fsys.WriteFile("/law/data/x.json", []byte("[9]\n"))
	_, err := p.Edit(ctx, Edit{Base: base, Ops: []Op{Set("a:n", Int(3))}, DryRun: true})
	if !errors.As(err, &stale) || !slices.Contains(stale.Files, "data/x.json") {
		t.Errorf("a change to a file a loads: %v, want stale naming data/x.json", err)
	}
}

// API.md E17a, O4 (DECISIONS 330): an active layer is judged against the layer headers of
// every scanned package, so an edit of a under v's layer analyses neither v nor fails; a layer no
// package declares is still ErrUnknownLayer.
func TestEditLayerJudgedByHeaders(t *testing.T) {
	p, _ := scopeOpen(t, scopeLaw(), "live")
	ctx := context.Background()
	res, err := p.Edit(ctx, Edit{Ops: []Op{Set("a:n", Int(2))}, DryRun: true})
	if err != nil || len(res.Findings) != 0 {
		t.Errorf("an edit of a under v's layer: %v, %v", err, res)
	}
	q, _ := scopeOpen(t, scopeLaw(), "nowhere")
	if _, err := q.Edit(ctx, Edit{Ops: []Op{Set("a:n", Int(2))}, DryRun: true}); !errors.Is(err, ErrUnknownLayer) {
		t.Errorf("a layer no package declares: %v, want ErrUnknownLayer", err)
	}
}

// API.md E18 (DECISIONS 330): a header two packages load reports its redefinition to each, so a
// package's findings never depend on which packages a call analysed: the edit of g reports g's
// own, and canon check one per package.
func TestSharedHeaderFindingsPerPackage(t *testing.T) {
	user := func(pkg string) []byte {
		return []byte("/// " + pkg + ".\npackage " + pkg + "\n\nlocal let m = load.defines(\"../data/h.h\")\n\n/// V.\nlet v: Int = m.Y.value\n\n/// N.\nlet n: Int = 1\n")
	}
	files := map[string][]byte{
		"/law/project.canon": []byte("project acme {\n  canon: \"0.1\"\n}\n"),
		"/law/data/h.h":      []byte("#define X 1\n#define X 2\n#define Y 3\n"),
		"/law/f/f.canon":     user("f"),
		"/law/g/g.canon":     user("g"),
	}
	p, _ := scopeOpen(t, files)
	ctx := context.Background()
	res, err := p.Edit(ctx, Edit{Ops: []Op{Set("g:n", Int(4))}, AllowErrors: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	redefined := string(diag.E7102.Def().Code)
	if got := codesOf(res.Findings); !slices.Equal(got, []string{"g " + redefined}) {
		t.Errorf("the edit of g reports %v, want g's redefinition", got)
	}
	all, err := p.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := codesOf(all.Findings); !slices.Equal(got, []string{"f " + redefined, "g " + redefined}) {
		t.Errorf("canon check reports %v, want one redefinition per package", got)
	}
}

// API.md E17, E19, TYPES.md 13.4 (DECISIONS 330): b, which a does not import, names a's entry file
// through an asset root, written plainly or through a link: removing that entry affects b, whose
// re-check lists the root by its real path, so the edit is refused for b's missing asset.
func TestEditAffectsAssetRoot(t *testing.T) {
	for _, root := range []string{"../a/items", "ln"} {
		dir := t.TempDir()
		linksWrite(t, dir, map[string]string{
			"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
			"a/a.canon": "/// A.\npackage a\n\n/// An item.\nrecord Item {\n  /// N.\n  n: Int\n}\n\n" +
				"/// Items.\n@files(\"items/{id}.canon\")\nlet items: table Item = {}\n",
			"a/items/axe.canon": "package a\n\nentry items.axe {\n  n: 1\n}\n",
			"b/b.canon": "/// B.\npackage b\n\n/// An icon.\ntype Icon = asset(\"" + root + "\", ext: [canon])\n\n" +
				"/// The icon.\nlet icon: Icon = \"axe.canon\"\n",
		})
		if err := os.Symlink("../a/items", filepath.Join(dir, "b", "ln")); err != nil {
			t.Skipf("no symbolic links here: %v", err)
		}
		p, err := Open(dir, Options{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Edit(context.Background(), Edit{Ops: []Op{Remove("a:items.axe")}})
		if !errors.Is(err, ErrRejected) || !slices.Equal(codesOf(res.Findings), []string{"b " + string(diag.E3701.Def().Code)}) {
			t.Errorf("root %s: removing a's entry b lists: %v, %v", root, err, res)
		}
		_ = p.Close()
	}
}

// codesOf is each finding's package and code, sorted.
func codesOf(findings []Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Package+" "+f.Code)
	}
	slices.Sort(out)
	return out
}

// API.md E20: an edit adding an id to a's stable table writes a's lock alone; b, re-checked as a's
// importer, keeps its lock not in canonical order untouched.
func TestEditLeavesOtherLocks(t *testing.T) {
	bLock := []byte("# canon.lock v1\ntable  b.things  t2\ntable  b.things  t1\n")
	files := map[string][]byte{
		"/law/project.canon": []byte("project acme {\n  canon: \"0.1\"\n}\n"),
		"/law/a/a.canon": []byte("/// A.\npackage a\n\n/// An item.\nrecord Item {\n  /// V.\n  v: Int\n}\n\n" +
			"/// Items.\nlet items: stable table Item = {\n  i1 { v: 1 }\n}\n"),
		"/law/a/canon.lock": []byte("# canon.lock v1\ntable  a.items  i1\n"),
		"/law/b/b.canon": []byte("/// B.\npackage b\n\nimport a\n\n/// A thing.\nrecord Thing {\n  /// N.\n  n: Int\n}\n\n" +
			"/// Things.\nlet things: stable table Thing = {\n  t1 { n: a.items.i1.v }\n  t2 { n: 2 }\n}\n"),
		"/law/b/canon.lock": bLock,
	}
	p, fsys := scopeOpen(t, files)
	res, err := p.Edit(context.Background(), Edit{Ops: []Op{AddEntry("a:items", Key("i2"), Source("{ v: 2 }"))}})
	if err != nil || !res.Applied {
		t.Fatalf("Edit: %v, %+v", err, res)
	}
	paths := make([]string, len(res.Changes))
	for i, c := range res.Changes {
		paths[i] = c.Path
	}
	if !slices.Equal(paths, []string{"a/a.canon", "a/canon.lock"}) {
		t.Errorf("the edit writes %v, want a's source and lock alone", paths)
	}
	if got, err := fsys.ReadFile("/law/b/canon.lock"); err != nil || !bytes.Equal(got, bLock) {
		t.Errorf("b's lock is now %q (%v)", got, err)
	}
}

// API.md E17a: an edit without ops analyses no package: u's and r's errors are not reported, and
// its revision is the project's.
func TestEditNoOpsAnalysesNothing(t *testing.T) {
	p, _ := scopeOpen(t, scopeLaw())
	res, err := p.Edit(context.Background(), Edit{})
	if err != nil || len(res.Findings) != 0 || res.Revision != p.Revision() {
		t.Errorf("an edit without ops: %v, %+v", err, res)
	}
}
