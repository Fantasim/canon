package stock

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

const testFilePerm = 0o644

// fakeChangedRepo makes a throwaway git repo with the given files left untracked, so
// repo.SetChanged picks them up the same way it would real changed-but-uncommitted work.
func fakeChangedRepo(t *testing.T, changed []string) *repo.Repo {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fake\n\ngo 1.25\n"), testFilePerm); err != nil {
		t.Fatal(err)
	}
	git("add", "go.mod")
	git("commit", "-q", "-m", "init")
	for _, f := range changed {
		full := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(full), os.ModePerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("package x\n"), testFilePerm); err != nil {
			t.Fatal(err)
		}
	}
	r, err := repo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetChanged("HEAD"); err != nil {
		t.Fatal(err)
	}
	return r
}

func loadFixture(t *testing.T) []golangciIssue {
	t.Helper()
	data, err := os.ReadFile("testdata/golangci_issues.json")
	if err != nil {
		t.Fatal(err)
	}
	var issues []golangciIssue
	if err := json.Unmarshal(data, &issues); err != nil {
		t.Fatal(err)
	}
	return issues
}

func allOn() *lane.Context {
	on := map[string]bool{}
	for _, id := range ruleIDs {
		on[id] = true
	}
	return &lane.Context{Enabled: on}
}

func findOne(t *testing.T, fs []finding.Finding, rule, file string) finding.Finding {
	t.Helper()
	for _, f := range fs {
		if f.Rule == rule && f.File == file {
			return f
		}
	}
	t.Fatalf("no %s finding for %s among %d findings", rule, file, len(fs))
	return finding.Finding{}
}

func TestGolangciFindingsMapping(t *testing.T) {
	issues := loadFixture(t)
	fs := golangciFindings(allOn(), issues)

	cog := findOne(t, fs, ruleFnComplexity, "cmd/server/main.go")
	if cog.Value != 20 {
		t.Errorf("gocognit value = %d, want 20", cog.Value)
	}

	findOne(t, fs, ruleSecurity, "internal/config/constants.go")
	findOne(t, fs, ruleSecurity, "internal/tool/run.go")
	findOne(t, fs, ruleErrStyle, "internal/x/errors.go")
	findOne(t, fs, ruleNaming, "internal/x/names.go")
	findOne(t, fs, ruleStaticcheck, "internal/x/re.go")
	findOne(t, fs, ruleCtxFirst, "internal/y/handler.go")
	findOne(t, fs, ruleNaming, "internal/y/names.go")
	findOne(t, fs, ruleNaming, "internal/y/recv.go")
	findOne(t, fs, ruleErrUnwrapped, "internal/w/err.go")
	findOne(t, fs, ruleErrCompare, "internal/w/cmp.go")

	dup := findOne(t, fs, ruleDupInRepo, "a.go")
	if dup.Value != 65 {
		t.Errorf("dupl value = %d, want 65", dup.Value)
	}

	for _, f := range fs {
		if f.File == "internal/y/other.go" {
			t.Errorf("unrecognized revive rule should not produce a finding, got %+v", f)
		}
		if f.File == "internal/n/noise.go" {
			t.Errorf("unmapped linter should not produce a finding, got %+v", f)
		}
	}
}

func TestGolangciFindingsFmtDedupPerFile(t *testing.T) {
	fs := golangciFindings(allOn(), loadFixture(t))
	var oneGo, twoGo int
	for _, f := range fs {
		if f.Rule != ruleFmt {
			continue
		}
		switch f.File {
		case "internal/z/one.go":
			oneGo++
		case "internal/z/two.go":
			twoGo++
		}
	}
	if oneGo != 1 {
		t.Errorf("internal/z/one.go: %d fmt findings, want 1 (gofumpt+goimports collapse to one)", oneGo)
	}
	if twoGo != 1 {
		t.Errorf("internal/z/two.go: %d fmt findings, want 1", twoGo)
	}
}

func TestGolangciFindingsRespectEnabled(t *testing.T) {
	ctx := &lane.Context{Enabled: map[string]bool{ruleFnComplexity: true}}
	fs := golangciFindings(ctx, loadFixture(t))
	for _, f := range fs {
		if f.Rule != ruleFnComplexity {
			t.Errorf("rule %s should have been filtered out by ctx.Enabled, got %+v", f.Rule, f)
		}
	}
	if len(fs) != 1 {
		t.Errorf("got %d findings, want 1 (only fn-complexity is on)", len(fs))
	}
}

func TestWantedLinters(t *testing.T) {
	ctx := &lane.Context{Enabled: map[string]bool{ruleFnComplexity: true, ruleSecurity: true}}
	got := wantedLinters(ctx)
	if len(got) != 2 || got[0] != linterGocognit || got[1] != linterGosec {
		t.Errorf("wantedLinters = %v, want [gocognit gosec]", got)
	}
}

func TestGolangciArgsDisablesUnwanted(t *testing.T) {
	args := golangciArgs("cfg.yml", []string{linterGocognit}, []string{targetAll})
	found := false
	for i, a := range args {
		if a == argDisable {
			found = true
			if i+1 >= len(args) {
				t.Fatal("--disable with no value")
			}
		}
	}
	if !found {
		t.Errorf("args %v missing --disable for a narrow rule set", args)
	}
}

func TestGolangciArgsSkipsDisableWhenAllWanted(t *testing.T) {
	args := golangciArgs("cfg.yml", allLinters, []string{targetAll})
	for _, a := range args {
		if a == argDisable {
			t.Errorf("args %v should not --disable anything when every linter is wanted", args)
		}
	}
}

// TestGolangciFindingsDropsSkippedPaths guards against a regression like flatted.go: an npm
// package under node_modules/ that ships Go sources, which "./..." also matches so
// golangci-lint reports on it same as a repo source file.
func TestGolangciFindingsDropsSkippedPaths(t *testing.T) {
	issues := []golangciIssue{
		{FromLinter: linterGocognit, Text: "cognitive complexity 74 of func `Stringify` is high (> 15)", Pos: golangciPos{Filename: "web/node_modules/flatted/golang/pkg/flatted/flatted.go", Line: 16}},
		{FromLinter: linterGocognit, Text: "cognitive complexity 20 of func `run` is high (> 15)", Pos: golangciPos{Filename: "cmd/server/main.go", Line: 37}},
	}
	fs := golangciFindings(allOn(), issues)
	for _, f := range fs {
		if strings.Contains(f.File, "node_modules/") {
			t.Errorf("finding on a skipped path should have been dropped, got %+v", f)
		}
	}
	findOne(t, fs, ruleFnComplexity, "cmd/server/main.go")
	if len(fs) != 1 {
		t.Errorf("got %d findings, want 1 (the node_modules one dropped)", len(fs))
	}
}

func TestChangedGoDirsSkipsSkippedTreesAndNonGo(t *testing.T) {
	dirs := changedGoDirs(fakeChangedRepo(t, []string{"internal/a/x.go", "internal/a/y.go", "internal/b/z.go", "README.md", "vendor/x/x.go", "examples/pipeline/expected/go/x.gen.go"}))
	if len(dirs) != 2 || dirs[0] != "./internal/a" || dirs[1] != "./internal/b" {
		t.Errorf("changedGoDirs = %v, want [./internal/a ./internal/b]", dirs)
	}
}
