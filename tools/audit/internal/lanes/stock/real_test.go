package stock

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

// TestReal runs the lane on a real repo: CANON_AUDIT_REAL=1, CANON_AUDIT_REPO (default this
// project's root), CANON_AUDIT_DUMP (TSV dir). Any skip fails it. Opt-in: the gate's
// audit-check already runs this lane here; this would add a cold golangci-lint pass.
func TestReal(t *testing.T) {
	if os.Getenv("CANON_AUDIT_REAL") != "1" {
		t.Skip("set CANON_AUDIT_REAL=1 to audit a real repo")
	}
	root := os.Getenv("CANON_AUDIT_REPO")
	if root == "" {
		root = filepath.Join("..", "..", "..", "..", "..")
	}
	auditReal(t, root, toolchainDir(t))
}

func toolchainDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller info")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "toolchain")
}

func auditReal(t *testing.T, root, toolchain string) {
	ctx := realContext(t, root, toolchain)
	r := ctx.Repo
	ctx.Go, _ = gosrc.Parse(r.Root, r.FilesWithExt(repo.GoExt))
	ctx.Enabled, ctx.Log = map[string]bool{}, os.Stderr
	for _, id := range ruleIDs {
		if rl, ok := rules.Lookup(id); ok && r.Mode(*rl) != rules.Off {
			ctx.Enabled[id] = true
		}
	}

	cold := time.Now()
	res := measuredRun(t, ctx)
	t.Logf("%s: cold %d findings in %s", r.Name, len(res.Findings), time.Since(cold).Round(time.Millisecond))

	warm := time.Now()
	res2 := measuredRun(t, ctx)
	t.Logf("%s: warm %d findings in %s", r.Name, len(res2.Findings), time.Since(warm).Round(time.Millisecond))

	counts := map[string]int{}
	for _, f := range res.Findings {
		counts[f.Rule]++
	}
	for _, id := range ruleIDs {
		t.Logf("  %-20s %d", id, counts[id])
	}
	if dir := os.Getenv("CANON_AUDIT_DUMP"); dir != "" {
		dump(t, filepath.Join(dir, r.Name+".tsv"), res.Findings, ctx)
	}
}

// measuredRun runs the lane and fails on any skip: an unmeasured lane is a failure (DECISIONS 25).
func measuredRun(t *testing.T, ctx *lane.Context) lane.Result {
	t.Helper()
	res, err := New().Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) > 0 {
		t.Fatalf("%s: unmeasured: %+v", ctx.Repo.Name, res.Skipped)
	}
	return res
}

func dump(t *testing.T, path string, fs []finding.Finding, ctx *lane.Context) {
	t.Helper()
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Rule != fs[j].Rule {
			return fs[i].Rule < fs[j].Rule
		}
		if fs[i].File != fs[j].File {
			return fs[i].File < fs[j].File
		}
		return fs[i].Line < fs[j].Line
	})
	var b strings.Builder
	for _, f := range fs {
		sym := f.Symbol
		if sym == "" && f.Line > 0 && strings.HasSuffix(f.File, repo.GoExt) && ctx.Go != nil {
			sym = ctx.Go.Enclosing(f.File, f.Line)
		}
		b.WriteString(f.Rule + "\t" + f.File + "\t" + sym + "\t" + f.Detail + "\t" + f.Message + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), testFilePerm); err != nil {
		t.Fatal(err)
	}
}
