package build_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

const (
	optionalSource = "/// A.\npackage a\n\nconst SHOW = true\n\n" +
		"/// A row.\nrecord Row {\n  /// Id.\n  id: Int\n}\n\n" +
		"/// Maybe text.\n@text(\"s.sql\")\nexport fn s() -> String? { return if SHOW { \"SELECT 1;\" } else { none } }\n\n" +
		"/// Maybe a row.\n@text(\"r.json\")\nexport fn r() -> Row? { return if SHOW { Row { id: 1 } } else { none } }\n\n" +
		"/// Always.\n@text(\"t.sql\")\nexport fn t() -> String { return \"T\" }\n\n" +
		"emit text { out: \"@out/sql\" }\n"
	rule336 = "CODEGEN.md §2.9, DECISIONS 336: "
)

// The bytes of the files optionalSource writes.
const (
	optionalRow  = "{\n  \"id\": 1\n}\n"
	optionalText = "SELECT 1;"
)

var (
	optionalBoth = outputsFor("a", map[string]string{"@out/sql/r.json": optionalRow, "@out/sql/s.sql": optionalText, "@out/sql/t.sql": "T"})
	optionalNone = outputsFor("a", map[string]string{"@out/sql/t.sql": "T"})
)

// hasOutput reports an output at display path among res.
func hasOutput(res *build.BuildResult, display string) bool {
	return slices.ContainsFunc(res.Outputs, func(o build.Output) bool { return o.Path == display })
}

// CODEGEN.md §2.9, DECISIONS 336: an optional @text result holding a value is written as its inner type would be (String? verbatim, T? as JSON) and listed.
func TestTextOptionalSome(t *testing.T) {
	fsys := textTree(optionalSource)
	res := buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 {
		t.Fatalf("errors: %v", codes(res.List))
	}
	for path, want := range map[string]string{ //canon:unordered each file is judged alone
		"out/sql/s.sql":   "SELECT 1;",
		"out/sql/r.json":  "{\n  \"id\": 1\n}\n",
		"out/sql/t.sql":   "T",
		"a/canon.outputs": optionalBoth,
	} {
		if got := string(fsys["p/"+path].Data); got != want {
			t.Errorf("%s%s = %q, want %q", rule336, path, got, want)
		}
	}
}

// CODEGEN.md §2.9, DECISIONS 336: an optional @text result that is none writes no file and is not listed; a plain @text beside it is unaffected.
func TestTextOptionalNone(t *testing.T) {
	fsys := textTree(strings.Replace(optionalSource, "SHOW = true", "SHOW = false", 1))
	res := buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 {
		t.Fatalf("errors: %v", codes(res.List))
	}
	if got := filesUnder(fsys, "p/out/"); !slices.Equal(got, []string{"p/out/sql/t.sql"}) {
		t.Errorf("%sfiles written: %v", rule336, got)
	}
	for _, skipped := range []string{"@out/sql/s.sql", "@out/sql/r.json"} {
		if hasOutput(res, skipped) {
			t.Errorf("%s%s is an output", rule336, skipped)
		}
	}
	if got := string(fsys["p/a/canon.outputs"].Data); got != optionalNone {
		t.Errorf("%scanon.outputs = %q", rule336, got)
	}
}

// onFile reports the file at path (relative to the project) in fsys.
func onFile(fsys mapFS, path string) bool {
	_, ok := fsys["p/"+path]
	return ok
}

// CODEGEN.md §2.9, DECISIONS 336: flipping the value on, off, on writes the file, removes it (reported written with no content, not listed), and writes it again byte-identical; --check is green after each build.
func TestTextOptionalFlip(t *testing.T) {
	some, none := optionalSource, strings.Replace(optionalSource, "SHOW = true", "SHOW = false", 1)
	fsys := textTree(some)
	buildTree(t, fsys, build.BuildOptions{})
	want := string(fsys["p/out/sql/r.json"].Data)
	for step, src := range []string{none, some, none, some} {
		fsys["p/a/a.canon"] = file(src)
		res := buildTree(t, fsys, build.BuildOptions{})
		for _, f := range []string{"out/sql/s.sql", "out/sql/r.json"} {
			if onFile(fsys, f) != (src == some) {
				t.Errorf("%sstep %d: %s present = %v", rule336, step, f, !(src == some))
			}
		}
		if src == none && (statusOf(t, res, "@out/sql/s.sql") != build.StatusWritten || len(res.Outputs[slices.IndexFunc(res.Outputs, func(o build.Output) bool { return o.Path == "@out/sql/s.sql" })].Content) != 0) {
			t.Errorf("%sstep %d: removal not reported: %+v", rule336, step, res.Outputs)
		}
		if check := buildTree(t, fsys, build.BuildOptions{Check: true}); check.Stale || check.Summary.Errors != 0 || hasOutput(check, "@out/sql/s.sql") != (src == some) {
			t.Fatalf("%sstep %d: --check stale %v: %+v", rule336, step, check.Stale, check.Outputs)
		}
	}
	if got := string(fsys["p/out/sql/r.json"].Data); got != want {
		t.Errorf("%sr.json came back as %q, want %q", rule336, got, want)
	}
	if got := string(fsys["p/a/canon.outputs"].Data); got != optionalBoth {
		t.Errorf("%scanon.outputs = %q", rule336, got)
	}
}

// CODEGEN.md §2.9, DECISIONS 336: --check after flipping the source without building reports the file stale and removes nothing.
func TestTextOptionalCheckStale(t *testing.T) {
	fsys := textTree(optionalSource)
	buildTree(t, fsys, build.BuildOptions{})
	fsys["p/a/a.canon"] = file(strings.Replace(optionalSource, "SHOW = true", "SHOW = false", 1))
	res := buildTree(t, fsys, build.BuildOptions{Check: true})
	if !res.Stale || statusOf(t, res, "@out/sql/s.sql") != build.StatusStale || statusOf(t, res, "a/canon.outputs") != build.StatusStale {
		t.Errorf("%scheck: stale %v, %+v", rule336, res.Stale, res.Outputs)
	}
	if !onFile(fsys, "out/sql/s.sql") || !onFile(fsys, "out/sql/r.json") {
		t.Errorf("%s--check removed a file", rule336)
	}
}

// CODEGEN.md §2.9, DECISIONS 336: renaming a @text file removes the old name; a file in the directory that no canon.outputs lists is untouched, and a listed file already missing is no error.
func TestTextOptionalRenameAndGuards(t *testing.T) {
	fsys := textTree(optionalSource)
	buildTree(t, fsys, build.BuildOptions{})
	fsys["p/out/sql/mine.sql"] = file("-- mine")
	fsys["p/a/canon.outputs"] = file(optionalBoth + "@out/sql/gone.sql\n")
	renamed := strings.Replace(optionalSource, "\"s.sql\"", "\"s2.sql\"", 1)
	fsys["p/a/a.canon"] = file(renamed)
	res := buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 || onFile(fsys, "out/sql/s.sql") || string(fsys["p/out/sql/s2.sql"].Data) != "SELECT 1;" {
		t.Errorf("%srename: %v, %+v", rule336, codes(res.List), res.Outputs)
	}
	if string(fsys["p/out/sql/mine.sql"].Data) != "-- mine" || hasOutput(res, "@out/sql/mine.sql") || hasOutput(res, "@out/sql/gone.sql") {
		t.Errorf("%san unlisted or missing file was touched: %+v", rule336, res.Outputs)
	}
}

// CODEGEN.md §2.4, §2.9, DECISIONS 336: a copy under an absent root stays listed and is not removed; a package whose every optional file is none still writes a canon.outputs.
func TestTextOptionalAbsentRootAndEmptyList(t *testing.T) {
	fsys := absentTree(false)
	res := buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 || !strings.Contains(string(fsys["p/"+absentListing].Data), "@src/sql/Schema.sql\n") {
		t.Errorf("%sabsent root: %v", rule336, codes(res.List))
	}
	none := strings.Replace(optionalSource, "SHOW = true", "SHOW = false", 1)
	none = strings.Replace(none, "/// Always.\n@text(\"t.sql\")\nexport fn t() -> String { return \"T\" }\n\n", "", 1)
	tree := textTree(optionalSource)
	buildTree(t, tree, build.BuildOptions{})
	tree["p/a/a.canon"] = file(none)
	buildTree(t, tree, build.BuildOptions{})
	if got := string(tree["p/a/canon.outputs"].Data); got != "# GENERATED by canon from a/. DO NOT EDIT.\n" || len(filesUnder(tree, "p/out/")) != 0 {
		t.Errorf("%sall none: canon.outputs %q, files %v", rule336, got, filesUnder(tree, "p/out/"))
	}
}

// CODEGEN.md §2.9, DECISIONS 336: a package that drops its emit text loses the files its canon.outputs lists, then the list; a file that list does not name stays.
func TestTextOptionalDroppedEmit(t *testing.T) {
	fsys := textTree(optionalSource)
	buildTree(t, fsys, build.BuildOptions{})
	fsys["p/out/sql/mine.sql"] = file("-- mine")
	fsys["p/a/a.canon"] = file(strings.Replace(optionalSource, "emit text { out: \"@out/sql\" }\n", "", 1))
	res := buildTree(t, fsys, build.BuildOptions{Check: true})
	if !res.Stale || statusOf(t, res, "@out/sql/t.sql") != build.StatusStale || statusOf(t, res, "a/canon.outputs") != build.StatusStale {
		t.Fatalf("%scheck: stale %v, %+v", rule336, res.Stale, res.Outputs)
	}
	res = buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 || onFile(fsys, "out/sql/t.sql") || onFile(fsys, "out/sql/s.sql") || onFile(fsys, "a/canon.outputs") || !onFile(fsys, "out/sql/mine.sql") {
		t.Errorf("%sdropped emit: %v, files %v", rule336, codes(res.List), filesUnder(fsys, "p/"))
	}
}

// CODEGEN.md §2.9, DECISIONS 336: a file is removed only when the previous canon.outputs carries the marker naming this package's own directory; a list copied from another directory removes nothing, and the paths drop out of the new list.
func TestTextOptionalRemovalNeedsOwnMarker(t *testing.T) {
	for _, c := range []struct {
		name, marker string
		gone         bool
	}{
		{"own directory", "a/", true},
		{"other directory", "b/", false},
		{"a longer path", "x/a/", false},
	} {
		fsys := textTree(optionalSource)
		buildTree(t, fsys, build.BuildOptions{})
		fsys["p/a/canon.outputs"] = file(strings.Replace(optionalBoth, "from a/.", "from "+c.marker+".", 1))
		fsys["p/a/a.canon"] = file(strings.Replace(optionalSource, "SHOW = true", "SHOW = false", 1))
		res := buildTree(t, fsys, build.BuildOptions{})
		if res.Summary.Errors != 0 || onFile(fsys, "out/sql/s.sql") == c.gone || hasOutput(res, "@out/sql/s.sql") != c.gone {
			t.Errorf("%s%s: errors %v, files %v", rule336, c.name, codes(res.List), filesUnder(fsys, "p/out/"))
		}
		if got := string(fsys["p/a/canon.outputs"].Data); got != optionalNone {
			t.Errorf("%s%s: canon.outputs = %q", rule336, c.name, got)
		}
	}
}
