package eval_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// The shape of the layer scale case: entries of the owning record, amendments of its layer.
const (
	scaleEntries    = 10_000
	scaleAmendments = 100
	scaleLayer      = "big"
)

// layerScaleSource is a large owning record and a layer of scaleAmendments amendments into it.
func layerScaleSource() ([]string, [][]byte) {
	src := fmt.Sprintf(`/// A.
package a

/// Item.
record Item {
  /// Key.
  k: String
  /// N.
  n: Int = 0
}

/// Link.
record Link {
  /// To.
  to: ref Item
}

/// Big.
record Big {
  /// Items.
  items: [Item] keyed by k
  /// M.
  m: {String: Int}
  /// Links.
  links: [Link]
}

/// Big.
let big: Big = {
  items: [{ k: "k{i}" } for i in 0..%[1]d]
  m: { "k{i}": i for i in 0..%[1]d }
  links: [{ to: "k%[2]d" }]
}

/// Reads the last amended entry through a ref.
let n: Int = big.links[0].to.n
`, scaleEntries, scaleAmendments-1)
	var layer strings.Builder
	layer.WriteString("package a\nlayer " + scaleLayer + "\n\namend big {\n")
	for i := range scaleAmendments / 2 {
		fmt.Fprintf(&layer, "  m[\"k%d\"]: -1\n", i)
	}
	for i := scaleAmendments / 2; i < scaleAmendments; i++ {
		fmt.Fprintf(&layer, "  items[\"k%d\"].n: %d\n", i, i)
	}
	layer.WriteString("}\n")
	return []string{"a/a.canon", "a/big.canon"}, [][]byte{[]byte(src), []byte(layer.String())}
}

// EVALUATION.md §9.3 step 4, §3.4: amending a large owning instance rebinds its refs, at scale.
func TestLayersScale(t *testing.T) {
	names, data := layerScaleSource()
	b := runBuild(t, parseFiles(t, names, data), eval.Options{Layers: []string{scaleLayer}})
	if v, ok := b.values[eval.Root{Pkg: "a", Name: "n"}]; !ok || v.CanonText() != fmt.Sprint(scaleAmendments-1) {
		t.Errorf("got %v, want %d\n%s", v, scaleAmendments-1, b.findings(t))
	}
}

// BenchmarkLayersScale times the build of the scale case with and without its layer.
func BenchmarkLayersScale(b *testing.B) {
	names, data := layerScaleSource()
	for _, layers := range [][]string{nil, {scaleLayer}} {
		b.Run(strings.Join(append([]string{"layers"}, layers...), "="), func(b *testing.B) {
			for b.Loop() {
				runBuild(b, parseFiles(b, names, data), eval.Options{Layers: layers})
			}
		})
	}
}
