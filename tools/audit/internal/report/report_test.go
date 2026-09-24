package report

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/ratchet"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

// TestCheckFailedOnUnmeasuredLane proves DECISIONS 25 (strictest option): a lane the census
// could not measure fails the gate even when the ratchet itself found nothing new.
func TestCheckFailedOnUnmeasuredLane(t *testing.T) {
	clean := ratchet.Verdict{}
	skips := []lane.Skip{{What: "stock: golangci-lint", Reason: "parallel golangci-lint is running"}}
	if checkFailed(clean, nil) {
		t.Fatal("an empty verdict with nothing skipped must pass")
	}
	if !checkFailed(clean, skips) {
		t.Fatal("an empty verdict with an unmeasured lane must fail")
	}
}

// TestCheckFailedOnRatchetVerdict proves the pre-existing gate (new/grown/enforced findings)
// still fails regardless of skips.
func TestCheckFailedOnRatchetVerdict(t *testing.T) {
	v := ratchet.Verdict{New: []finding.Finding{{Rule: "r"}}}
	if !checkFailed(v, nil) {
		t.Fatal("a verdict with a new finding must fail even with nothing skipped")
	}
}

// TestPrintCheckReportsUnmeasuredLane proves the printed status and the returned bool agree,
// and names the lane and the cause (not just a bare FAIL).
func TestPrintCheckReportsUnmeasuredLane(t *testing.T) {
	var sb strings.Builder
	r := &repo.Repo{Name: "canonlang"}
	skips := []lane.Skip{{What: "stock: golangci-lint", Reason: "parallel golangci-lint is running"}}
	failed, err := PrintCheck(&sb, r, ratchet.Verdict{}, skips)
	if err != nil {
		t.Fatalf("PrintCheck: %v", err)
	}
	if !failed {
		t.Fatal("PrintCheck must report failed=true for an unmeasured lane")
	}
	out := sb.String()
	if !strings.Contains(out, "stock: golangci-lint") || !strings.Contains(out, "parallel golangci-lint is running") {
		t.Errorf("output must name the lane and the cause, got:\n%s", out)
	}
	if !strings.Contains(out, "FAIL") {
		t.Errorf("output must say FAIL, got:\n%s", out)
	}
}

// TestPrintCheckPassesWhenFullyMeasured proves a clean, fully-measured run still prints PASS.
func TestPrintCheckPassesWhenFullyMeasured(t *testing.T) {
	var sb strings.Builder
	r := &repo.Repo{Name: "canonlang"}
	failed, err := PrintCheck(&sb, r, ratchet.Verdict{}, nil)
	if err != nil {
		t.Fatalf("PrintCheck: %v", err)
	}
	if failed {
		t.Fatal("a clean, fully-measured run must not fail")
	}
	if !strings.Contains(sb.String(), "PASS") {
		t.Errorf("output must say PASS, got:\n%s", sb.String())
	}
}
