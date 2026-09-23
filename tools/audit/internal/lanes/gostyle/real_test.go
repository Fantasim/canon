package gostyle

import (
	"os"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

const (
	realEnv         = "CANON_AUDIT_REAL"
	realEnvOn       = "1"
	realRepoEnv     = "CANON_AUDIT_REPO"
	defaultRealRepo = "../../../../.."
	realSampleCount = 5
)

// TestReal runs the gostyle lane against a real repo so its findings can be read and tuned
// against actual code. Skipped unless CANON_AUDIT_REAL=1; CANON_AUDIT_REPO picks the repo
// (default this project's root).
func TestReal(t *testing.T) {
	if os.Getenv(realEnv) != realEnvOn {
		t.Skip("set CANON_AUDIT_REAL=1 to run against a real repo")
	}
	repoPath := os.Getenv(realRepoEnv)
	if repoPath == "" {
		repoPath = defaultRealRepo
	}
	r, err := repo.Open(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	gofiles := r.FilesWithExt(goSuffix)
	tree, perrs := gosrc.Parse(r.Root, gofiles)
	for _, e := range perrs {
		t.Logf("parse error: %v", e)
	}
	ctx := &lane.Context{Repo: r, Go: tree, Enabled: enabledAll(), Limits: shippedLimits(t)}
	fs, skips := lane.Run(ctx, []lane.Lane{New()})
	for _, s := range skips {
		t.Logf("skip: %+v", s)
	}
	t.Logf("%s: %d Go files, %d findings", r.Name, len(gofiles), len(fs))
	logSamples(t, fs)
}

func logSamples(t *testing.T, fs []finding.Finding) {
	t.Helper()
	counts := countByRule(fs)
	for _, id := range ruleIDs {
		t.Logf("%s: %d findings", id, counts[id])
		logRuleSamples(t, fs, id)
	}
}

func logRuleSamples(t *testing.T, fs []finding.Finding, id string) {
	t.Helper()
	shown := 0
	for _, f := range fs {
		if f.Rule != id || shown >= realSampleCount {
			continue
		}
		t.Logf("  %s:%d %s %q | %s", f.File, f.Line, f.Symbol, f.Detail, f.Message)
		shown++
	}
}
