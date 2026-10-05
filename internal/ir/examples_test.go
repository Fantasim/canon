package ir_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// legacyExample is the one example whose emit stage E refuses: its legacy C++ struct is E8019 `LegacyStruct` until M6 (DECISIONS 320), and its expected/findings.txt records it.
var legacyExample = "error[" + string(diag.E8019.Def().Code) + "]  features/legacycpp/"

// EVALUATION.md §1 phase 7: stage E over every example reports no emit finding but legacycpp's legacy struct (DECISIONS 320); values the fixture host cannot give (every load) leave their value unset, never a finding.
func TestExamplesHaveNoEmitFinding(t *testing.T) {
	w := newWorld(t)
	w.examples(t, "balance", "features", "game", "pipeline", "resource", "service", "sovcommon", "studio", "teamboard")
	pkgs := w.build(t)
	out := w.findings(t)
	legacy := 0
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, legacyExample):
			legacy++
		case strings.HasPrefix(line, "error[") || strings.HasPrefix(line, "warning["):
			t.Errorf("finding outside %s:\n%s", legacyExample, out)
		}
	}
	if legacy != 1 {
		t.Errorf("want legacycpp's one LegacyStruct finding, got %d:\n%s", legacy, out)
	}
	if len(pkgs) == 0 {
		t.Fatal("no package built")
	}
}
