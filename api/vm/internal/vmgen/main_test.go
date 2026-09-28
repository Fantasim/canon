package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const (
	specSchema = "../../../../spec/viewmodel.schema.json"
	vmDir      = "../.."
)

// API.md R10: the committed vm.gen.go is what vmgen generates from the schema now (the Go
// side of make vm-check).
func TestCommittedFileIsCurrent(t *testing.T) {
	out := t.TempDir()
	if code := run([]string{"-schema", specSchema, "-out", out}, io.Discard); code != exitOK {
		t.Fatalf("exit %d", code)
	}
	got, err1 := os.ReadFile(filepath.Join(out, outFile))
	want, err2 := os.ReadFile(filepath.Join(vmDir, outFile))
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("api/vm/%s is stale: run go run ./api/vm/internal/vmgen", outFile)
	}
}

// DOCTRINE §5: two runs write the same bytes.
func TestDeterministic(t *testing.T) {
	data, err := os.ReadFile(specSchema)
	if err != nil {
		t.Fatal(err)
	}
	first, err := generate(data, names)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		again, err := generate(data, names)
		if err != nil || !bytes.Equal(first, again) {
			t.Fatalf("a second run differs (%v)", err)
		}
	}
}

// A refused schema writes nothing and exits 1.
func TestRefusalWritesNothing(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(bad, []byte(`{"type":"object","properties":{"a":{"nope":1}}}`), filePerm); err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{bad, filepath.Join(t.TempDir(), "missing.json")} {
		out := t.TempDir()
		var stderr bytes.Buffer
		if code := run([]string{"-schema", schema, "-out", out}, &stderr); code != exitFail {
			t.Errorf("%s: exit %d, want %d: %s", schema, code, exitFail, stderr.String())
		}
		if entries, _ := os.ReadDir(out); len(entries) != 0 {
			t.Errorf("%s: wrote %d files", schema, len(entries))
		}
	}
}

func TestUsageErrors(t *testing.T) {
	for _, a := range [][]string{{"-nope"}, {"-schema", specSchema, "extra"}} {
		if code := run(a, io.Discard); code != exitUsage {
			t.Errorf("%v: exit %d, want %d", a, code, exitUsage)
		}
	}
}
