package build_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

const rule451 = "CODEGEN.md §2.4, DECISIONS 451: "

// CODEGEN.md §2.4, DECISIONS 451: a pattern through a link to the output directory is E8026 before the file exists.
func TestFeedbackUnwrittenThroughLink(t *testing.T) {
	src := "/// A.\npackage a\n\n/// R.\nrecord R {\n  /// V.\n  v: Int\n}\n\n/// Fed.\nlet fed: [R] = load.dir(\"alias/*.json\")\n\n" +
		"/// Note.\n@text(\"note.json\")\nexport fn note() -> String { return \"n\\n\" }\n\nemit text { out: \"gen\" }\n"
	files := map[string]string{"p/project.canon": linkProject, "p/a/a.canon": src, "p/a/gen/hand.json": "{\"v\": 1}\n", "Resource/.keep": ""}
	tmp := linkTree(t, files, "p/a/alias", "gen")
	want := []string{reported(diag.E8026.At(source.Span{}, "a/alias/note.json", "text", "a"))}
	for _, check := range []bool{true, false} {
		res := buildAt(t, filepath.Join(tmp, "p"), check)
		if got := messagesOf(&res.Result, diag.E8026.Def()); !slices.Equal(got, want) {
			t.Errorf("%scheck %v: refusals %q, findings %v", rule451, check, got, res.List)
		}
		if names := dirNames(t, filepath.Join(tmp, "p", "a", "gen")); !slices.Equal(names, []string{"hand.json"}) {
			t.Errorf("%scheck %v: the output directory was written: %v", rule451, check, names)
		}
	}
}

// CODEGEN.md §2.4, DECISIONS 451: a pattern matching no output stays clean before and after the build.
func TestFeedbackUnwrittenNarrowPatternStaysClean(t *testing.T) {
	src := "/// A.\npackage a\n\n/// R.\nrecord R {\n  /// V.\n  v: Int\n}\n\n/// Fed.\nlet fed: [R] = load.dir(\"gen/h*.json\")\n\n" +
		"/// Note.\n@text(\"note.json\")\nexport fn note() -> String { return \"n\\n\" }\n\nemit text { out: \"gen\" }\n"
	files := map[string]string{"p/project.canon": linkProject, "p/a/a.canon": src, "p/a/gen/hand.json": "{\"v\": 1}\n", "Resource/.keep": ""}
	tmp := t.TempDir()
	writeTree(t, tmp, files)
	for _, check := range []bool{true, false, true} {
		res := buildAt(t, filepath.Join(tmp, "p"), check)
		if got := messagesOf(&res.Result, diag.E8026.Def()); len(got) != 0 || res.Summary.Errors != 0 {
			t.Errorf("%scheck %v: refusals %q, findings %v", rule451, check, got, res.List)
		}
	}
	if names := dirNames(t, filepath.Join(tmp, "p", "a", "gen")); !slices.Equal(names, []string{"hand.json", "note.json"}) {
		t.Errorf("%sthe output directory holds %v", rule451, names)
	}
}

// CODEGEN.md §2.4, DECISIONS 451: one physical file reached by two paths is one E8026, named as the walk names it.
func TestFeedbackOneFindingPerPhysicalFile(t *testing.T) {
	src := "/// A.\npackage a\n\n/// R.\nrecord R {\n  /// V.\n  v: Int\n}\n\n/// Fed.\nlet fed: [R] = load.dir(\"@data/**/*.json\")\n\n" +
		"/// Note.\n@text(\"y.json\")\nexport fn note() -> String { return \"n\\n\" }\n\nemit text { out: \"@data/zgen\" }\n"
	project := "project acme {\n  canon: \"0.1\"\n  roots {\n    data: \"a/data\"\n  }\n}\n"
	files := map[string]string{"p/project.canon": project, "p/a/a.canon": src, "p/a/data/zgen/y.json": "{\"v\": 1}\n"}
	tmp := linkTree(t, files, "p/a/data/agen", "zgen")
	want := []string{reported(diag.E8026.At(source.Span{}, "@data/agen/y.json", "text", "a"))}
	res := buildAt(t, filepath.Join(tmp, "p"), true)
	if got := messagesOf(&res.Result, diag.E8026.Def()); !slices.Equal(got, want) {
		t.Errorf("%srefusals %q, findings %v", rule451, got, res.List)
	}
}
