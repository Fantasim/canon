package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cppruntime "github.com/fantasim/canonlang/internal/gen/cpp/runtime"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// otherStamp stands for the stamp of another canon's runtime texts.
const otherStamp = "000000000000"

// stampDrivers are the sources under testdata/main/stamp that the runtime copy tests compile.
var stampDrivers = []string{"meet_main.cpp", "same.cpp", "other.cpp", "other_main.cpp", "other_json_main.cpp", "ordered_main.cpp"}

// stampTree is a directory holding identical copies of both runtime texts as written in a/ and
// b/, the texts with another stamp in other/, and the drivers of testdata/main/stamp.
func stampTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	other := strings.NewReplacer(cppruntime.Stamp, otherStamp)
	for _, c := range []struct {
		sub          string
		header, json string
	}{
		{"a", cppruntime.Header, cppruntime.JSON},
		{"b", cppruntime.Header, cppruntime.JSON},
		{"other", other.Replace(cppruntime.Header), other.Replace(cppruntime.JSON)},
	} {
		sub := filepath.Join(dir, c.sub)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, sub, []ir.File{{Path: "canon_runtime.h", Content: []byte(c.header)}, {Path: "canon_runtime_json.h", Content: []byte(c.json)}})
	}
	for _, d := range stampDrivers {
		copyFile(t, filepath.Join("testdata", "main", "stamp", d), filepath.Join(dir, d), same)
	}
	return dir
}

// CODEGEN.md §7.4, DECISIONS 436: identical copies meet and share the handler; a copy of another stamp keeps its own, and stops the compile after one.
func TestRuntimeCopiesMeet(t *testing.T) {
	t.Parallel()
	dir := stampTree(t)
	t.Run("identical copies meet", func(t *testing.T) {
		t.Parallel()
		want := "map 1, decoded 1 3\nhandler shared: same 1, other 0\n"
		for _, out := range buildAndRun(t, dir, []string{"meet_main.cpp", "same.cpp", "other.cpp"}) {
			if out != want {
				t.Errorf("driver output:\n%s\nwant\n%s", out, want)
			}
		}
	})
	for _, c := range []struct{ name, driver, want string }{
		{"canon_runtime.h", "other_main.cpp", "a different canon_runtime.h is already included"},
		{"canon_runtime_json.h", "other_json_main.cpp", "a different canon_runtime_json.h is already included"},
	} {
		t.Run("another "+c.name+" stops", func(t *testing.T) {
			t.Parallel()
			compilers, include := cxx.Toolchain(t)
			for _, cc := range compilers {
				args := append(append([]string(nil), cxx.Flags...), "-I", dir, "-I", include, "-fsyntax-only", filepath.Join(dir, c.driver))
				out, err := compile(cc, args, dir)
				if err == nil || !strings.Contains(string(out), c.want) {
					t.Errorf("%s: %v, want a compile error naming %q:\n%s", filepath.Base(cc), err, c.want, out)
				}
			}
		})
	}
}

// CODEGEN.md §7.5, DECISIONS 437: detail::DecodeOrdered reads an ordered_json's objects in its order, at every depth, and fails first where that order does.
func TestDecodeOrderedHelper(t *testing.T) {
	t.Parallel()
	dir := stampTree(t)
	want := "keys: b a d c l z y m e f \n" +
		"keys: \n" +
		"error: ordered: $.b.x[0]: expected an object\n"
	for _, out := range buildAndRun(t, dir, []string{"ordered_main.cpp"}) {
		if out != want {
			t.Errorf("driver output:\n%s\nwant\n%s", out, want)
		}
	}
}
