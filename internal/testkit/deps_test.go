package testkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag/catalog"
)

const (
	modulePath  = "github.com/fantasim/canonlang"
	moduleRoot  = "../.."
	planPath    = "../../spec/IMPLEMENTATION-PLAN.md"
	testkitName = "testkit"
)

// listedPackage is the part of `go list -json` the rule reads.
type listedPackage struct {
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
	DepOnly      bool
	Module       *struct{ Path string }
}

// IMPLEMENTATION-PLAN.md §3 and §11: `go list -deps` of the module follows the allowed graph.
func TestDependencyRule(t *testing.T) {
	plan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := catalog.Packages(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range violations(rows, libraries(string(plan)), goList(t)) {
		t.Error(v)
	}
}

// IMPLEMENTATION-PLAN.md §3: `api/vm` has its own row, above the view packages and `api`.
func TestViewModelRow(t *testing.T) {
	plan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := catalog.Packages(plan)
	if err != nil {
		t.Fatal(err)
	}
	vm := rankOf(rows, "api/vm")
	if vm < 0 || rows[vm].Dir != "api/vm" {
		t.Fatalf("api/vm is in row %d, want its own row", vm)
	}
	for _, dir := range []string{"internal/views", "internal/gen/view", "api"} {
		if r := rankOf(rows, dir); r <= vm {
			t.Errorf("%s (row %d) is not below api/vm (row %d)", dir, r, vm)
		}
	}
}

// IMPLEMENTATION-PLAN.md §3: views' sub-packages rank with views (as eval/std), so edit may import views/shape.
func TestViewsSubPackagesRankWithViews(t *testing.T) {
	plan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := catalog.Packages(plan)
	if err != nil {
		t.Fatal(err)
	}
	views := rankOf(rows, "internal/views")
	for _, dir := range []string{"internal/views/control", "internal/views/encode", "internal/views/rules", "internal/views/shape", "internal/views/typedef"} {
		if r := rankOf(rows, dir); r != views {
			t.Errorf("%s is in row %d, want views' row %d", dir, r, views)
		}
	}
	if edit := rankOf(rows, "internal/edit"); edit <= views {
		t.Errorf("edit (row %d) is not below views (row %d)", edit, views)
	}
}

// The rule itself: each kind of violation of a small synthetic graph is reported.
func TestDependencyRuleReportsViolations(t *testing.T) {
	rows := []catalog.Package{
		{Name: "source", Dir: "internal/source"},
		{Name: "eval", Dir: "internal/eval", Subs: []string{"internal/eval/std"}},
		{Name: "testkit", Dir: "internal/testkit"},
	}
	pkg := func(dir string, imports ...string) listedPackage {
		return listedPackage{ImportPath: modulePath + "/" + dir, Imports: imports, Module: &struct{ Path string }{modulePath}}
	}
	graph := []listedPackage{
		pkg("internal/source", modulePath+"/internal/eval"),
		pkg("internal/eval", modulePath+"/internal/source", modulePath+"/internal/eval/std", "fmt", "github.com/google/go-cmp/cmp"),
		pkg("internal/eval/std", modulePath+"/internal/eval", "unsafe"),
		pkg("internal/stray"),
		pkg("internal/testkit/golden", modulePath+"/internal/eval", "github.com/evil/lib"),
		{ImportPath: modulePath + "/internal/source", XTestImports: []string{"github.com/evil/test"}, Module: &struct{ Path string }{modulePath}},
	}
	got := violations(rows, []string{"github.com/google/go-cmp"}, graph)
	want := []string{"internal/source imports internal/eval", "internal/eval/std imports unsafe", "internal/stray is in no row", "github.com/evil/lib", "github.com/evil/test"}
	if len(got) != len(want) {
		t.Fatalf("got %d violations, want %d: %q", len(got), len(want), got)
	}
	for i, w := range want {
		if !strings.Contains(got[i], w) {
			t.Errorf("violation %d = %q, want it to mention %q", i, got[i], w)
		}
	}
}

func goList(t *testing.T) []listedPackage {
	t.Helper()
	statSources(t)
	cmd := exec.Command("go", "list", "-deps", "-json", "./...")
	cmd.Dir = moduleRoot
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var pkgs []listedPackage
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for {
		var p listedPackage
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			return pkgs
		} else if err != nil {
			t.Fatal(err)
		}
		pkgs = append(pkgs, p)
	}
}

// violations checks every module package: it has a row; its imports point up the table or
// into its row (testkit excepted); no cgo, no unsafe; every third-party import, tests
// included, is a listed library.
func violations(rows []catalog.Package, libs []string, pkgs []listedPackage) []string {
	var out []string
	for _, p := range pkgs {
		if p.DepOnly || p.Module == nil || p.Module.Path != modulePath {
			continue
		}
		dir := relDir(p.ImportPath)
		rank := rankOf(rows, dir)
		if rank < 0 {
			out = append(out, dir+" is in no row of the package table")
			continue
		}
		for _, imp := range p.Imports {
			if r := rankOf(rows, relDir(imp)); isModule(imp) && r > rank && rows[rank].Name != testkitName {
				out = append(out, fmt.Sprintf("%s imports %s, which is listed below it", dir, relDir(imp)))
			}
		}
		out = append(out, foreign(dir, libs, slices.Concat(p.Imports, p.TestImports, p.XTestImports))...)
	}
	return out
}

// foreign reports cgo, unsafe and every third-party import that is not a listed library.
func foreign(dir string, libs, imports []string) []string {
	var out []string
	for _, imp := range imports {
		first, _, _ := strings.Cut(imp, "/")
		switch {
		case imp == "C" || imp == "unsafe":
			out = append(out, fmt.Sprintf("%s imports %s", dir, imp))
		case isModule(imp) || !strings.Contains(first, "."):
		case !slices.ContainsFunc(libs, func(l string) bool { return imp == l || strings.HasPrefix(imp, l+"/") }):
			out = append(out, fmt.Sprintf("%s imports %s, which is not a library of IMPLEMENTATION-PLAN.md §11", dir, imp))
		}
	}
	return out
}

// rankOf is the row of the package table a module-relative directory belongs to: the row
// whose directory or sub-package is its longest prefix; -1 for none.
func rankOf(rows []catalog.Package, dir string) int {
	best, bestLen := -1, 0
	for i, r := range rows {
		for _, d := range append([]string{r.Dir}, r.Subs...) {
			if (dir == d || strings.HasPrefix(dir, d+"/")) && len(d) > bestLen {
				best, bestLen = i, len(d)
			}
		}
	}
	return best
}

// statSources stats every Go file of the module, so that `go test` caches this test's result
// only as long as no source changed: the output of the go list command is not tracked.
func statSources(t *testing.T) {
	t.Helper()
	err := filepath.WalkDir(moduleRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != moduleRoot && (strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata" || isNestedModule(p)) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") {
			_, err = os.Stat(p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func isNestedModule(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

func isModule(imp string) bool { return strings.HasPrefix(imp, modulePath+"/") }

func relDir(imp string) string { return strings.TrimPrefix(imp, modulePath+"/") }

var reLibrary = regexp.MustCompile("`([a-z0-9.-]+\\.[a-z]+/[^`]+)`")

// libraries lists the module paths of the Choice column of the libraries table.
func libraries(plan string) []string {
	_, rest, _ := strings.Cut(plan, "\n## 11. Libraries")
	section, _, _ := strings.Cut(rest, "\n## ")
	var out []string
	for _, line := range strings.Split(section, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}
		for _, m := range reLibrary.FindAllStringSubmatch(cells[2], -1) {
			out = append(out, m[1])
		}
	}
	return out
}
