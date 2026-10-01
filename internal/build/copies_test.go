package build_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

const (
	copiesProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    east: \"east\"\n    west: \"west\"\n  }\n" +
		"  go_module {\n    east: \"example.com/east\"\n    west: \"example.com/west\"\n  }\n}\n"
	copiesB = "/// B.\npackage b\n\n/// A colour.\nenum Tone { red, blue }\n\n" +
		"emit go { out: [\"@east/b/\", \"@west/b/\"] }\n" +
		"emit cpp { out: [\"@east/cpp/b/\", \"@west/b/cpp/\"], mode: types }\n"
	copiesA = "/// A.\npackage a\n\nimport b { Tone }\n\n/// A badge.\nrecord Badge {\n  /// Its colour.\n  tone: Tone\n}\n\n" +
		"/// The badges.\nlet badges: table Badge = {\n  gold { tone: red }\n  iron { tone: blue }\n}\n\n" +
		"emit go { out: [\"@east/a/\", \"@west/a/\"], mode: data }\n" +
		"emit cpp { out: [\"@east/cpp/a/\", \"@west/deep/a/cpp/\"], mode: data }\n" +
		"emit json { out: [\"@east/data/\", \"@west/data/\"] }\n"
)

// copiesTree is a project whose packages a and b each emit go, cpp and json copies under the
// roots east and west, a importing b.
func copiesTree() mapFS {
	return mapFS{"p/project.canon": file(copiesProject), "p/a/a.canon": file(copiesA), "p/b/b.canon": file(copiesB)}
}

// outputOf is the content of the output at display path, or fails.
func outputOf(t *testing.T, res *build.BuildResult, display string) string {
	t.Helper()
	i := slices.IndexFunc(res.Outputs, func(o build.Output) bool { return o.Path == display })
	if i < 0 {
		t.Fatalf("no output %s among %d", display, len(res.Outputs))
	}
	return string(res.Outputs[i].Content)
}

// CODEGEN.md §2.1, §2.3, §2.8, DECISIONS 229.
func TestCopiesImportTheirSiblings(t *testing.T) {
	fsys := copiesTree()
	res := buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors != 0 {
		t.Fatalf("errors: %v", codes(res.List))
	}
	for _, c := range []struct{ path, has, hasNot string }{
		{"@east/a/a.gen.go", "\"example.com/east/b\"", "example.com/west"},
		{"@east/a/a.gen.go", "\"example.com/east/a/rt\"", "example.com/west"},
		{"@west/a/a.gen.go", "\"example.com/west/b\"", "example.com/east"},
		{"@west/a/a.gen.go", "\"example.com/west/a/rt\"", "example.com/east"},
		{"@east/b/b.gen.go", "package b", "example.com/west"},
		{"@west/b/b.gen.go", "package b", "example.com/east"},
		{"@east/cpp/a/a.gen.h", "#include \"../b/b.gen.h\"", "../../"},
		{"@west/deep/a/cpp/a.gen.h", "#include \"../../../b/cpp/b.gen.h\"", "../b/b.gen.h"},
	} {
		got := outputOf(t, res, c.path)
		if !strings.Contains(got, c.has) || strings.Contains(got, c.hasNot) {
			t.Errorf("%s: want %s and no %s:\n%s", c.path, c.has, c.hasNot, got)
		}
		if string(fsys["p/"+strings.TrimPrefix(c.path, "@")].Data) != got {
			t.Errorf("%s: not written", c.path)
		}
	}
	for _, rt := range []string{"@east/a/rt/rt.go", "@west/a/rt/rt.go", "@east/cpp/a/canon_runtime.h", "@west/deep/a/cpp/canon_runtime.h"} {
		outputOf(t, res, rt)
	}
	if east, west := outputOf(t, res, "@east/data/badges.json"), outputOf(t, res, "@west/data/badges.json"); east != west {
		t.Errorf("json copies differ:\n%s\n%s", east, west)
	}
}

// CODEGEN.md §2.3 (copies, E8001 and the lock), DECISIONS 229.
func TestCopiesWriteAllOrNothing(t *testing.T) {
	fsys := copiesTree()
	fsys["p/west/data/badges.json"] = file("{\"rows\": []}\n")
	res := buildTree(t, fsys, build.BuildOptions{})
	if got := codes(res.List); !slices.Equal(got, []diag.Code{diag.E8001.Def().Code}) {
		t.Fatalf("codes %v, want %s alone", got, diag.E8001.Def().Code)
	}
	if at := source.Pos(strings.Index(copiesA, "\"@west/data/\"")); res.List[0].Span.Start != at {
		t.Errorf("%s at offset %d, want the entry at %d", res.List[0].Code, res.List[0].Span.Start, at)
	}
	for name := range fsys { //canon:unordered each name is judged alone
		if strings.HasPrefix(name, "p/east/") {
			t.Errorf("%s written by a failed build", name)
		}
	}
}

// WIRE.md §8.1 (E8152), CODEGEN.md §2.3, DECISIONS 229.
func TestCopiesCollide(t *testing.T) {
	fsys := copiesTree()
	fsys["p/c/c.canon"] = file("/// C.\npackage c\n\n/// A clash.\nlet badges: Int = 4\n\nemit json { out: \"@west/data/\" }\n")
	res := buildTree(t, fsys, build.BuildOptions{})
	got := codes(res.List)
	if !slices.Contains(got, diag.E8152.Def().Code) {
		t.Fatalf("codes %v, want %s", got, diag.E8152.Def().Code)
	}
}
