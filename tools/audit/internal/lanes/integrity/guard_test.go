package integrity

import (
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
	if got := guardRev(git, "base"); got != "HEAD" {
		t.Fatalf("ratchet-only re-record: guardRev = %q, want HEAD", got)
	}
}

func TestGuardRevKeepsBaseForMixedCommits(t *testing.T) {
	git := fakeGit(map[string]string{"a1": ".sovaudit/baseline.tsv", "b2": ".sovaudit/baseline.tsv\ninternal/x.go"})
	if got := guardRev(git, "base"); got != "base" {
		t.Fatalf("baseline loosened alongside code: guardRev = %q, want base", got)
	}
}

func TestGuardRevNoRatchetCommits(t *testing.T) {
	if got := guardRev(fakeGit(nil), "base"); got != "base" {
		t.Fatalf("no ratchet commits: guardRev = %q, want base", got)
	}
}
