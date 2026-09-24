package eval_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// writtenSuffix names the let holding, written in source, what a layer makes of its twin.
const writtenSuffix = "Written"

// EVALUATION.md §9.3 step 4, §3.4: an amended let `x` equals its twin `xWritten`, written in source.
func TestLayerParity(t *testing.T) {
	golden.Run(t, "testdata/parity/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		opt := eval.Options{Layers: strings.Fields(string(archiveFile(c.Archive, layersFile)))}
		b := runBuild(t, fromArchive(t, c.Archive), opt)
		pairs := 0
		for _, root := range b.order {
			twin := eval.Root{Pkg: root.Pkg, Name: root.Name + writtenSuffix}
			w, written := b.values[twin]
			if !slices.Contains(b.order, twin) {
				continue
			}
			pairs++
			v, amended := b.values[root]
			if amended != written || amended && v.CanonText() != w.CanonText() {
				t.Errorf("%s: amended %t %v, written %t %v", root.Name, amended, v, written, w)
			}
		}
		if pairs == 0 {
			t.Errorf("no let has a %s twin", writtenSuffix)
		}
		return []byte(b.dump() + "\n" + b.findings(t))
	}, golden.Expected(dumpFile))
}
