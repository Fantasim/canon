package integrity

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

// emptyGitRepo is a real checkout with no commits: `rev-parse --is-inside-work-tree` succeeds
// (it is a checkout) but every comparison with HEAD fails for real (HEAD is unborn).
func emptyGitRepo(t *testing.T) string {
	t.Helper()
	top := t.TempDir()
	if out, err := exec.Command("git", "-C", top, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return top
}

func run(t *testing.T, root string, rules ...string) lane.Result {
	t.Helper()
	on := map[string]bool{}
	for _, r := range rules {
		on[r] = true
	}
	res, err := New().Run(&lane.Context{Repo: &repo.Repo{Root: root}, Enabled: on})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res
}

func skipped(res lane.Result) []string {
	var what []string
	for _, s := range res.Skipped {
		what = append(what, s.What)
	}
	return what
}

// TestRunSkipsGitRulesOnUnbornHead proves a real git failure inside a checkout leaves the git
// rules unmeasured (a Skip, so check fails), never an empty finding set (log-2026-09-24,
// audit gate review calls: an unmeasured lane FAILs check).
func TestRunSkipsGitRulesOnUnbornHead(t *testing.T) {
	res := run(t, emptyGitRepo(t), ruleBaselineGuard, ruleDecision)
	if got, want := skipped(res), []string{skipGuard, skipDecision}; !slices.Equal(got, want) {
		t.Fatalf("Run().Skipped = %v, want %v", got, want)
	}
}

// TestRunSkipsNothingOutsideAGitCheckout proves the documented case (README.md: outside a git
// checkout the guard and decision-dropped have nothing to compare) stays a silent no-op.
func TestRunSkipsNothingOutsideAGitCheckout(t *testing.T) {
	res := run(t, t.TempDir(), ruleBaselineGuard, ruleDecision)
	if len(res.Skipped) != 0 || len(res.Findings) != 0 {
		t.Errorf("Run() = %+v, want nothing outside a git checkout", res)
	}
}

// TestRunGuardFilesAbsentAtRevision proves a file the guarded revision lacks is "nothing to
// compare", not a git failure: the guard runs, measured, with no finding.
func TestRunGuardFilesAbsentAtRevision(t *testing.T) {
	res := run(t, gitRepo(t, map[string]string{"README.md": "x\n"}), ruleBaselineGuard)
	if len(res.Skipped) != 0 || len(res.Findings) != 0 {
		t.Errorf("Run() = %+v, want measured and clean", res)
	}
}

// TestHeadText proves the three outcomes: present, absent at the revision, git failure.
func TestHeadText(t *testing.T) {
	top := gitRepo(t, map[string]string{repo.StateFile: "fmt\tenforce\n"})
	ctx := &lane.Context{Repo: &repo.Repo{Root: top}}
	if text, ok, err := headText(ctx, repo.StateFile); err != nil || !ok || text != "fmt\tenforce" {
		t.Errorf("headText(present) = %q, %v, %v", text, ok, err)
	}
	if text, ok, err := headText(ctx, repo.BaselineFile); err != nil || ok || text != "" {
		t.Errorf("headText(absent) = %q, %v, %v, want \"\", false, nil", text, ok, err)
	}
	unborn := &lane.Context{Repo: &repo.Repo{Root: emptyGitRepo(t)}}
	if _, ok, err := headText(unborn, repo.StateFile); err == nil || ok {
		t.Errorf("headText(unborn HEAD) = %v, %v, want an error", ok, err)
	}
}

// TestRunSkipsGuardOnMissingBlob proves a file the revision lists but git cannot show (its
// blob gone from the object store) is a git failure, a Skip, never "absent at the revision".
func TestRunSkipsGuardOnMissingBlob(t *testing.T) {
	top := gitRepo(t, map[string]string{repo.StateFile: "fmt\tenforce\n"})
	hash, err := exec.Command("git", "-C", top, "rev-parse", "HEAD:"+repo.StateFile).Output()
	if err != nil {
		t.Fatal(err)
	}
	h := strings.TrimSpace(string(hash))
	if err := os.Remove(filepath.Join(top, ".git", "objects", h[:2], h[2:])); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := headText(&lane.Context{Repo: &repo.Repo{Root: top}}, repo.StateFile); err == nil || ok {
		t.Errorf("headText(missing blob) = %v, %v, want an error", ok, err)
	}
	if got, want := skipped(run(t, top, ruleBaselineGuard)), []string{skipGuard}; !slices.Equal(got, want) {
		t.Fatalf("Run(missing blob).Skipped = %v, want %v", got, want)
	}
}

// TestRunSkipsGuardOnUnreadableBaseline proves a committed baseline whose working copy cannot
// be opened (a symlink loop) leaves baseline-guard unmeasured instead of passing it silently.
func TestRunSkipsGuardOnUnreadableBaseline(t *testing.T) {
	top := gitRepo(t, map[string]string{repo.BaselineFile: "# baseline\n"})
	onDisk := filepath.Join(top, filepath.FromSlash(repo.BaselineFile))
	if err := os.Remove(onDisk); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(onDisk), onDisk); err != nil {
		t.Fatal(err)
	}
	if got, want := skipped(run(t, top, ruleBaselineGuard)), []string{skipGuard}; !slices.Equal(got, want) {
		t.Fatalf("Run(unreadable baseline).Skipped = %v, want %v", got, want)
	}
}

// TestRunSkipsUnreadableState proves an unreadable state file leaves rule-state unmeasured,
// while a missing one is simply no finding.
func TestRunSkipsUnreadableState(t *testing.T) {
	root := t.TempDir()
	if got := run(t, root, ruleState); len(got.Skipped) != 0 || len(got.Findings) != 0 {
		t.Fatalf("Run(no state file) = %+v, want measured and clean", got)
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(repo.StateFile)), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if got, want := skipped(run(t, root, ruleState)), []string{skipState}; !slices.Equal(got, want) {
		t.Fatalf("Run(state file unreadable).Skipped = %v, want %v", got, want)
	}
}

// TestInGitCheckout proves the distinguishing check itself, real and fake directories both.
func TestInGitCheckout(t *testing.T) {
	if !inGitCheckout(&lane.Context{Repo: &repo.Repo{Root: emptyGitRepo(t)}}) {
		t.Error("inGitCheckout must be true inside a real checkout")
	}
	if inGitCheckout(&lane.Context{Repo: &repo.Repo{Root: t.TempDir()}}) {
		t.Error("inGitCheckout must be false outside one")
	}
}
