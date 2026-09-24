package golden

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// cppCompileDir holds the driver TestPipelineCppCompiles compiles alongside the golden.
var cppCompileDir = filepath.Join("testdata", "cpp")

// staleSchema is a fingerprint that never matches potions.json's real one (CPP-06).
const staleSchema = "pipeline.Potion@00000000"

// buildAndRun compiles sources in dir with every compiler cxx.Toolchain finds and every mode of
// cxx.Modes (W4: both with and without exceptions), runs each binary with args under a
// timeout, and returns the output of each run; a failed build or a non-zero exit fails the test.
func buildAndRun(t *testing.T, dir string, sources []string, args ...string) []string {
	t.Helper()
	compilers, include := cxx.Toolchain(t)
	var outs []string
	for _, cc := range compilers {
		for i, mode := range cxx.Modes {
			bin := filepath.Join(dir, filepath.Base(cc)+"-"+string(rune('a'+i)))
			cmdArgs := append(append(append([]string(nil), cxx.Flags...), mode...), "-I", dir, "-I", include, "-o", bin)
			for _, s := range sources {
				cmdArgs = append(cmdArgs, filepath.Join(dir, s))
			}
			buildCtx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
			out, err := exec.CommandContext(buildCtx, cc, cmdArgs...).CombinedOutput()
			cancel()
			if err != nil {
				t.Fatalf("%s: %v\n%s", filepath.Base(cc), err, out)
			}
			runCtx, runCancel := context.WithTimeout(context.Background(), cxx.Timeout)
			var stdout, stderr bytes.Buffer
			run := exec.CommandContext(runCtx, bin, args...)
			run.Stdout, run.Stderr = &stdout, &stderr
			err = run.Run()
			runCancel()
			if err != nil {
				t.Fatalf("%s: run: %v\n%s%s", filepath.Base(cc), err, stdout.String(), stderr.String())
			}
			outs = append(outs, stdout.String())
		}
	}
	return outs
}

// copyGolden copies every regenerated pipeline C++ source and header into dir.
func copyGolden(t *testing.T, expected, dir string) {
	t.Helper()
	for _, name := range []string{
		"canon_runtime.h", "canon_runtime_json.h",
		"pipeline.gen.h", "pipeline.gen.cpp", "pipeline_conformance.gen.cpp",
	} {
		b, err := os.ReadFile(filepath.Join(expected, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, filePerm); err != nil {
			t.Fatal(err)
		}
	}
}

// potionsSchema is data's "$schema" envelope value, read from the golden itself so this test
// never needs its own copy of the fingerprint.
func potionsSchema(t *testing.T, data []byte) string {
	t.Helper()
	var envelope struct {
		Schema string `json:"$schema"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Schema
}

// writeDataDirs lays out good/ (the regenerated potions.json), stale/ (its schema replaced) and
// an empty missing/, as the driver's argv[1] expects (CPP-06, M2 acceptance 5).
func writeDataDirs(t *testing.T, expected, dir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(expected, "potions.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile := func(sub string, b []byte) {
		full := filepath.Join(dir, sub, "potions.json")
		if err := os.MkdirAll(filepath.Dir(full), dirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, b, filePerm); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("good", data)
	writeFile("stale", bytes.Replace(data, []byte(potionsSchema(t, data)), []byte(staleSchema), 1))
	if err := os.MkdirAll(filepath.Join(dir, "missing"), dirPerm); err != nil {
		t.Fatal(err)
	}
}

// TestPipelineCppCompiles builds the regenerated pipeline files with every found toolchain and exception mode, -Werror, and checks RunPipelineConformance() and the loaded values (M2 acceptance 2, DECISIONS 190).
func TestPipelineCppCompiles(t *testing.T) {
	expected := filepath.Join(examplesDir, "pipeline", "expected")
	dir := t.TempDir()
	copyGolden(t, expected, dir)
	writeDataDirs(t, expected, dir)
	main, err := os.ReadFile(filepath.Join(cppCompileDir, "main.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.cpp"), main, filePerm); err != nil {
		t.Fatal(err)
	}
	outs := buildAndRun(t, dir, []string{"pipeline.gen.cpp", "pipeline_conformance.gen.cpp", "main.cpp"}, dir)
	for _, out := range outs {
		if !strings.Contains(out, "failures: 0\n") {
			t.Errorf("driver output:\n%s", out)
		}
	}
}
