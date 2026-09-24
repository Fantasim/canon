package integrity

import (
	"errors"
	"strings"
	"testing"
)

func fakeGit(commits map[string]string) func(...string) (string, error) {
	return func(args ...string) (string, error) {
		if args[0] == gitLog {
			var hs []string
			for h := range commits {
				hs = append(hs, h)
			}
			return strings.Join(hs, "\n"), nil
		}
		return commits[args[len(args)-1]], nil
	}
}

func TestGuardRevAcceptsRatchetOnlyCommits(t *testing.T) {
	git := fakeGit(map[string]string{"a1": ".sovaudit/baseline.tsv\n.sovaudit/state.tsv\ntools/audit/.sovaudit/baseline.tsv"})
	if got, err := guardRev(git, "base"); err != nil || got != "HEAD" {
		t.Fatalf("ratchet-only re-record: guardRev = %q, want HEAD", got)
	}
}

func TestGuardRevKeepsBaseForMixedCommits(t *testing.T) {
	git := fakeGit(map[string]string{"a1": ".sovaudit/baseline.tsv", "b2": ".sovaudit/baseline.tsv\ninternal/x.go"})
	if got, err := guardRev(git, "base"); err != nil || got != "base" {
		t.Fatalf("baseline loosened alongside code: guardRev = %q, want base", got)
	}
}

func TestGuardRevNoRatchetCommits(t *testing.T) {
	if got, err := guardRev(fakeGit(nil), "base"); err != nil || got != "base" {
		t.Fatalf("no ratchet commits: guardRev = %q, want base", got)
	}
}

// TestGuardRevGitFailure proves a git failure is an error, not a silent fall back to base
// (log-2026-09-24, audit gate review calls: an unmeasured lane FAILs check).
func TestGuardRevGitFailure(t *testing.T) {
	boom := errors.New("boom")
	for _, failing := range []string{gitLog, gitShow} {
		git := func(args ...string) (string, error) {
			if args[0] == failing {
				return "", boom
			}
			return fakeGit(map[string]string{"a1": ".sovaudit/baseline.tsv"})(args...)
		}
		if got, err := guardRev(git, "base"); !errors.Is(err, boom) {
			t.Errorf("git %s failing: guardRev = %q, %v, want %v", failing, got, err, boom)
		}
	}
}
