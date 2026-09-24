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

// nestedScaleSource is layerScaleSource's owner inside a record its entries name, amended.
func nestedScaleSource() ([]string, [][]byte) {
	src := fmt.Sprintf(`/// A.
package a

/// Tier.
record Tier {
  /// W.
  w: Int = 1
}

/// Item.
record Item {
  /// Key.
  k: String
  /// N.
  n: Int = 0
  /// Tier (Config's).
  tier: ref Tier = low
}

/// Link.
record Link {
  /// To (Big's).
  to: ref Item
}

/// Big.
record Big {
  /// Items.
  items: [Item] keyed by k
  /// Links.
  links: [Link]
}

/// Config.
record Config {
  /// Tiers.
  tiers: table Tier = { low {} }
  /// Big.
  big: Big
}

/// Config.
let config: Config = { big: {
  items: [{ k: "k{i}" } for i in 0..%[1]d]
  links: [{ to: "k%[2]d" }]
} }

/// Reads the last amended entry through Big's ref, then Config's.
let n: Int = config.big.links[0].to.n + config.big.links[0].to.tier.w - 1
`, scaleEntries, scaleAmendments-1)
	var layer strings.Builder
	layer.WriteString("package a\nlayer " + scaleLayer + "\n\namend config {\n")
	for i := range scaleAmendments {
		fmt.Fprintf(&layer, "  big.items[\"k%d\"].n: %d\n", i, i)
	}
	layer.WriteString("}\n")
	return []string{"a/a.canon", "a/big.canon"}, [][]byte{[]byte(src), []byte(layer.String())}
}

// scaleSources are the scale cases, by name.
var scaleSources = []struct {
	name   string
	source func() ([]string, [][]byte)
}{{"owner", layerScaleSource}, {"nested", nestedScaleSource}}

// EVALUATION.md §9.3 step 4, §3.4, §4.2: amending a large owning instance, nested or not, rebinds its refs.
func TestLayersScale(t *testing.T) {
	for _, c := range scaleSources {
		names, data := c.source()
		for _, layers := range [][]string{nil, {scaleLayer}} {
			want := 0
			if len(layers) > 0 {
				want = scaleAmendments - 1
			}
			b := runBuild(t, parseFiles(t, names, data), eval.Options{Layers: layers})
			if v, ok := b.values[eval.Root{Pkg: "a", Name: "n"}]; !ok || v.CanonText() != fmt.Sprint(want) {
				t.Errorf("%s, layers %v: got %v, want %d\n%s", c.name, layers, v, want, b.findings(t))
			}
		}
	}
}

// BenchmarkLayersScale times the build of each scale case with and without its layer.
func BenchmarkLayersScale(b *testing.B) {
	for _, c := range scaleSources {
		names, data := c.source()
		for _, layers := range [][]string{nil, {scaleLayer}} {
			b.Run(strings.Join(append([]string{c.name, "layers"}, layers...), "="), func(b *testing.B) {
				for b.Loop() {
					runBuild(b, parseFiles(b, names, data), eval.Options{Layers: layers})
				}
			})
		}
	}
}
