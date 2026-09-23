package main

import (
	"bytes"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	specErrors = "../../../../spec/ERRORS.md"
	specPlan   = "../../../../spec/IMPLEMENTATION-PLAN.md"
	diagDir    = "../.."
)

func args(out string, extra ...string) []string {
	return append([]string{"-errors", specErrors, "-plan", specPlan, "-out", out}, extra...)
}

// IMPLEMENTATION-PLAN.md §4.4: the committed files are what diaggen generates now.
func TestCommittedFilesAreCurrent(t *testing.T) {
	out := t.TempDir()
	if code := run(args(out), io.Discard); code != exitOK {
		t.Fatalf("exit %d", code)
	}
	for _, name := range []string{"codes.go", "codes_test.go"} {
		got, err1 := os.ReadFile(filepath.Join(out, name))
		want, err2 := os.ReadFile(filepath.Join(diagDir, name))
		if err1 != nil || err2 != nil {
			t.Fatal(err1, err2)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("internal/diag/%s is stale: run go run ./internal/diag/cmd/diaggen", name)
		}
	}
}

// ERRORS.md §2.1: a refused catalogue or runtime text writes nothing and exits 1.
func TestRefusalWritesNothing(t *testing.T) {
	doc, err := os.ReadFile(specErrors)
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(t.TempDir(), "ERRORS.md")
	if err := os.WriteFile(bad, []byte(strings.Replace(string(doc), "\nThe catalogue holds ", "\nThe catalogue holds 1", 1)), filePerm); err != nil {
		t.Fatal(err)
	}
	runtime := filepath.Join(t.TempDir(), "go", "runtime")
	if err := os.MkdirAll(runtime, dirPermForTest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime, "rt.go.txt"), []byte(`Fail("E4101", "overflow")`), filePerm); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]string{
		"catalogue":    {"-errors", bad, "-plan", specPlan},
		"runtime text": {"-errors", specErrors, "-plan", specPlan, "-runtime", filepath.Dir(filepath.Dir(runtime))},
		"runtime file": {"-errors", specErrors, "-plan", specPlan, "-runtime", specPlan},
	}
	for _, name := range slices.Sorted(maps.Keys(cases)) {
		a := cases[name]
		t.Run(name, func(t *testing.T) {
			out := t.TempDir()
			var stderr bytes.Buffer
			if code := run(append(a, "-out", out), &stderr); code != exitFail {
				t.Errorf("exit %d, want %d: %s", code, exitFail, stderr.String())
			}
			if entries, _ := os.ReadDir(out); len(entries) != 0 {
				t.Errorf("wrote %d files", len(entries))
			}
		})
	}
}

// ERRORS.md §2.1: a tree without runtime helper texts passes the runtime check.
func TestRuntimeCheckOfAnEmptyTree(t *testing.T) {
	if code := run(args(t.TempDir(), "-runtime", t.TempDir()), io.Discard); code != exitOK {
		t.Errorf("exit %d", code)
	}
}

func TestUsageErrors(t *testing.T) {
	for _, a := range [][]string{{"-nope"}, args(t.TempDir(), "extra")} {
		if code := run(a, io.Discard); code != exitUsage {
			t.Errorf("%v: exit %d, want %d", a, code, exitUsage)
		}
	}
}

const dirPermForTest = 0o700
