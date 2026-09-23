package determinism

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

// runFixture runs the lane on testdata/repo through the real runner.
func runFixture(t *testing.T) ([]finding.Finding, []lane.Skip) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "repo"))
	if err != nil {
		t.Fatal(err)
	}
	var rel []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, repo.GoExt) {
			r, _ := filepath.Rel(root, p)
			rel = append(rel, filepath.ToSlash(r))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(rel)
	tree, perrs := gosrc.Parse(root, rel)
	if len(perrs) > 0 {
		t.Fatal(perrs)
	}
	ctx := &lane.Context{
		Repo: &repo.Repo{Root: root, Name: "canon", Module: "example.com/canon"}, Go: tree,
		Enabled: map[string]bool{ruleMapRange: true},
	}
	return lane.Run(ctx, []lane.Lane{New()})
}

// IMPLEMENTATION-PLAN.md §7.5: unmarked map ranges of output packages, bad markers; no more.
func TestFixtureFindings(t *testing.T) {
	fs, skips := runFixture(t)
	for _, s := range skips {
		t.Errorf("skip: %+v", s)
	}
	var got []string
	for _, f := range fs {
		got = append(got, fmt.Sprintf("%s:%d %s %s", f.File, f.Line, f.Symbol, f.Detail))
	}
	slices.Sort(got)
	want := []string{
		"api/api.go:6 Keys map range",
		"internal/check/check.go:13 Sum stale marker",
		"internal/gen/go/gen.go:5 Keys map range",
		"internal/ir/ir.go:13 Ranges map range",
		"internal/ir/ir.go:16 Ranges map range",
		"internal/ir/ir.go:19 Ranges maps iterator",
		"internal/ir/ir.go:22 Ranges maps iterator",
		"internal/ir/ir.go:38 Ranges marker without reason",
		"internal/ir/ir.go:42 Ranges stale marker",
		"internal/ir/ir.go:47 Ranges map range",
		"internal/ir/ir_test.go:6 TestRanges map range",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

var reOutputList = regexp.MustCompile("(?s)`maprange`.*? in the packages (.*?) and the packages under them")

var reBackticked = regexp.MustCompile("`([a-z0-9/]+)`")

// The lane judges the output packages IMPLEMENTATION-PLAN.md §7.5 names.
func TestOutputDirsFollowThePlan(t *testing.T) {
	plan, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "spec", "IMPLEMENTATION-PLAN.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := reOutputList.FindSubmatch(plan)
	if m == nil {
		t.Fatal("IMPLEMENTATION-PLAN.md §7.5 names no output packages for maprange")
	}
	var named []string
	for _, b := range reBackticked.FindAllSubmatch(m[1], -1) {
		dir := "internal/" + string(b[1])
		if string(b[1]) == "api" {
			dir = "api"
		}
		named = append(named, dir)
	}
	if !slices.Equal(named, outputDirs) {
		t.Errorf("§7.5 names %v, the lane judges %v", named, outputDirs)
	}
}
