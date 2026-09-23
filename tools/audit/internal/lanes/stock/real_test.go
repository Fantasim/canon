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
// project's root), CANON_AUDIT_DUMP (dir for one TSV of findings).
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
	r, err := repo.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	tree, _ := gosrc.Parse(r.Root, r.FilesWithExt(repo.GoExt))
	on := map[string]bool{}
	for _, id := range ruleIDs {
		if rl, ok := rules.Lookup(id); ok && r.Mode(*rl) != rules.Off {
			on[id] = true
		}
	}
	ctx := &lane.Context{Repo: r, Go: tree, Enabled: on, Toolchain: toolchain, Log: os.Stderr}

	cold := time.Now()
	res, err := New().Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: cold %d findings in %s, skipped %v", r.Name, len(res.Findings), time.Since(cold).Round(time.Millisecond), res.Skipped)

	warm := time.Now()
	res2, err := New().Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
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
