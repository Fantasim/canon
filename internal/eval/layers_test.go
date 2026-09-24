package eval_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const layersFile = "layers"

// EVALUATION.md §9.1–§9.3 (LAY-01, LAY-02), §3.4: the archive's first line names its rules.
func TestLayers(t *testing.T) {
	golden.Run(t, "testdata/layers/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		opt := eval.Options{Layers: strings.Fields(string(archiveFile(c.Archive, layersFile)))}
		b := runBuild(t, fromArchive(t, c.Archive), opt)
		var sb strings.Builder
		for _, s := range b.ev.StableAmendments() {
			fmt.Fprintf(&sb, "stable amendment: layer %s table %q field %q\n", s.Layer, s.Table, s.Field)
		}
		return []byte(b.dump() + sb.String() + "\n" + b.findings(t))
	}, golden.Expected(dumpFile))
}
