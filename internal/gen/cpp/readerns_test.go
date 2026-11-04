package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// readerNSFiles are the Ws rdtypes parses, and what the reader says of each: the branch of H's param follows the shape of the library's item that ev names (count for sword, place for shield, none for ghost).
var readerNSFiles = []struct{ name, text, want string }{
	{"count.json", `{"h": {"ev": "sword", "param": 7}}`, "ev=sword param=count 7"},
	{"place.json", `{"h": {"ev": "shield", "param": "home"}}`, "ev=shield param=place home"},
	{"absent.json", `{"h": {"ev": "sword"}}`, "ev=sword param=none"},
	{"unknown.json", `{"h": {"ev": "axe", "param": 1}}`, "error W: $.h.param: no branch for this value"},
	{"never.json", `{"h": {"ev": "ghost", "param": 1}}`, "error W: $.h.param: no branch for this value"},
	{"wrong.json", `{"h": {"ev": "sword", "param": "x"}}`, "error W: $.h.param: expected an integer"},
	{"wrongplace.json", `{"h": {"ev": "shield", "param": 1}}`, "error W: $.h.param: expected a string"},
}

// CODEGEN.md §2.8, §5.6, DECISIONS 350, 456, 457: a data and a types reader of another package's class, whose dependent type is decided through a ref into a table of a third package whose ids are strings, compile and read: the branch table and the switch name the third package's discriminant enum, which neither the owner's class nor the readers' headers hold, so the reader's .gen.cpp includes that package's header (DECISIONS 457).
func TestReaderNamesTheLibrarysDiscriminant(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := newReaderNS()
	var sources []string
	for _, p := range []*ir.Package{f.lib, f.own, f.data, f.typed} {
		e := cppEmit(p)
		out := filepath.Join(dir, filepath.FromSlash(e.Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, p))
		sources = append(sources, e.Dir+"/"+p.Name[strings.LastIndex(p.Name, ".")+1:]+".gen.cpp")
	}
	copyFile(t, filepath.Join("testdata", "main", "readerns_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	data := `{"$schema": "` + rnsSchema + `", "value": {"ev": "sword", "param": 3}}`
	if err := os.WriteFile(filepath.Join(dir, "h.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	args, want := []string{dir}, "h.json: ev=sword param=count 3\n"
	for _, c := range readerNSFiles {
		if err := os.WriteFile(filepath.Join(dir, c.name), []byte(c.text), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, c.name)
		want += c.name + ": " + c.want + "\n"
	}
	for _, got := range buildAndRun(t, dir, append(sources, "main.cpp"), args...) {
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	}
}

// CODEGEN.md §2.7, §2.8, DECISIONS 457: the .gen.cpp of a reader includes, in the group of its runtime and after it, the header of every imported package it names that its .gen.h does not include; its .gen.h does not.
func TestReaderIncludesTheLibrarysHeaderInItsSource(t *testing.T) {
	t.Parallel()
	for _, p := range []*ir.Package{readerNSData(), readerNSTypes()} {
		header, source := generatedTexts(t, p)
		const group = "#include \"canon_runtime_json.h\"\n#include \"../lib/rdlib.gen.h\"\n\nnamespace"
		if !strings.Contains(source, group) || strings.Contains(header, "rdlib.gen.h") {
			head, _, _ := strings.Cut(source, "namespace")
			t.Errorf("%s: the source should include the library right after the runtime and the header not:\n%s", p.Name, head)
		}
	}
}
