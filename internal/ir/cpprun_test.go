package ir_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

const (
	cppRunProbe = "probe.cpp"
	cppRunBin   = "probe"
	cppRunCases = "cases"
	cppRunFlagI = "-I"
	cppRunFlagO = "-o"
	cppRunSep   = "-"
)

// cppRunFile is one argument of a probe, a file it reads unless body is empty, and what the probe prints for it; a quiet file is written and named by no argument (another argument reads it).
type cppRunFile struct {
	name, body, want string
	quiet            bool
}

// cppRunBuild writes the project of the archive at path in a temp dir (its project.canon, else e2eFeatures), builds it with no finding and returns the dir: gen holds the outputs, the archive's probe.cpp sits beside it.
func cppRunBuild(t *testing.T, path string) string {
	t.Helper()
	dir := writeCase(t, path)
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Errors > 0 {
		t.Fatalf("build findings:\n%s", renderFindings(t, res.Findings))
	}
	return dir
}

// cppRunWrite writes each file with a body under dir's cases directory, where the probe runs, and returns every argument, in order.
func cppRunWrite(t *testing.T, dir string, files []cppRunFile) []string {
	t.Helper()
	cases := filepath.Join(dir, cppRunCases)
	if err := os.MkdirAll(cases, e2eDirMode); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		if !f.quiet {
			names = append(names, f.name)
		}
		if f.body == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(cases, f.name), []byte(f.body), e2eFileMode); err != nil {
			t.Fatal(err)
		}
	}
	return names
}

// cppRunOutputs links the probe with every .gen.cpp under dir's gen, once per compiler found and per mode of cxx.Modes (exceptions on, then off), runs it in the cases directory on args and returns each run's output; the compile and the run share one cxx.Timeout.
func cppRunOutputs(t *testing.T, dir string, args []string) []string {
	t.Helper()
	sources := append([]string{cppRunProbe}, filesUnder(t, filepath.Join(dir, e2eGenRoot), e2eCppSuffix)...)
	return cppRunSourcesOutputs(t, dir, sources, nil, args)
}

// cppRunSourcesOutputs is cppRunOutputs for the given sources, compiled with the extra flags.
func cppRunSourcesOutputs(t *testing.T, dir string, sources, extra, args []string) []string {
	t.Helper()
	compilers, include := cxx.Toolchain(t)
	var outs []string
	for _, cc := range compilers {
		for i, mode := range cxx.Modes {
			bin := filepath.Join(dir, cppRunBin+cppRunSep+filepath.Base(cc)+strings.Repeat(cppRunSep, i+1))
			cmd := slices.Concat(cxx.Flags, mode, extra, []string{cppRunFlagI, include, cppRunFlagI, e2eGenRoot, cppRunFlagO, bin}, sources)
			ctx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
			runIn(t, dir, exec.CommandContext(ctx, cc, cmd...))
			run := exec.CommandContext(ctx, bin, args...)
			run.Dir = filepath.Join(dir, cppRunCases)
			out, err := run.CombinedOutput()
			cancel()
			if err != nil {
				t.Fatalf("%s %s: %v\n%s", filepath.Base(cc), strings.Join(mode, " "), err, out)
			}
			outs = append(outs, string(out))
		}
	}
	return outs
}

// cppRunCheck runs the probe of the archive at path on the files of the build in dir, and requires every build of it to print, per file, `<name>: <want>`.
func cppRunCheck(t *testing.T, path string, filesOf func(dir string) []cppRunFile) {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := cppRunBuild(t, path)
	files := filesOf(dir)
	var want strings.Builder
	for _, f := range files {
		if !f.quiet {
			want.WriteString(f.name + ": " + f.want + "\n")
		}
	}
	for _, got := range cppRunOutputs(t, dir, cppRunWrite(t, dir, files)) {
		if got != want.String() {
			t.Errorf("got:\n%s\nwant:\n%s", got, want.String())
		}
	}
}

// cppRunSchema is the $schema of the data file at rel under dir, which a json emit of the build wrote.
func cppRunSchema(t *testing.T, dir, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	var head struct {
		Schema string `json:"$schema"`
	}
	if err := json.Unmarshal(raw, &head); err != nil || head.Schema == "" {
		t.Fatalf("no $schema in %s: %v", rel, err)
	}
	return head.Schema
}
