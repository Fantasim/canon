package tsgen_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

const (
	driversGlob = "testdata/drivers/*.test.js"
	jsonDir     = "json"
	dirPerm     = 0o755
	filePerm    = 0o644
)

// driverCase is the testdata/ts case a driver named <case>.test.js runs against.
func driverCase(driver string) string {
	return strings.TrimSuffix(filepath.Base(driver), ".test.js")
}

// CODEGEN.md §5.9, §5.13, §8, WIRE.md §5: the generated modules run under node. For each driver of testdata/drivers, its case is built, compiled and written beside the data files its json emit writes; the driver imports `./<case>/<path>.js` and reads `./<case>/json/<file>`.
func TestDrivers(t *testing.T) {
	tc := tsc.Find(t)
	drivers, err := filepath.Glob(driversGlob)
	if err != nil || len(drivers) == 0 {
		t.Fatalf("no driver: %v", err)
	}
	src := t.TempDir()
	type data struct {
		name  string
		files map[string][]byte
	}
	var jsons []data
	var paths []string
	for _, d := range drivers {
		name := driverCase(d)
		cases, err := golden.Load("testdata/ts/"+name+".txtar", golden.Expected(generatedTS))
		if err != nil {
			t.Fatal(err)
		}
		w := buildWorld(t, cases[0].Archive.Files)
		paths = append(paths, writeOutputs(t, filepath.Join(src, name), w.generate(t))...)
		jsons = append(jsons, data{name, w.generateJSON(t)})
	}
	out := compile(t, tc, src, paths...)
	for i, d := range drivers {
		body, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(out, filepath.Base(d)), body)
		for _, file := range slices.Sorted(maps.Keys(jsons[i].files)) {
			writeFile(t, filepath.Join(out, jsons[i].name, jsonDir, file), jsons[i].files[file])
		}
	}
	runTests(t, tc, out)
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, filePerm); err != nil {
		t.Fatal(err)
	}
}
