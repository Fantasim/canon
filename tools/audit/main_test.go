package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

const fakeFilePerm = 0o644

// TestUnmeasured proves the gate's own name-the-lanes message (DECISIONS 25: an unmeasured
// lane is never silently accepted).
func TestUnmeasured(t *testing.T) {
	if msg, bad := unmeasured(nil); bad || msg != "" {
		t.Errorf("unmeasured(nil) = %q, %v, want \"\", false", msg, bad)
	}
	skips := []lane.Skip{{What: "stock: golangci-lint", Reason: "x"}, {What: "integrity: git", Reason: "y"}}
	msg, bad := unmeasured(skips)
	if !bad || msg != "stock: golangci-lint, integrity: git" {
		t.Errorf("unmeasured(skips) = %q, %v, want the two lane names joined, true", msg, bad)
	}
}

// fakeSkipLane reports one rule unmeasured and never a finding, so a session built around it
// can never produce a clean census.
type fakeSkipLane struct{}

func (fakeSkipLane) Name() string    { return "fake" }
func (fakeSkipLane) Rules() []string { return []string{fakeRule} }
func (fakeSkipLane) Run(*lane.Context) (lane.Result, error) {
	return lane.Result{Skipped: []lane.Skip{{What: "fake: broken", Reason: "synthetic"}}}, nil
}

const fakeRule = "fn-complexity"

// fakeSession is a real repo.Repo (a throwaway git init) driven by one lane that is always
// unmeasured, so --init/--tighten's refusal is exercised end to end, not just unmeasured().
func fakeSession(t *testing.T) (*session, string) {
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
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fake\n\ngo 1.25\n"), fakeFilePerm); err != nil {
		t.Fatal(err)
	}
	git("add", "go.mod")
	git("commit", "-q", "-m", "init")

	r, err := repo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	limits, err := threshold.Load(besideSource(threshold.FileName))
	if err != nil {
		t.Fatal(err)
	}
	s := &session{o: opts{repo: dir}, all: []lane.Lane{fakeSkipLane{}}}
	s.ctx = &lane.Context{Repo: r, Limits: limits}
	s.ctx.Enabled = s.enabled()
	if !s.ctx.Enabled[fakeRule] {
		t.Fatalf("%s must be enabled by default for this test to exercise the fake lane", fakeRule)
	}
	return s, dir
}

// TestInitBaselineRefusesWhenUnmeasured proves --init never records a run that skipped part of
// the rulebook (point 7: only a fully-measured run may become the baseline).
func TestInitBaselineRefusesWhenUnmeasured(t *testing.T) {
	s, dir := fakeSession(t)
	var out, errw bytes.Buffer
	if code := s.initBaseline(&out, &errw, s.ctx.Repo.Abs(repo.BaselineFile)); code != exitUsage {
		t.Errorf("initBaseline = %d, want exitUsage; stderr: %s", code, errw.String())
	}
	if _, err := os.Stat(filepath.Join(dir, repo.BaselineFile)); err == nil {
		t.Error("initBaseline must not write a baseline while unmeasured")
	}
}

// TestTightenBaselineRefusesWhenUnmeasured mirrors TestInitBaselineRefusesWhenUnmeasured for
// --tighten: it must not lower an existing baseline against an incomplete census either.
func TestTightenBaselineRefusesWhenUnmeasured(t *testing.T) {
	s, _ := fakeSession(t)
	var out, errw bytes.Buffer
	code := s.tightenBaseline(&out, &errw, s.ctx.Repo.Abs(repo.BaselineFile), nil)
	if code != exitUsage {
		t.Errorf("tightenBaseline = %d, want exitUsage; stderr: %s", code, errw.String())
	}
	if !bytes.Contains(errw.Bytes(), []byte("fake: broken")) {
		t.Errorf("stderr must name the unmeasured lane, got: %s", errw.String())
	}
}

// TestAuditCensusKeepsExitZeroOnSkip proves the census (unlike check and baseline) still
// prints and exits 0 on a skip: it is the tool for looking at what is measured, not the gate.
func TestAuditCensusKeepsExitZeroOnSkip(t *testing.T) {
	s, _ := fakeSession(t)
	var out bytes.Buffer
	if code := s.audit(&out); code != exitOK {
		t.Errorf("audit() = %d, want exitOK even with a skip", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("fake: broken")) {
		t.Errorf("the census must still print the skip, got: %s", out.String())
	}
}
