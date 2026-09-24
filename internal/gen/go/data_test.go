package gogen_test

import (
	"bytes"
	"go/version"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
)

const dataGoldens = "testdata/data"

// generateData runs the generator twice on each package, outputs keyed by module path (CODEGEN.md §2.7: equal both times).
func generateData(t *testing.T, pkgs ...*ir.Package) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, p := range pkgs {
		e := p.Emits[0]
		first, err := gogen.Generate(p, e)
		if err != nil {
			t.Fatalf("%s: %v", p.Name, err)
		}
		again, err := gogen.Generate(p, e)
		if err != nil {
			t.Fatalf("%s: %v", p.Name, err)
		}
		dir := strings.TrimPrefix(e.GoImport, dataModule+"/")
		for i, f := range first {
			if !bytes.Equal(f.Content, again[i].Content) {
				t.Errorf("%s: %s differs between two runs", p.Name, f.Path)
			}
			out[dir+"/"+f.Path] = f.Content
		}
	}
	return out
}

func shopFiles(t *testing.T) map[string][]byte {
	t.Helper()
	shop := newShop()
	return generateData(t, base(), shop)
}

// CODEGEN.md §5.9–§5.11, §6.1, §6.3: the pipeline's data-mode Go equals its golden.
func TestDataPipelineGolden(t *testing.T) {
	files := generateData(t, potionPipeline())
	checkGoldens(t, files, sortedPaths(files), filepath.Join(dataGoldens, "pipeline"))
}

// CODEGEN.md §4–§6, WIRE.md §5: every construct data mode reads equals its golden.
func TestDataShopGolden(t *testing.T) {
	files := shopFiles(t)
	checkGoldens(t, files, sortedPaths(files), filepath.Join(dataGoldens, "shop"))
}

// runData vets and tests with -race a module of the files, the smoke test and the data files (IMPLEMENTATION-PLAN.md §6 M2 items 3, 5).
func runData(t *testing.T, files map[string][]byte, pkgDir, smoke string, data map[string][]byte) {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	dir := t.TempDir()
	write := func(p string, content []byte) {
		path := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", []byte("module "+dataModule+"\n\ngo "+goVersion(t)+"\n"))
	for _, p := range sortedPaths(files) {
		write(p, files[p])
	}
	src, err := os.ReadFile(smoke)
	if err != nil {
		t.Fatal(err)
	}
	write(pkgDir+"/smoke_test.go", src)
	for _, p := range sortedPaths(data) {
		write(pkgDir+"/testdata/"+p, data[p])
	}
	if out := run(t, dir, exec.Command("gofmt", "-s", "-l", ".")); len(out) > 0 {
		t.Errorf("gofmt -s would change:\n%s", out)
	}
	run(t, dir, exec.Command("go", "vet", "./..."))
	run(t, dir, exec.Command("go", "test", "-race", "-count=1", "./..."))
}

// goVersion is the installed Go's language version: generated Go targets the current Go only (decision 205).
func goVersion(t *testing.T) string {
	lang := version.Lang(runtime.Version())
	if lang == "" {
		out, err := exec.Command("go", "env", "GOVERSION").Output()
		if err != nil {
			t.Fatal(err)
		}
		lang = version.Lang(strings.TrimSpace(string(out)))
	}
	return strings.TrimPrefix(lang, "go")
}

// readData reads the committed data files of one fixture, keyed by their path under it.
func readData(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		out[filepath.ToSlash(rel)] = b
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// M2 items 3, 5, WIRE.md §8.3: potions.json loads, another fingerprint is refused naming both, a failed Reload keeps the snapshot.
func TestDataPipelineRuns(t *testing.T) {
	potions, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "pipeline", "expected", "potions.json"))
	if err != nil {
		t.Fatal(err)
	}
	stale := bytes.Replace(potions, []byte(potionSchema), []byte("pipeline.Potion@00000000"), 1)
	data := map[string][]byte{"good/potions.json": potions, "stale/potions.json": stale}
	runData(t, generateData(t, potionPipeline()), "pipeline/out/go", "testdata/smoke/data_pipeline_test.go", data)
}

// CODEGEN.md §4–§6, WIRE.md §5: every construct loads, resolves and reports its failures.
func TestDataShopRuns(t *testing.T) {
	runData(t, shopFiles(t), "demo/shop/out/go", "testdata/smoke/data_shop_test.go", readData(t, "testdata/datafiles/shop"))
}
