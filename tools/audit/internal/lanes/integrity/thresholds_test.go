package integrity

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

// gitRepo commits files (path -> content) in a fresh repository and returns its top level.
func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	top := t.TempDir()
	for p, content := range files {
		abs := filepath.Join(top, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "base"},
	} {
		cmd := exec.Command("git", append([]string{"-C", top}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return top
}

// The audited repo is a subdirectory of git's top level, as tools/audit auditing itself.
func TestThresholdsRaisedInSubdirRepo(t *testing.T) {
	top := gitRepo(t, map[string]string{"tools/audit/" + threshold.FileName: "fn-lines\t60\nfn-params\t5\n"})
	root := filepath.Join(top, "tools", "audit")
	ctx := &lane.Context{
		Repo:       &repo.Repo{Root: root},
		Limits:     threshold.Set{FnLines: 70, FnParams: 4},
		LimitsFile: filepath.Join(root, threshold.FileName),
	}
	got := thresholdsRaised(ctx)
	if len(got) != 1 || got[0].Detail != "fn-lines" || got[0].File != threshold.FileName {
		t.Fatalf("thresholdsRaised = %+v, want only fn-lines raised in %s", got, threshold.FileName)
	}
}

func TestThresholdsOutsideRepoNotGuarded(t *testing.T) {
	top := gitRepo(t, map[string]string{"README.md": "x\n"})
	ctx := &lane.Context{
		Repo:       &repo.Repo{Root: top},
		Limits:     threshold.Set{FnLines: 70},
		LimitsFile: filepath.Join(t.TempDir(), threshold.FileName),
	}
	if got := thresholdsRaised(ctx); len(got) != 0 {
		t.Fatalf("thresholdsRaised = %+v, want none for a file outside the repo", got)
	}
}
