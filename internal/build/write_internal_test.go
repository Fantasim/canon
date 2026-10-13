package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// API.md §10.3 (log-2026-09-29 M4 U8-r): the OS temporary name is hidden, new, and holds the base.
func TestTempNameKeepsBase(t *testing.T) {
	stage := filepath.Join("d", ".x.canon.canon-edit")
	a, errA := tempName(stage)
	b, errB := tempName(stage)
	if errA != nil || errB != nil || a == b {
		t.Fatalf("%q, %q: %v, %v", a, b, errA, errB)
	}
	base := filepath.Base(a)
	if filepath.Dir(a) != "d" || !strings.HasPrefix(base, tempPrefix) || !strings.Contains(base, ".x.canon.canon-edit") ||
		!strings.HasSuffix(base, tempSuffix) || len(base) > len(".x.canon.canon-edit")+len(tempPrefix+tempSep+tempSuffix)+tokenLen {
		t.Errorf("temporary name %q", base)
	}
}

// DECISIONS 201: an --adopt output under --check is reported and counted as stale, like every
// other would-be change; nothing is written.
func TestCommitCheckMarksAdoptedStale(t *testing.T) {
	r := &run{}
	out := &BuildResult{}
	outputs := []*output{{Output: Output{Path: "@source/x.h", Status: StatusAdopted}}}
	if err := r.commit(true, out, outputs, nil); err != nil {
		t.Fatal(err)
	}
	if !out.Stale || out.Outputs[0].Status != StatusStale {
		t.Errorf("check-mode adopt: stale %t, status %v", out.Stale, out.Outputs[0].Status)
	}
}

// DECISIONS 351: a token is 13 base32 characters, new each run; a staging name is recognised
// with any token, and with 0.1.5's fixed name, for its own file only.
func TestStagingNames(t *testing.T) {
	a, errA := newToken()
	b, errB := newToken()
	if errA != nil || errB != nil || a == b || len(a) != tokenLen || strings.Trim(a, tokenAlphabet) != "" {
		t.Fatalf("tokens %q, %q: %v, %v", a, b, errA, errB)
	}
	for _, c := range []struct {
		name string
		want bool
	}{
		{".x.json." + a + ".canon-tmp", true},
		{".x.json.canon-tmp", true},
		{".x.json." + strings.ToLower(a) + ".canon-tmp", false},
		{".x.json.ABC.canon-tmp", false},
		{".y.x.json." + a + ".canon-tmp", false},
		{a + ".canon-tmp", false},
		{"x.json", false},
	} {
		if got := stagedOf(c.name, "x.json"); got != c.want {
			t.Errorf("DECISIONS 351: %q of x.json: %v", c.name, got)
		}
	}
	if got := stagedAs("/d/x.json", a); got != "/d/.x.json."+a+".canon-tmp" {
		t.Errorf("DECISIONS 351: staged as %q", got)
	}
}

// ADR-0010, DECISIONS 351: a link planted at a run's staging name is removed, never written through.
func TestPlantedStageLinkRemoved(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.ToSlash(filepath.Join(dir, "a.canon"))
	if err := os.WriteFile(abs, []byte("old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "target.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Staging{token: "AAAAAAAAAAAAA"}
	if os.Symlink(outside, filepath.FromSlash(stagedAs(abs, s.token))) != nil {
		t.Skip("no symbolic links here")
	}
	if err := s.WriteAtomic(OS(), abs, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "outside\n" {
		t.Errorf("the link's target holds %q, %v", got, err)
	}
	info, err := os.Lstat(abs)
	got, _ := os.ReadFile(abs)
	list, _ := os.ReadDir(dir)
	if err != nil || !info.Mode().IsRegular() || string(got) != "new\n" || len(list) != 1 {
		t.Errorf("output %q (%v, %v), %d entries", got, info, err, len(list))
	}
}
