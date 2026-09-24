package build

import "testing"

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
