package build

import (
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
		!strings.HasSuffix(base, tempSuffix) || len(base) > len(".x.canon.canon-edit")+len(tempPrefix+tempSep+tempSuffix)+2*tempRandBytes {
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
