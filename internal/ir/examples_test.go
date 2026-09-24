package ir_test

import (
	"strings"
	"testing"
)

// EVALUATION.md §1 phase 7: stage E over every example reports no emit finding; values the fixture host cannot give (every load) leave their value unset, never a finding.
func TestExamplesHaveNoEmitFinding(t *testing.T) {
	w := newWorld(t)
	w.examples(t, "balance", "features", "game", "pipeline", "resource", "service", "sovcommon", "studio", "teamboard")
	pkgs := w.build(t)
	if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
		t.Errorf("findings:\n%s", out)
	}
	if len(pkgs) == 0 {
		t.Fatal("no package built")
	}
}
