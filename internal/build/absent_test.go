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

const (
	absentProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    src: \"../Source\"\n    out: \"out\"\n  }\n  optional_roots: [src]\n}\n"
	absentSource  = "/// A.\npackage a\n\n/// V.\nlet v: Int = 1\n\n" +
		"/// The version.\n@text(\"_version.sql\")\nexport fn versionSql() -> String { return \"SELECT 1;\\n\" }\n\n" +
		"/// The schema.\n@text(\"Schema.sql\")\nexport fn schemaSql() -> String { return \"CREATE TABLE t (id INT);\\n\" }\n\n" +
		"emit json { out: [\"@out/\", \"@src/json/\"] }\n\nemit text { out: [\"@out/sql\", \"@src/sql\"] }\n"
	absentListing = "a/canon.outputs"
)

// absentTree is the project with the optional root @src, present when present.
func absentTree(present bool) mapFS {
	m := mapFS{"p/project.canon": file(absentProject), "p/a/a.canon": file(absentSource)}
	if present {
		m["Source/README"] = file("")
	}
	return m
}

// contents is each output of res by display path.
func contents(res *build.BuildResult) map[string]string {
	out := map[string]string{}
	for _, o := range res.Outputs {
		out[o.Path] = string(o.Content)
	}
	return out
}

// CODEGEN.md §2.4, §2.9, CLI.md §3.4, DECISIONS 332: outputs are machine-independent bytes.
func TestAbsentRootSkipsOutputs(t *testing.T) {
	without, with := absentTree(false), absentTree(true)
	skipped := buildTree(t, without, build.BuildOptions{})
	full := buildTree(t, with, build.BuildOptions{})
	if got := codesOf(skipped.List); !slices.Equal(got, []diag.Code{diag.W8024.Def().Code}) {
		t.Fatalf("absent: findings %v", skipped.List)
	}
	want := diag.NewBag(&source.FileSet{}, "")
	diag.W8024.AtMany(source.Span{}, 3, "src", "../Source").Report(want)
	if got := skipped.List[0].Message; got != want.Findings()[0].Message {
		t.Errorf("skip: %s", got)
	}
	if len(full.List) != 0 {
		t.Fatalf("present: findings %v", full.List)
	}
	all := contents(full)
	for _, o := range skipped.Outputs {
		if strings.HasPrefix(o.Path, "@src/") || all[o.Path] != string(o.Content) {
			t.Errorf("%s: written without @src as %q, with it as %q", o.Path, o.Content, all[o.Path])
		}
	}
	if n := len(skipped.Outputs) + 3; n != len(full.Outputs) {
		t.Errorf("outputs: %d without @src, %d with it", len(skipped.Outputs), len(full.Outputs))
	}
	listing := string(without["p/"+absentListing].Data)
	if listing != string(with["p/"+absentListing].Data) || !strings.Contains(listing, "@src/sql/Schema.sql\n") {
		t.Errorf("canon.outputs differs or lacks the skipped files:\n%s---\n%s", listing, with["p/"+absentListing].Data)
	}
	//canon:unordered each name is judged on its own
	for name := range without {
		if strings.HasPrefix(name, "Source/") {
			t.Errorf("%s written under the absent root", name)
		}
	}
}

// CLI.md §3.4 --adopt, DECISIONS 332: a path under an absent optional root is not adopted.
func TestAbsentRootRefusesAdopt(t *testing.T) {
	fsys := absentTree(false)
	res := buildTree(t, fsys, build.BuildOptions{Adopt: []string{"@src/sql/Schema.sql"}})
	for _, o := range res.Outputs {
		if o.Status == build.StatusAdopted || strings.HasPrefix(o.Path, "@src/") {
			t.Errorf("output %s, status %d", o.Path, o.Status)
		}
	}
	if got := codesOf(res.List); !slices.Equal(got, []diag.Code{diag.W8024.Def().Code}) {
		t.Errorf("findings %v", res.List)
	}
}

// CODEGEN.md §2.4, CLI.md §3.4 --check, DECISIONS 332: E8023, nothing stale nor written.
func TestAbsentRootCheck(t *testing.T) {
	fsys := absentTree(false)
	res := buildTree(t, fsys, build.BuildOptions{Check: true})
	if got := codesOf(res.List); !slices.Equal(got, []diag.Code{diag.E8023.Def().Code}) || res.Stale || len(res.Outputs) != 0 {
		t.Errorf("findings %v, stale %t, outputs %d", res.List, res.Stale, len(res.Outputs))
	}
	if fsys["p/"+absentListing] != nil {
		t.Error("--check wrote canon.outputs")
	}
}

// CODEGEN.md §2.4, DECISIONS 332: no absent root's directory is created; below a present one, yes.
func TestNoRootDirectoryCreated(t *testing.T) {
	tmp := t.TempDir()
	writeTree(t, tmp, map[string]string{
		"p/project.canon": "project acme {\n  canon: \"0.1\"\n  roots {\n    src: \"../Source\"\n    ext: \"../Ext\"\n  }\n  optional_roots: [src]\n}\n",
		"p/a/a.canon":     "/// A.\npackage a\n\n/// V.\nlet v: Int = 1\n\nemit json { out: [\"@src/deep/json/\", \"@ext/deep/json/\"] }\n",
		"Ext/README":      "",
	})
	p, err := build.Open(build.OS(), filepath.ToSlash(filepath.Join(tmp, "p")), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{})
	if err != nil || res.Summary.Errors != 0 {
		t.Fatalf("build: %v, %v", err, res.List)
	}
	if _, err := os.Stat(filepath.Join(tmp, "Source")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the absent root's directory was created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "Ext", "deep", "json", "v.json")); err != nil {
		t.Errorf("below the present root: %v", err)
	}
}

// CODEGEN.md §2.4, DECISIONS 332: an out naming present @ext below absent @sub is skipped under @sub.
func TestNestedAbsentRootOnDisk(t *testing.T) {
	tmp := t.TempDir()
	writeTree(t, tmp, map[string]string{
		"p/project.canon": "project acme {\n  canon: \"0.1\"\n  roots {\n    ext: \"../Ext\"\n    sub: \"../Ext/sub\"\n  }\n  optional_roots: [sub]\n}\n",
		"p/a/a.canon":     "/// A.\npackage a\n\n/// V.\nlet v: Int = 1\n\nemit json { out: [\"@ext/sub/json/\", \"@ext/json/\"] }\n",
		"Ext/README":      "",
	})
	p, err := build.Open(build.OS(), filepath.ToSlash(filepath.Join(tmp, "p")), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := diag.NewBag(&source.FileSet{}, "")
	diag.W8024.AtOne(source.Span{}, "sub", "../Ext/sub").Report(want)
	if len(res.List) != 1 || res.List[0].Message != want.Findings()[0].Message {
		t.Errorf("findings %v", res.List)
	}
	if _, err := os.Stat(filepath.Join(tmp, "Ext", "sub")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the absent nested root's directory was created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "Ext", "json", "v.json")); err != nil {
		t.Errorf("the output under @ext: %v", err)
	}
}
