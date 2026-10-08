package build_test

import (
	"context"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
)

const (
	guardB        = "/// B.\npackage b\n\n/// V.\nlet v: Int = 1\n"
	guardLoad     = "/// X.\nlet xs: [Int] = load(\"../res/x.json\")\n\n/// Always.\n@text(\"u.sql\")\nexport fn u() -> String { return \"U{xs.len()}\" }\n\n"
	repointed     = "project acme {\n  canon: \"0.1\"\n  roots {\n    out: \".\"\n    copy: \"copy\"\n  }\n}\n"
	maybeAliases  = "/// A.\npackage a\n\n/// Maybe text.\ntype Maybe = String?\n\n/// Always.\n@text(\"t.sql\")\nexport fn t() -> String { return \"T\" }\n\n/// Alias.\n@text(\"w.sql\")\nexport fn w() -> Maybe { return none }\n\n/// Refined.\n@text(\"x.sql\")\nexport fn x() -> String(1..)? { return none }\n\nemit text { out: \"@out/sql\" }\n"
	guardRuleText = "DECISIONS 336: "
)

// hidden is the optional source with its files none.
func hidden(src string) string { return strings.Replace(src, showFlag, hideFlag, 1) }

// listed rewrites package a's canon.outputs, which the previous build wrote, to name extra files too: display path to the file in fsys, each line carrying the SHA-256 of the bytes the file holds now.
func listed(fsys mapFS, extra map[string]string) {
	var lines strings.Builder
	for _, display := range slices.Sorted(maps.Keys(extra)) {
		var content string
		if f := fsys[extra[display]]; f != nil {
			content = string(f.Data)
		}
		lines.WriteString(sumLine(display, content))
	}
	fsys["p/a/canon.outputs"] = file(optionalBoth + lines.String())
}

// DECISIONS 336, CODEGEN.md §2.9 (1): a listed path that is an input of the run (a source of any package, project.canon, a canon.lock, a file a load read) is never removed, with the root as declared or repointed; the file the package did write is still removed.
func TestTextRemovalSparesInputs(t *testing.T) {
	for _, c := range []struct {
		name, project, prefix string
	}{
		{"declared root", textProject, ""},
		{"repointed root", repointed, "@out/"},
	} {
		src := strings.Replace(optionalSource, "/// Always.", guardLoad+"/// Always.", 1)
		fsys := mapFS{"p/project.canon": file(c.project), "p/a/a.canon": file(src), "p/b/b.canon": file(guardB), "p/res/x.json": file("[1,2]")}
		buildTree(t, fsys, build.BuildOptions{})
		fsys["p/a/a.canon"] = file(hidden(src))
		listed(fsys, map[string]string{c.prefix + "b/b.canon": "p/b/b.canon", c.prefix + "project.canon": "p/project.canon", c.prefix + "res/x.json": "p/res/x.json", c.prefix + "a/a.canon": "p/a/a.canon", c.prefix + "b/canon.lock": "p/b/canon.lock"})
		res := buildTree(t, fsys, build.BuildOptions{})
		if res.Summary.Errors != 0 {
			t.Fatalf("%s%s: %v", guardRuleText, c.name, codes(res.List))
		}
		for _, f := range []string{"b/b.canon", "project.canon", "res/x.json", "a/a.canon"} {
			if !onFile(fsys, f) || hasOutput(res, c.prefix+f) {
				t.Errorf("%s%s: %s was removed", guardRuleText, c.name, f)
			}
		}
		if onFile(fsys, "out/sql/s.sql") && c.prefix == "" {
			t.Errorf("%s%s: the stale output was not removed", guardRuleText, c.name)
		}
	}
}

// DECISIONS 336, CODEGEN.md §2.9 (3, 5): with --target text, a listed path that another target's emit writes (a json data file, marked or stripped of its marker) is kept.
func TestTextRemovalSparesOtherTargets(t *testing.T) {
	src := strings.Replace(optionalSource, "emit text", "/// V.\nlet v: Int = 1\n\nemit json { out: \"@out/sql/\" }\n\nemit text", 1)
	for _, strip := range []bool{false, true} {
		fsys := textTree(src)
		buildTree(t, fsys, build.BuildOptions{})
		data := slices.DeleteFunc(filesUnder(fsys, "p/out/sql/"), func(n string) bool { return !strings.HasSuffix(n, "v.json") })
		if len(data) != 1 {
			t.Fatalf("data file: %v", filesUnder(fsys, "p/out/sql/"))
		}
		if strip {
			fsys[data[0]] = file("1\n")
		}
		fsys["p/a/a.canon"] = file(hidden(src))
		listed(fsys, map[string]string{"@out/sql/v.json": data[0]})
		res := buildTree(t, fsys, build.BuildOptions{Targets: []ir.Target{ir.TargetText}})
		if res.Summary.Errors != 0 || fsys[data[0]] == nil || onFile(fsys, "out/sql/s.sql") {
			t.Errorf("%sstripped %v: errors %v, files %v", guardRuleText, strip, codes(res.List), filesUnder(fsys, "p/out/"))
		}
	}
}

// DECISIONS 336, CODEGEN.md §2.9 (4): after --adopt gave a file to another package, building the old package alone does not remove it.
func TestTextRemovalSparesAdoptedFile(t *testing.T) {
	fsys := textTree(textEmit)
	buildTree(t, fsys, build.BuildOptions{})
	fsys["p/a/a.canon"] = file(strings.Replace(textEmit, "@text(\"Schema.sql\")\n", "", 1))
	fsys["p/b/b.canon"] = file("/// B.\npackage b\n\n/// F.\n@text(\"Schema.sql\")\nexport fn f() -> String { return \"b\" }\n\nemit text { out: \"@out/sql\" }\n")
	if res := buildTree(t, fsys, build.BuildOptions{Packages: []string{"b"}, Adopt: []string{"@out/sql/Schema.sql"}}); res.Summary.Errors != 0 {
		t.Fatalf("adopt: %v", codes(res.List))
	}
	if !strings.Contains(string(fsys["p/a/canon.outputs"].Data), "Schema.sql") {
		t.Fatal("package a's list was rewritten by a build of b")
	}
	if res := buildTree(t, fsys, build.BuildOptions{Packages: []string{"a"}}); res.Summary.Errors != 0 || string(fsys["p/out/sql/Schema.sql"].Data) != "b" {
		t.Errorf("%sthe adopted file: %v, %q", guardRuleText, codes(res.List), fsys["p/out/sql/Schema.sql"].Data)
	}
}

// DECISIONS 336, CODEGEN.md §2.9: an alias or a refinement of an optional String that is none gives no file and no list line.
func TestTextOptionalAliasNone(t *testing.T) {
	fsys := textTree(maybeAliases)
	res := buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 || !slices.Equal(filesUnder(fsys, "p/out/"), []string{"p/out/sql/t.sql"}) {
		t.Errorf("%s%v, files %v", guardRuleText, codes(res.List), filesUnder(fsys, "p/out/"))
	}
	if got := string(fsys["p/a/canon.outputs"].Data); got != optionalNone {
		t.Errorf("%scanon.outputs = %q", guardRuleText, got)
	}
}

// DECISIONS 336, CODEGEN.md §2.9: a set-aside name an earlier removal could not delete is swept when its file is gone and was listed.
func TestTextRemovalSweepsAside(t *testing.T) {
	fsys := textTree(optionalSource)
	buildTree(t, fsys, build.BuildOptions{})
	delete(fsys, "p/out/sql/s.sql")
	fsys["p/out/sql/.s.sql.canon-tmp"] = file("SELECT 1;")
	fsys["p/out/sql/.mine.canon-tmp"] = file("not listed")
	fsys["p/a/a.canon"] = file(hidden(optionalSource))
	res := buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 || onFile(fsys, "out/sql/.s.sql.canon-tmp") || !onFile(fsys, "out/sql/.mine.canon-tmp") {
		t.Errorf("%saside: %v, %v", guardRuleText, codes(res.List), filesUnder(fsys, "p/out/"))
	}
}

// caseSensitive reports a temporary directory on a case-sensitive file system.
func caseSensitive(t *testing.T, dir string) bool {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "probe"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(filepath.Join(dir, "PROBE"))
	return err != nil
}

// DECISIONS 336, CODEGEN.md §2.9: renaming a @text file by case only removes the old name on a case-sensitive file system, where it is another file; on a case-insensitive one it is the file just written and stays.
func TestTextRemovalCaseOnlyRename(t *testing.T) {
	rt := newRemoveTree(t)
	if !caseSensitive(t, rt.tmp) {
		t.Skip("case-insensitive file system")
	}
	src := filepath.Join(rt.tmp, "p", "a", "a.canon")
	if err := os.WriteFile(src, []byte(strings.Replace(optionalSource, "\"s.sql\"", "\"S.sql\"", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if res, err := rt.build(false); err != nil || res.Summary.Errors != 0 {
		t.Fatalf("build: %v %v", err, res.List)
	}
	names := dirNames(t, rt.path(sqlDir))
	if !slices.Contains(names, "S.sql") || slices.Contains(names, "s.sql") {
		t.Errorf("%scase-only rename left %v", guardRuleText, names)
	}
}

// DECISIONS 336: a listed path below a file (ENOTDIR on a real file system) holds nothing: it is skipped, not an error.
func TestTextRemovalNotADirectory(t *testing.T) {
	rt := newRemoveTree(t)
	listing := rt.path("p", "a", "canon.outputs")
	data, err := os.ReadFile(listing)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(listing, append(data, "@out/sql/t.sql/inner.sql\n"...), 0o600); err != nil {
		t.Fatal(err)
	}
	if res, err := rt.build(false); err != nil || res.Summary.Errors != 0 {
		t.Errorf("%sENOTDIR: %v, %v", guardRuleText, err, res)
	}
}

// DECISIONS 336, CODEGEN.md §2.9 (5): a listed file that starts with a generated-file line, or a JSON file with a `$schema` key, is kept though no emit claims it; a plain file in the same list is removed.
func TestTextRemovalSparesGeneratedFiles(t *testing.T) {
	fsys := textTree(optionalSource)
	buildTree(t, fsys, build.BuildOptions{})
	fsys["p/out/sql/gen.txt"] = file("// GENERATED by canon from x/. DO NOT EDIT.\n")
	fsys["p/out/sql/data.json"] = file("{\n  \"$schema\": \"x.Y@0123abcd\"\n}\n")
	fsys["p/out/sql/plain.txt"] = file("plain")
	listed(fsys, map[string]string{"@out/sql/gen.txt": "p/out/sql/gen.txt", "@out/sql/data.json": "p/out/sql/data.json", "@out/sql/plain.txt": "p/out/sql/plain.txt"})
	res := buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 || !onFile(fsys, "out/sql/gen.txt") || !onFile(fsys, "out/sql/data.json") || onFile(fsys, "out/sql/plain.txt") {
		t.Errorf("%sgenerated files: %v, %v", guardRuleText, codes(res.List), filesUnder(fsys, "p/out/"))
	}
}

// DECISIONS 336, 326 (the seventh guard): a listed file is removed only if its bytes hash to the sum its line recorded. The unedited one goes; one edited by hand after the build stays; a hand-written notes.md named in a list, by a line with no sum or with the wrong one, stays; a list of the older form, with no sums, removes nothing. Each path drops out of the new list.
func TestTextRemovalNeedsRecordedSum(t *testing.T) {
	fsys := textTree(optionalSource)
	buildTree(t, fsys, build.BuildOptions{})
	fsys["p/out/sql/r.json"] = file("{\"id\": 2}\n")
	fsys["p/notes.md"] = file("my notes")
	fsys["p/other.md"] = file("my other notes")
	fsys["p/a/canon.outputs"] = file(string(fsys["p/a/canon.outputs"].Data) + "notes.md\n" + sumLine("other.md", "not the bytes"))
	fsys["p/a/a.canon"] = file(hidden(optionalSource))
	res := buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 || onFile(fsys, "out/sql/s.sql") || !onFile(fsys, "out/sql/r.json") || !onFile(fsys, "notes.md") || !onFile(fsys, "other.md") {
		t.Fatalf("%s%v, files %v", guardRuleText, codes(res.List), filesUnder(fsys, "p/"))
	}
	if got := string(fsys["p/a/canon.outputs"].Data); got != optionalNone {
		t.Errorf("%scanon.outputs = %q", guardRuleText, got)
	}
}

// DECISIONS 336, 326: a canon.outputs of the older form, a bare path a line, still grants ownership to overwrite but removes nothing.
func TestTextRemovalOlderListRemovesNothing(t *testing.T) {
	fsys := textTree(optionalSource)
	buildTree(t, fsys, build.BuildOptions{})
	fsys["p/a/canon.outputs"] = file("# GENERATED by canon from a/. DO NOT EDIT.\n@out/sql/r.json\n@out/sql/s.sql\n@out/sql/t.sql\n")
	fsys["p/a/a.canon"] = file(hidden(optionalSource))
	res := buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 || !onFile(fsys, "out/sql/s.sql") || !onFile(fsys, "out/sql/r.json") {
		t.Errorf("%solder list: %v, files %v", guardRuleText, codes(res.List), filesUnder(fsys, "p/out/"))
	}
	if got := string(fsys["p/a/canon.outputs"].Data); got != optionalNone {
		t.Errorf("%scanon.outputs = %q", guardRuleText, got)
	}
}

// DECISIONS 326, 336, CODEGEN.md §2.4, §2.9: the line of a copy under an absent root carries the SHA-256 of the bytes it would hold, the same as on a machine that has the root.
func TestTextListingSumOfAbsentCopy(t *testing.T) {
	for _, present := range []bool{false, true} {
		fsys := absentTree(present)
		buildTree(t, fsys, build.BuildOptions{})
		listing := string(fsys["p/"+absentListing].Data)
		for _, root := range []string{"@out", "@src"} {
			if !strings.Contains(listing, sumLine(root+"/sql/Schema.sql", "CREATE TABLE t (id INT);\n")) {
				t.Errorf("%sroot %s present %v: %q", guardRuleText, root, present, listing)
			}
		}
	}
}

// foldFS is a case-insensitive file system over a mapFS: every name is folded to lower case, so the listing of a directory holds one spelling.
type foldFS struct {
	mapFS
}

func (f foldFS) ReadFile(name string) ([]byte, error)  { return f.mapFS.ReadFile(strings.ToLower(name)) }
func (f foldFS) Stat(name string) (fs.FileInfo, error) { return f.mapFS.Stat(strings.ToLower(name)) }
func (f foldFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return f.mapFS.ReadDir(strings.ToLower(name))
}
func (f foldFS) WriteFile(name string, data []byte) error {
	return f.mapFS.WriteFile(strings.ToLower(name), data)
}
func (f foldFS) Rename(oldname, newname string) error {
	return f.mapFS.Rename(strings.ToLower(oldname), strings.ToLower(newname))
}
func (f foldFS) Remove(name string) error { return f.mapFS.Remove(strings.ToLower(name)) }

// buildOn builds the project over fsys, any file system.
func buildOn(t *testing.T, fsys project.FS, opt build.BuildOptions) *build.BuildResult {
	t.Helper()
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// DECISIONS 336, CODEGEN.md §2.9: a case-only rename on a case-insensitive file system, judged by the directory listing, keeps the file just written; on a case-sensitive one the old name is removed, whether or not the new name exists yet.
func TestTextRemovalCaseOnlyRenameByListing(t *testing.T) {
	const old, renamed = "s.sql", "S.sql"
	tree := func() mapFS {
		return mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(onePackage("a", old, "@out/sql"))}
	}
	fsys := foldFS{tree()}
	buildOn(t, fsys, build.BuildOptions{})
	fsys.mapFS["p/a/a.canon"] = file(onePackage("a", renamed, "@out/sql"))
	res := buildOn(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 || string(fsys.mapFS["p/out/sql/s.sql"].Data) != "a" || hasOutput(res, "@out/sql/s.sql") {
		t.Errorf("%sinsensitive: %v, %v", guardRuleText, codes(res.List), res.Outputs)
	}
	if !strings.Contains(string(fsys.mapFS["p/a/canon.outputs"].Data), "@out/sql/S.sql") {
		t.Errorf("%scanon.outputs = %q", guardRuleText, fsys.mapFS["p/a/canon.outputs"].Data)
	}
	for _, both := range []bool{false, true} {
		sensitive := tree()
		buildOn(t, sensitive, build.BuildOptions{})
		sensitive["p/a/a.canon"] = file(onePackage("a", renamed, "@out/sql"))
		if both {
			sensitive["p/out/sql/S.sql"] = file("a")
		}
		buildOn(t, sensitive, build.BuildOptions{})
		if onFile(sensitive, "out/sql/s.sql") || !onFile(sensitive, "out/sql/S.sql") {
			t.Errorf("%ssensitive, new name there %v: %v", guardRuleText, both, filesUnder(sensitive, "p/out/"))
		}
	}
}

// DECISIONS 336, CODEGEN.md §2.4, §2.9 (6): a listed file whose first line is any target's marker (Go, a runtime file, C++, TS, a text list) or whose JSON holds a `$schema` is kept though no emit claims it.
func TestTextRemovalSparesEveryMarker(t *testing.T) {
	files := map[string]string{
		"m.go":      "// Code generated by canon from x/. DO NOT EDIT.\npackage x\n",
		"rt.go":     "// Code generated by canon: Go runtime rt v1. DO NOT EDIT.\n",
		"m.h":       "// GENERATED by canon from x/. DO NOT EDIT.\n",
		"m.cpp":     "// GENERATED by canon: C++ runtime rt_v1 (JSON). DO NOT EDIT.\r\n",
		"m.ts":      "// GENERATED by canon from x/. DO NOT EDIT.\nexport {}\n",
		"list.txt":  "# GENERATED by canon from x/. DO NOT EDIT.\n",
		"m.json":    "{\n  \"$schema\": \"x.Y@0123abcd\"\n}\n",
		"plain.txt": "plain",
	}
	fsys := textTree(optionalSource)
	buildTree(t, fsys, build.BuildOptions{})
	extra := map[string]string{}
	for name, content := range files { //canon:unordered each file is set up alone
		fsys["p/out/sql/"+name] = file(content)
		extra["@out/sql/"+name] = "p/out/sql/" + name
	}
	listed(fsys, extra)
	fsys["p/a/a.canon"] = file(hidden(optionalSource))
	if res := buildTree(t, fsys, build.BuildOptions{}); res.Summary.Errors != 0 {
		t.Fatalf("%s%v", guardRuleText, codes(res.List))
	}
	for name := range files { //canon:unordered each file is judged alone
		if onFile(fsys, "out/sql/"+name) != (name != "plain.txt") {
			t.Errorf("%s%s present = %v", guardRuleText, name, !(name != "plain.txt"))
		}
	}
}

// DECISIONS 336: a set-aside name is swept only for a listed line with a sum, and only if its bytes hash to it.
func TestTextRemovalAsideNeedsSum(t *testing.T) {
	for _, c := range []struct {
		name, line, aside string
		swept             bool
	}{
		{"line with the sum", sumLine("@out/sql/s.sql", optionalText), optionalText, true},
		{"bare line", "@out/sql/s.sql\n", optionalText, false},
		{"other bytes", sumLine("@out/sql/s.sql", optionalText), "edited", false},
	} {
		fsys := textTree(optionalSource)
		buildTree(t, fsys, build.BuildOptions{})
		delete(fsys, "p/out/sql/s.sql")
		fsys["p/out/sql/.s.sql.canon-tmp"] = file(c.aside)
		fsys["p/a/canon.outputs"] = file(optionalNone + c.line)
		fsys["p/a/a.canon"] = file(hidden(optionalSource))
		res := buildTree(t, fsys, build.BuildOptions{})
		if res.Summary.Errors != 0 || onFile(fsys, "out/sql/.s.sql.canon-tmp") == c.swept {
			t.Errorf("%s%s: %v, %v", guardRuleText, c.name, codes(res.List), filesUnder(fsys, "p/out/"))
		}
	}
}
