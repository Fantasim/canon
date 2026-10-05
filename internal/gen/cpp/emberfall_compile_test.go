package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// emberTree writes game.tone, game.core and game.world (in mode m), and game.camp beside a data world, at their outputs under dir, then the driver as main.cpp, and returns the sources to compile.
func emberTree(t *testing.T, dir string, m ir.Mode, driver string) []string {
	t.Helper()
	var sources []string
	packages := []*ir.Package{emberTone(), emberCoreFor(m), emberWorld(m)()}
	if m == ir.ModeData {
		packages = append(packages, emberCamp())
	}
	for _, p := range packages {
		e := cppEmit(p)
		out := filepath.Join(dir, filepath.FromSlash(e.Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, p))
		segs := strings.Split(p.Name, ".")
		sources = append(sources, e.Dir+"/"+segs[len(segs)-1]+".gen.cpp")
	}
	copyFile(t, filepath.Join("testdata", "main", driver), filepath.Join(dir, "main.cpp"), same)
	return append(sources, "main.cpp")
}

// CODEGEN.md §2.8 Literals, §5.9, §5.10, §5.14, DECISIONS 323: baked literals through the owner's hooks.
func TestEmberfallBakedCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sources := emberTree(t, dir, ir.ModeBaked, "emberfall_baked_main.cpp")
	for _, out := range buildAndRun(t, dir, sources) {
		if !strings.HasSuffix(out, "failures: 0\n") {
			t.Errorf("driver output:\n%s", out)
		}
	}
}

// CODEGEN.md §2.8, §5.13, §5.14: a types-mode decoder reads another package's classes with its readers.
func TestEmberfallTypesCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sources := emberTree(t, dir, ir.ModeTypes, "emberfall_types_main.cpp")
	for _, out := range buildAndRun(t, dir, sources) {
		if !strings.HasSuffix(out, "failures: 0\n") || !strings.Contains(out, "$.levels.status: no entry gone\n") {
			t.Errorf("driver output:\n%s", out)
		}
	}
}

// CODEGEN.md §2.8 Readers and Refs, §5.9, §7.6, DECISIONS 323: readers, hooks, `no entry <key>`.
func TestEmberfallDataCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sources := emberTree(t, dir, ir.ModeData, "emberfall_data_main.cpp")
	data := filepath.Join("testdata", "main", "emberfall")
	for _, f := range []string{"zones.json", "bands.json", "start.json", "bad/bands.json", "bad/zones.json", "bad/cells.json", "bad/pairs.json", "camp.json"} {
		copyFile(t, filepath.Join(data, filepath.FromSlash(f)), filepath.Join(dir, "files", filepath.FromSlash(f)), same)
	}
	files := filepath.Join(dir, "files")
	for _, out := range buildAndRun(t, dir, sources, files, filepath.Join(files, "bad")) {
		if !strings.HasSuffix(out, "failures: 0\n") || !strings.Contains(out, "rows[0].status: no entry nowhere\n") ||
			!strings.Contains(out, "rows[0].gear.kind: no entry zz\n") || !strings.Contains(out, "rows[0].gear.$kindFor.soft: no entry yy\n") ||
			!strings.Contains(out, "rows[0].stat0: no entry nope\n") {
			t.Errorf("driver output:\n%s", out)
		}
	}
}
