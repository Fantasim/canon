package build_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

const dirCaseRule = "DECISIONS 377, CODEGEN.md §2.4: "

// textIn is package a writing one @text file of the given name and content into out.
func textIn(out, name, content string) string {
	return strings.Replace(textFn(name, content), "\"@out/sql\"", "\""+out+"\"", 1)
}

// jsonIn is package a writing the json file of its table named name into out: its bytes name
// the row type, not the table.
func jsonIn(out, name string) string {
	return "/// P.\npackage a\n\n/// R.\nrecord Row {\n  /// Weight.\n  weight: Int\n}\n\n/// V.\nlet " + name +
		": table Row = {\n  x { weight: 1 }\n}\n\nemit json { out: \"" + out + "\" }\n"
}

// outFiles are the files of fsys under p/out/, sorted, as the disk spells them.
func outFiles(fsys mapFS) []string {
	return filesUnder(fsys, "p/out/")
}

// renameCase is a project built once from before, then given after as package a's source, with
// the files the build must leave and the output whose status the --check must report stale.
type renameCase struct {
	name, before, after string
	files               []string
	stale               string
}

// run builds c on a case-insensitive file system: --check reports it stale with no error, the
// build gives every file its declared spelling, and the next --check is fresh.
func (c renameCase) run(t *testing.T) caseFS {
	t.Helper()
	fsys := caseFS{mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(c.before)}}
	if res := buildOn(t, fsys, build.BuildOptions{}); res.Summary.Errors != 0 {
		t.Fatalf("%s%s: first build %v", dirCaseRule, c.name, codes(res.List))
	}
	fsys.mapFS["p/a/a.canon"] = file(c.after)
	check := buildOn(t, fsys, build.BuildOptions{Check: true})
	if check.Summary.Errors != 0 || !check.Stale || statusOf(t, check, c.stale) != build.StatusStale {
		t.Errorf("%s%s: --check %v, stale %v", dirCaseRule, c.name, codes(check.List), check.Stale)
	}
	res := buildOn(t, fsys, build.BuildOptions{})
	if got := outFiles(fsys.mapFS); res.Summary.Errors != 0 || !slices.Equal(got, c.files) {
		t.Errorf("%s%s: %v, files %v, want %v", dirCaseRule, c.name, codes(res.List), got, c.files)
	}
	if again := buildOn(t, fsys, build.BuildOptions{Check: true}); again.Stale || again.Summary.Errors != 0 {
		t.Errorf("%s%s: not fresh after the build: %v", dirCaseRule, c.name, codes(again.List))
	}
	return fsys
}

// DECISIONS 377, CODEGEN.md §2.4, §2.9: a case-only rename of an out directory on a case-insensitive file system is no E8001.
func TestCaseOnlyDirRename(t *testing.T) {
	for _, c := range []renameCase{
		{"directory, bytes changed", textIn("@out/Sql", "s.sql", "A"), textIn("@out/sql", "s.sql", "B"), []string{"p/out/sql/s.sql"}, "@out/sql/s.sql"},
		{"directory, bytes unchanged", textIn("@out/Sql", "s.sql", "A"), textIn("@out/sql", "s.sql", "A"), []string{"p/out/sql/s.sql"}, "@out/sql/s.sql"},
		{"directory and file name", textIn("@out/Sql", "S.sql", "A"), textIn("@out/sql", "s.sql", "A"), []string{"p/out/sql/s.sql"}, "@out/sql/s.sql"},
		{"two nested directories", textIn("@out/Gen/Sql", "s.sql", "A"), textIn("@out/gen/sql", "s.sql", "A"), []string{"p/out/gen/sql/s.sql"}, "@out/gen/sql/s.sql"},
	} {
		fsys := c.run(t)
		listing := string(fsys.mapFS["p/a/canon.outputs"].Data)
		if want := strings.TrimPrefix(c.files[0], "p/out/"); !strings.Contains(listing, "  @out/"+want+"\n") || strings.Contains(listing, "Sql") {
			t.Errorf("%s%s: canon.outputs %q", dirCaseRule, c.name, listing)
		}
	}
}

// DECISIONS 377, CODEGEN.md §2.4: a json out directory, and a json value of the same bytes, renamed by case get their declared spelling.
func TestCaseOnlySpellingEveryTarget(t *testing.T) {
	for _, c := range []renameCase{
		{"json directory", jsonIn("@out/Data/", "v"), jsonIn("@out/data/", "v"), []string{"p/out/data/v.json"}, "@out/data/v.json"},
		{"json value", jsonIn("@out/data/", "potionA"), jsonIn("@out/data/", "potiona"), []string{"p/out/data/potiona.json"}, "@out/data/potiona.json"},
	} {
		c.run(t)
	}
}

// DECISIONS 377, API.md N11: a build failing after the directory rename puts the directory back
// under its old spelling, with the old bytes.
func TestCaseOnlyDirRenameRollback(t *testing.T) {
	base := caseFS{mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(textIn("@out/Sql", "s.sql", "A"))}}
	buildOn(t, base, build.BuildOptions{})
	base.mapFS["p/a/a.canon"] = file(textIn("@out/sql", "s.sql", "B"))
	fails := func(o, n string) bool { return strings.HasSuffix(n, "/s.sql") && strings.Contains(o, "/.s.sql.") }
	p, err := build.Open(failingCaseFS{caseFS: base, fails: fails}, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Build(context.Background(), build.BuildOptions{}); !errors.Is(err, errInjected) {
		t.Fatalf("%s%v", dirCaseRule, err)
	}
	if got := outFiles(base.mapFS); !slices.Equal(got, []string{"p/out/Sql/s.sql"}) || string(base.mapFS["p/out/Sql/s.sql"].Data) != "A" {
		t.Errorf("%sfiles %v", dirCaseRule, got)
	}
}

// dirCaseLists are the findings of a build and a check of fsys.
func dirCaseLists(t *testing.T, fsys mapFS) ([]diag.Finding, []diag.Finding) {
	t.Helper()
	built := buildTree(t, fsys, build.BuildOptions{}).List
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return built, res.List
}

// hasDirCase reports an E8152 dirCase finding naming the directories a and b in list.
func hasDirCase(list []diag.Finding, a, b string) bool {
	bag := diag.NewBag(&source.FileSet{}, "")
	diag.E8152.AtDirCase(source.Span{}, a, b).Report(bag)
	want := bag.Findings()[0].Message
	return slices.ContainsFunc(list, func(f diag.Finding) bool { return f.Code == diag.E8152.Def().Code && f.Message == want })
}

// WIRE.md §8.1, DECISIONS 377: two text or go emits into directories equal but for case are E8152 dirCase in build and check.
func TestDirCaseCollision(t *testing.T) {
	const goProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n  }\n  go_module {\n    out: \"example.com/out\"\n  }\n}\n"
	goPkg := func(name, out string) string {
		return "/// P.\npackage " + name + "\n\n/// V.\nlet v: Int = 1\n\nemit go { out: \"" + out + "\" }\n"
	}
	for _, c := range []struct {
		name string
		fsys mapFS
	}{
		{"text", mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(onePackage("a", "x.txt", "@out/Gen")), "p/b/b.canon": file(onePackage("b", "y.txt", "@out/gen"))}},
		{"go", mapFS{"p/project.canon": file(goProject), "p/a/a.canon": file(goPkg("a", "@out/Gen")), "p/b/b.canon": file(goPkg("b", "@out/gen"))}},
	} {
		built, checked := dirCaseLists(t, c.fsys)
		if !hasDirCase(built, "@out/Gen", "@out/gen") || !hasDirCase(checked, "@out/Gen", "@out/gen") {
			t.Errorf("%s%s: build %v, check %v", dirCaseRule, c.name, codes(built), codes(checked))
		}
	}
}

// viewIn is package name writing its view model to out.
func viewIn(name, out string) string {
	return "/// P.\npackage " + name + "\n\n/// V.\nlet v: Int = 1\n\nemit view { out: \"" + out + "\" }\n"
}

// WIRE.md §8.1, DECISIONS 377: a view, still written after errors, beside a package's directory under another spelling is E8152 dirCase, nothing renamed or written.
func TestDirCaseViewBesidePackage(t *testing.T) {
	fsys := caseFS{mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(viewIn("a", "../A/a.view.json"))}}
	res := buildOn(t, fsys, build.BuildOptions{})
	if got := filesUnder(fsys.mapFS, "p/"); !hasDirCase(res.List, "a", "A") || !slices.Equal(got, []string{"p/a/a.canon", "p/project.canon"}) {
		t.Errorf("%s%v, files %v", dirCaseRule, codes(res.List), got)
	}
}

// WIRE.md §8.1, DECISIONS 377: two views into directories equal but for case write neither side, on a case-sensitive file system too.
func TestDirCaseViews(t *testing.T) {
	fsys := mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(viewIn("a", "@out/Gen/a.view.json")), "p/b/b.canon": file(viewIn("b", "@out/gen/b.view.json"))}
	res := buildTree(t, fsys, build.BuildOptions{})
	if got := outFiles(fsys); !hasDirCase(res.List, "@out/Gen", "@out/gen") || len(got) != 0 {
		t.Errorf("%s%v, files %v", dirCaseRule, codes(res.List), got)
	}
}

// WIRE.md §8.1, DECISIONS 377: an emit into a package's directory under another spelling is E8152, and nothing is renamed.
func TestDirCasePackageDir(t *testing.T) {
	fsys := caseFS{mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(onePackage("a", "x.txt", "../A/gen"))}}
	res := buildOn(t, fsys, build.BuildOptions{})
	if !slices.Contains(codes(res.List), diag.E8152.Def().Code) || fsys.mapFS["p/a/a.canon"] == nil {
		t.Errorf("%s%v, files %v", dirCaseRule, codes(res.List), filesUnder(fsys.mapFS, "p/"))
	}
}

// onlyRootProject has the consumer root ui at web/ui, inside the project.
const onlyRootProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    ui: \"web/ui\"\n  }\n  consumer_roots: [ui]\n}\n"

// DECISIONS 377, 343: build --only-root renames below its root's directory; a rename above it
// is E8028, and nothing is written.
func TestCaseOnlyDirRenameOnlyRoot(t *testing.T) {
	only := build.BuildOptions{OnlyRoot: "ui"}
	fsys := caseFS{mapFS{"p/project.canon": file(onlyRootProject), "p/a/a.canon": file(jsonIn("@ui/Gen/", "v"))}}
	buildOn(t, fsys, build.BuildOptions{})
	buildOn(t, fsys, only)
	fsys.mapFS["p/a/a.canon"] = file(jsonIn("@ui/gen/", "v"))
	if res := buildOn(t, fsys, only); res.Summary.Errors != 0 || !slices.Equal(filesUnder(fsys.mapFS, "p/web/"), []string{"p/web/ui/gen/v.json"}) {
		t.Errorf("%sbelow the root: %v, files %v", dirCaseRule, codes(res.List), filesUnder(fsys.mapFS, "p/web/"))
	}
	above := caseFS{mapFS{"p/project.canon": file(onlyRootProject), "p/a/a.canon": file(jsonIn("@ui/gen/", "v")), "p/Web/ui/README": file("")}}
	buildOn(t, above, build.BuildOptions{})
	res := buildOn(t, above, only)
	if !slices.Contains(codes(res.List), diag.E8028.Def().Code) || !slices.Equal(filesUnder(above.mapFS, "p/Web/"), []string{"p/Web/ui/README"}) {
		t.Errorf("%sabove the root: %v, files %v", dirCaseRule, codes(res.List), filesUnder(above.mapFS, "p/"))
	}
}

// DECISIONS 377, CODEGEN.md §2.4: on a real case-insensitive disk (macOS, Windows) the directory takes its declared spelling.
func TestCaseOnlyDirRenameOnDisk(t *testing.T) {
	tmp := t.TempDir()
	if caseSensitive(t, tmp) {
		t.Skip("case-sensitive file system")
	}
	writeTree(t, tmp, map[string]string{"p/project.canon": textProject, "p/a/a.canon": textIn("@out/Sql", "s.sql", "A")})
	dir := filepath.ToSlash(filepath.Join(tmp, "p"))
	buildDisk := func() *build.BuildResult {
		p, err := build.Open(build.OS(), dir, build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Build(context.Background(), build.BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	buildDisk()
	if err := os.WriteFile(filepath.Join(tmp, "p", "a", "a.canon"), []byte(textIn("@out/sql", "s.sql", "B")), 0o600); err != nil {
		t.Fatal(err)
	}
	if res := buildDisk(); res.Summary.Errors != 0 || !slices.Equal(dirNames(t, filepath.Join(tmp, "p", "out")), []string{"sql"}) {
		t.Errorf("%s%v, %v", dirCaseRule, codes(res.List), dirNames(t, filepath.Join(tmp, "p", "out")))
	}
}

// DECISIONS 377, 336: on a case-sensitive file system the two spellings are two directories: the
// declared one is written, and the old listed file removed by its hash.
func TestCaseOnlyDirRenameSensitive(t *testing.T) {
	fsys := mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(textIn("@out/Sql", "s.sql", "A"))}
	buildOn(t, fsys, build.BuildOptions{})
	fsys["p/a/a.canon"] = file(textIn("@out/sql", "s.sql", "A"))
	if res := buildOn(t, fsys, build.BuildOptions{}); res.Summary.Errors != 0 || !slices.Equal(outFiles(fsys), []string{"p/out/sql/s.sql"}) {
		t.Errorf("%s%v, files %v", dirCaseRule, codes(res.List), outFiles(fsys))
	}
}
