package build_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

const (
	linkProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    resource: \"../Resource\"\n  }\n}\n"
	pwnedSource = "/// A.\npackage a\n\n/// Pwned.\n@text(\"pwned.txt\")\nexport fn pwned() -> String { return \"pwned\\n\" }\n\n" +
		"emit text { out: \"@resource/Server/Evil.png\" }\n"
	noteSource = "/// A.\npackage a\n\n/// Note.\n@text(\"note.txt\")\nexport fn note() -> String { return \"n\\n\" }\n\n"
	rule341    = "CODEGEN.md §2.4, DECISIONS 341: "
)

// linkTree is a project at tmp/p beside tmp/Resource, with the symbolic link name leading to target.
func linkTree(t *testing.T, files map[string]string, name, target string) string {
	t.Helper()
	if runtime.GOOS == windows {
		t.Skip("symlinks need elevated privilege on windows")
	}
	tmp := t.TempDir()
	writeTree(t, tmp, files)
	if err := os.Symlink(target, filepath.Join(tmp, filepath.FromSlash(name))); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	return tmp
}

// buildAt builds the project at dir on the OS file system.
func buildAt(t *testing.T, dir string, check bool) *build.BuildResult {
	t.Helper()
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{Check: check})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// messagesOf is the message of each finding of res with code c.
func messagesOf(res *build.Result, c *diag.Def) []string {
	var out []string
	for _, f := range res.List {
		if f.Code == c.Code {
			out = append(out, f.Message)
		}
	}
	return out
}

// reported is the message b reports.
func reported(b *diag.Builder) string {
	bag := diag.NewBag(&source.FileSet{}, "")
	b.Report(bag)
	return bag.Findings()[0].Message
}

// API.md B1d, DECISIONS 342: a committed directory link below the root's directory, leading outside the
// project, is E8027 naming the output and the link, under build and build --check alike; the
// link's target is untouched and nothing is written.
func TestBuildRefusesDirectoryLink(t *testing.T) {
	files := map[string]string{"p/project.canon": linkProject, "p/a/a.canon": pwnedSource, "Resource/Server/keep.txt": "keep", "outside/.keep": ""}
	tmp := linkTree(t, files, "Resource/Server/Evil.png", "../../outside")
	want := []string{reported(diag.E8027.At(source.Span{}, "@resource/Server/Evil.png/pwned.txt", "@resource/Server/Evil.png"))}
	for _, check := range []bool{true, false} {
		res := buildAt(t, filepath.Join(tmp, "p"), check)
		if got := messagesOf(&res.Result, diag.E8027.Def()); !slices.Equal(got, want) || len(res.Outputs) != 0 {
			t.Errorf("%scheck %v: refusals %q, outputs %v", rule342, check, got, res.Outputs)
		}
	}
	if names := dirNames(t, filepath.Join(tmp, "outside")); !slices.Equal(names, []string{".keep"}) {
		t.Errorf("%sthe link's target was written: %v", rule342, names)
	}
	if names := dirNames(t, filepath.Join(tmp, "p", "a")); !slices.Equal(names, []string{"a.canon"}) {
		t.Errorf("%sthe project was written: %v", rule342, names)
	}
}

// API.md B1d, DECISIONS 342: the directory of a root outside the project and what is above it
// may be links: a root placed through a link, in a project opened through a link (macOS's /var), is written.
func TestBuildThroughLinkedRootAndProject(t *testing.T) {
	project := "project acme {\n  canon: \"0.1\"\n  roots {\n    resource: \"../res\"\n  }\n}\n"
	files := map[string]string{"p/project.canon": project, "p/a/a.canon": noteSource + "emit text { out: [\"@resource/notes\", \"gen\"] }\n", "Resource/notes/keep.txt": "keep"}
	tmp := linkTree(t, files, "res", "Resource")
	if err := os.Symlink("p", filepath.Join(tmp, "plink")); err != nil {
		t.Fatal(err)
	}
	res := buildAt(t, filepath.Join(tmp, "plink"), false)
	if res.Summary.Errors != 0 {
		t.Fatalf("%s%v", rule342, res.List)
	}
	for _, name := range []string{"Resource/notes/note.txt", "p/a/gen/note.txt"} {
		if !exists(filepath.Join(tmp, filepath.FromSlash(name))) {
			t.Errorf("%s%s not written", rule342, name)
		}
	}
}

// DECISIONS 341: a load is compared with the outputs by real path: a load through a link to the
// output directory reads a file the emit writes, and is E8026 under check.
func TestCheckFeedbackThroughLink(t *testing.T) {
	src := noteSource + "/// Fed.\nlet fed: String = load.text(\"alias/note.txt\")\n\nemit text { out: \"gen\" }\n"
	files := map[string]string{"p/project.canon": linkProject, "p/a/a.canon": src, "p/a/gen/note.txt": "n\n", "Resource/.keep": ""}
	tmp := linkTree(t, files, "p/a/alias", "gen")
	p, err := build.Open(build.OS(), filepath.ToSlash(filepath.Join(tmp, "p")), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{reported(diag.E8026.At(source.Span{}, "a/alias/note.txt", "text", "a"))}
	if got := messagesOf(res, diag.E8026.Def()); !slices.Equal(got, want) {
		t.Errorf("%srefusals %q, findings %v", rule341, got, res.List)
	}
}

// API.md B1d, DECISIONS 342: a legacy `.canon-text` linked to a file is refused alone, its target untouched.
func TestBuildRefusesLegacyManifestLink(t *testing.T) {
	files := map[string]string{"p/project.canon": textProject, "p/a/a.canon": textEmit, "p/out/sql/.keep": "", "legacy": legacyOne}
	tmp := linkTree(t, files, "p/out/sql/.canon-text", "../../../legacy")
	res := buildAt(t, filepath.Join(tmp, "p"), false)
	want := []string{reported(diag.E8027.At(source.Span{}, "@out/sql/.canon-text", "@out/sql/.canon-text"))}
	if got := messagesOf(&res.Result, diag.E8027.Def()); !slices.Equal(got, want) || len(res.Outputs) != 0 {
		t.Errorf("%srefusals %q, outputs %v", rule342, got, res.Outputs)
	}
	if got, err := os.ReadFile(filepath.Join(tmp, "legacy")); err != nil || string(got) != legacyOne {
		t.Errorf("%sthe link's target: %q, %v", rule342, got, err)
	}
	if names := dirNames(t, filepath.Join(tmp, "p", "out", "sql")); !slices.Equal(names, []string{".canon-text", ".keep"}) {
		t.Errorf("%sthe output directory was written: %v", rule342, names)
	}
}
