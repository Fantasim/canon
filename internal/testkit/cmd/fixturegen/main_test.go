package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const header = "#ifndef __DEFINE_T\r\n#define __DEFINE_T\r\n\r\n// a comment\r\n#define\tA\t1\r\n#define B 2 // bee\r\n#define C 3\r\n\r\n#endif\r\n"

func realTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), filePerm); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// DECISIONS 29: without testdata-real/ the tool fails and writes nothing.
func TestAbsentRealDataWritesNothing(t *testing.T) {
	out := t.TempDir()
	var stderr bytes.Buffer
	code := run([]string{"-real", filepath.Join(t.TempDir(), "testdata-real"), "-fixtures", out}, &stderr)
	if code != exitFail || !strings.Contains(stderr.String(), errNoRealData.Error()) {
		t.Errorf("exit %d: %s", code, stderr.String())
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("wrote %d entries", len(entries))
	}
	if code := run([]string{"extra"}, io.Discard); code != exitUsage {
		t.Errorf("exit %d for an argument, want %d", code, exitUsage)
	}
}

// IMPLEMENTATION-PLAN.md §7.3: the guard and the listed defines, byte for byte, twice alike.
func TestDefinesExtraction(t *testing.T) {
	real := realTree(t, map[string]string{"resource/Server/Define/defineT.h": header})
	out := t.TempDir()
	m := "# comment\nresource/Server/Define/defineT.h\tdefines\tC A\n"
	for range 2 {
		if err := extract(options{real: real, out: out}, m); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(out, "resource/Server/Define/defineT.h"))
		if err != nil {
			t.Fatal(err)
		}
		want := "#ifndef __DEFINE_T\n#define __DEFINE_T\n\n#define\tA\t1\n#define C 3\n\n#endif\n"
		if string(got) != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	}
}

func TestExtractionRefuses(t *testing.T) {
	real := realTree(t, map[string]string{"r/a.h": "#define A 1\n#define A 2\n", "r/b.h": "#define B 1\n"})
	tests := map[string]struct {
		manifest string
		want     error
	}{
		"undefined name":  {"r/b.h\tdefines\tB Z\n", errExtract},
		"defined twice":   {"r/a.h\tdefines\tA\n", errExtract},
		"missing file":    {"r/c.h\tdefines\tC\n", errExtract},
		"unknown method":  {"r/b.h\tcopy\tB\n", errManifest},
		"unsorted":        {"r/b.h\tdefines\tB\nr/a.h\tdefines\tA\n", errManifest},
		"escaping path":   {"../b.h\tdefines\tB\n", errManifest},
		"no names":        {"r/b.h\tdefines\t\n", errManifest},
		"too many fields": {"r/b.h\tdefines\tB\textra\n", errManifest},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			out := t.TempDir()
			if err := extract(options{real: real, out: out}, tc.manifest); !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
			if entries, _ := os.ReadDir(out); len(entries) != 0 {
				t.Errorf("wrote %d entries", len(entries))
			}
		})
	}
}

// IMPLEMENTATION-PLAN.md §7.3: the fixture tree stays within 300 KB, counting what is kept.
func TestBudget(t *testing.T) {
	out := realTree(t, map[string]string{"big.json": strings.Repeat("x", maxFixtureBytes)})
	real := realTree(t, map[string]string{"r/b.h": "#define B 1\n"})
	if err := extract(options{real: real, out: out}, "r/b.h\tdefines\tB\n"); !errors.Is(err, errBudget) {
		t.Errorf("got %v, want %v", err, errBudget)
	}
}

// The embedded list is valid, and every committed fixture it names holds the listed names.
func TestManifestNamesCommittedFixtures(t *testing.T) {
	rows, err := parseManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		src, err := os.ReadFile(filepath.Join("../../../..", defaultOut, filepath.FromSlash(r.path)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := methods[r.method](src, r.names); err != nil {
			t.Errorf("%s: %v", r.path, err)
		}
	}
}
