package eval_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const layersFile = "layers"

// EVALUATION.md §9.3: layers amend in order, derived defaults follow, E1905, E4301, stable adds.
func TestLayers(t *testing.T) {
	golden.Run(t, "testdata/layers/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		var opt eval.Options
		for _, f := range c.Archive.Files {
			if f.Name == layersFile {
				opt.Layers = strings.Fields(string(f.Data))
			}
		}
		b := runBuild(t, fromArchive(t, c.Archive), opt)
		var sb strings.Builder
		for _, s := range b.ev.StableAmendments() {
			fmt.Fprintf(&sb, "stable amendment: layer %s table %q field %q\n", s.Layer, s.Table, s.Field)
		}
		return []byte(b.dump() + sb.String() + "\n" + b.findings(t))
	}, golden.Expected(dumpFile))
}
