package testkit

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag/catalog"
)

const (
	planTableHeader = "| Package (under `internal/` unless noted) |"
	allAbove        = "all of the above"
	proseCell       = "internal packages"
	planSection     = "\n## 3. Package map"
	sectionEnd      = "\n## "
	consumesCell    = 4 // the cells of a row split on "|": "", package, owns, implements, consumes
	minTableCells   = consumesCell + 1
)

var (
	reWord  = regexp.MustCompile(`[a-z][a-z0-9/]*`)
	reParen = regexp.MustCompile(`\([^)]*\)`)
)

// Why an import is allowed though §3's Consumes column does not license it.
const (
	reasonVM    = "api/vm sits above views in §3 but the Consumes cells of views and gen/view omit it: spec sync"
	reasonLock  = "lock/sources.go reads check and verify; §3 gives lock value and project only"
	reasonLoad  = "load.Request.Through and evalHost bind wire.Host to *eval.Evaluator: move to build, eval's tests use it"
	reasonLive  = "views/live reads verify; §3 gives views check, eval and i18n"
	reasonTable = "views/table reads wire.KeyID (`$id`); a copy in views is a const-dup: a lower home is owed"
)

// consumesAllowlist is shrink-only (log M4 P13c-r): "importer -> imported" and why; a stale entry fails.
var consumesAllowlist = map[string]string{
	"internal/gen/view -> api/vm":            reasonVM,
	"internal/views -> api/vm":               reasonVM,
	"internal/views/control -> api/vm":       reasonVM,
	"internal/views/encode -> api/vm":        reasonVM,
	"internal/views/layout -> api/vm":        reasonVM,
	"internal/views/live -> api/vm":          reasonVM,
	"internal/views/table -> api/vm":         reasonVM,
	"internal/views/typedef -> api/vm":       reasonVM,
	"internal/views/live -> internal/verify": reasonLive,
	"internal/views/table -> internal/wire":  reasonTable,
	"internal/load -> internal/eval":         reasonLoad,
	"internal/lock -> internal/check":        reasonLock,
	"internal/lock -> internal/verify":       reasonLock,
}

// IMPLEMENTATION-PLAN.md §3, log M4 P13c-r: a package imports what its Consumes cell lists or reaches.
func TestConsumesColumn(t *testing.T) {
	plan := readPlan(t)
	rows, err := catalog.Packages(plan)
	if err != nil {
		t.Fatal(err)
	}
	consumes := consumesOf(string(plan), rows)
	found := map[string]bool{}
	for _, v := range consumesViolations(rows, consumes, goList(t)) {
		if _, ok := consumesAllowlist[v]; ok {
			found[v] = true
			continue
		}
		t.Errorf("%s: not in the Consumes column of its row, nor reachable through it", v)
	}
	for entry, reason := range consumesAllowlist {
		if !found[entry] {
			t.Errorf("allowlist entry %q (%s) matches no import any more: remove it", entry, reason)
		}
	}
}

// The rule itself: direct, own-row, reachable and unlisted imports of a small synthetic table.
func TestConsumesViolations(t *testing.T) {
	rows := []catalog.Package{
		{Name: "a", Dir: "internal/a"},
		{Name: "b", Dir: "internal/b"},
		{Name: "c", Dir: "internal/c", Subs: []string{"internal/c/sub"}},
		{Name: "d", Dir: "internal/d"},
		{Name: "e", Dir: "internal/e"},
		{Name: testkitName, Dir: "internal/testkit"},
	}
	consumes := closure([][]int{nil, {0}, {1}, {0}, {0, 1, 2, 3}, nil})
	mod := &struct{ Path string }{modulePath}
	pkg := func(dir string, imports ...string) listedPackage {
		full := make([]string, len(imports))
		for i, imp := range imports {
			full[i] = modulePath + "/" + imp
		}
		return listedPackage{ImportPath: modulePath + "/" + dir, Imports: full, Module: mod}
	}
	graph := []listedPackage{
		pkg("internal/c", "internal/b", "internal/a", "internal/c/sub", "internal/d"),
		pkg("internal/c/sub", "internal/c"),
		pkg("internal/e", "internal/a", "internal/d"),
		pkg("internal/testkit/golden", "internal/c", "internal/b"),
	}
	got := consumesViolations(rows, consumes, graph)
	want := []string{"internal/c -> internal/d"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// IMPLEMENTATION-PLAN.md §3: the Consumes cells of a synthetic table, read as the test reads the plan.
func TestConsumesOfReadsTheTable(t *testing.T) {
	plan := `intro

## 3. Package map

| Package (under ` + "`internal/`" + ` unless noted) | Owns | Implements | Consumes |
|---|---|---|---|
| ` + "`a`" + ` | x | y | — |
| ` + "`check`" + ` | x | y | a |
| ` + "`b`" + ` | x | y | a, unknown, words |
| ` + "`build`" + ` | x | y | all of the above |
| ` + "`edit`" + ` | x | y | b (re-check) |
| ` + "`cli`" + ` | x | y | internal packages for fmt, a |
| ` + "`api/`" + ` | x | y | b |

## 4. Elsewhere

| ` + "`b`" + ` | x | y | check |
`
	rows := []catalog.Package{
		{Name: "a", Dir: "internal/a"}, {Name: "check", Dir: "internal/check"}, {Name: "b", Dir: "internal/b"},
		{Name: "build", Dir: "internal/build"}, {Name: "edit", Dir: "internal/edit"},
		{Name: "cli", Dir: "internal/cli"}, {Name: "api", Dir: "api"},
	}
	want := [][]int{nil, {0}, {0}, {0, 1, 2}, {0, 2}, {0, 1, 2, 3, 4}, {0, 2}}
	got := consumesOf(plan, rows)
	for i := range rows {
		if !slices.Equal(got[i], want[i]) {
			t.Errorf("%s consumes %v, want %v", rows[i].Name, got[i], want[i])
		}
	}
}

// consumesOf reads the Consumes column of the package table: for each row, the rows it may
// import, transitively (the rows it lists, and everything reachable through them).
func consumesOf(plan string, rows []catalog.Package) [][]int {
	direct := make([][]int, len(rows))
	_, rest, _ := strings.Cut(plan, planSection)
	section, _, _ := strings.Cut(rest, sectionEnd)
	for _, line := range strings.Split(section, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < minTableCells || strings.HasPrefix(line, planTableHeader) {
			continue
		}
		row := rowNamed(rows, cells)
		if row < 0 {
			continue
		}
		cell := reParen.ReplaceAllString(cells[consumesCell], "")
		// IMPLEMENTATION-PLAN.md §3, log Cleanup-B-r: cli's prose cell means every row above it.
		if strings.Contains(cell, allAbove) || strings.Contains(cell, proseCell) {
			for i := range rows[:row] {
				direct[row] = append(direct[row], i)
			}
		}
		for _, w := range reWord.FindAllString(cell, -1) {
			if i := slices.IndexFunc(rows, func(r catalog.Package) bool { return r.Name == w }); i >= 0 {
				direct[row] = append(direct[row], i)
			}
		}
	}
	return closure(direct)
}

// rowNamed is the index of the row a table line describes, or -1 when it is no package row.
func rowNamed(rows []catalog.Package, cells []string) int {
	first, _, _ := strings.Cut(strings.TrimPrefix(strings.TrimSpace(cells[1]), "`"), "`")
	name := strings.TrimSuffix(first, "/")
	return slices.IndexFunc(rows, func(r catalog.Package) bool { return r.Name == name })
}

// closure makes each row's list transitive.
func closure(direct [][]int) [][]int {
	out := make([][]int, len(direct))
	for i := range direct {
		seen := map[int]bool{}
		stack := slices.Clone(direct[i])
		for len(stack) > 0 {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if !seen[n] {
				seen[n] = true
				stack = append(stack, direct[n]...)
			}
		}
		for n := range seen { //canon:unordered the list is sorted below
			out[i] = append(out[i], n)
		}
		slices.Sort(out[i])
	}
	return out
}

// consumesViolations lists "importer -> imported" for every direct import of a module package
// that is neither in the importer's row nor in the closure of its Consumes column.
func consumesViolations(rows []catalog.Package, consumes [][]int, pkgs []listedPackage) []string {
	var out []string
	for _, p := range pkgs {
		if p.DepOnly || p.Module == nil || p.Module.Path != modulePath {
			continue
		}
		dir := relDir(p.ImportPath)
		rank := rankOf(rows, dir)
		// IMPLEMENTATION-PLAN.md §3: "testkit, which may import anything", log Cleanup-B-r.
		if rank < 0 || rows[rank].Name == testkitName {
			continue
		}
		for _, imp := range p.Imports {
			r := rankOf(rows, relDir(imp))
			if isModule(imp) && r >= 0 && r != rank && !slices.Contains(consumes[rank], r) {
				out = append(out, fmt.Sprintf("%s -> %s", dir, relDir(imp)))
			}
		}
	}
	return out
}

func readPlan(t *testing.T) []byte {
	t.Helper()
	plan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}
