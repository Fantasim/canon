package check_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	layerFile = "a/dev.layer.canon"
	layerBase = `package a

local enum Grade { low, high }

local record Tier {
  rate: Int
}

local record Cfg {
  tiers: table Tier
  keyed: [Tier] keyed by rate
  grades: {Grade: Int}
  names: {String: Int}
  xs: [Int]
}

local let k: String = "k"

local let cfg: Cfg = {
  tiers: { low { rate: 1 }, high { rate: 2 } }
  keyed: [{ rate: 3 }]
  grades: { low: 1 }
  names: { "k": 1 }
  xs: [1, 2]
}
`
)

// checkLayer checks package a with a layer dev amending cfg, parse findings in the package's bag.
func checkLayer(t *testing.T, amends string) string {
	t.Helper()
	set := &source.FileSet{}
	bags := check.Bags{builtPackage: diag.NewBag(set, builtPackage)}
	var files []*syntax.File
	for _, f := range [][2]string{{builtFile, layerBase}, {layerFile, "package a\nlayer dev\n\namend cfg {\n" + amends + "}\n"}} {
		src, err := set.Add(f[0], "/"+f[0], []byte(f[1]))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, syntax.Parse(src, syntax.FileSource, bags[builtPackage]))
	}
	check.Check(context.Background(), exampleProject(), files, bags, literalFolder{})
	return render(t, set, diag.NewBag(set, ""), bags)
}

// EVALUATION.md §9.2 E1908 over API.md §6.5 P1, P2, P8, P9: paths compare by canonical segment.
func TestAmendPathsCompareCanonically(t *testing.T) {
	e1908 := "error[" + string(diag.E1908.Def().Code) + "]"
	for _, tc := range []struct {
		name, amends string
		want         int
	}{
		{"P9 table key as a field, a word and a string", "  tiers.low.rate: 2\n  tiers[low].rate: 3\n  tiers[\"low\"].rate: 4\n", 2},
		{"P8 an entry is a prefix of its field", "  tiers.low.rate: 2\n  tiers[\"low\"]: { rate: 4 }\n", 1},
		{"P8 two entries", "  tiers.low.rate: 2\n  tiers.high.rate: 3\n", 0},
		{"P8 position against key, table", "  tiers[#0].rate: 2\n  tiers.low.rate: 3\n", 0},
		{"P8 position twice", "  tiers[#0].rate: 2\n  tiers[#0].rate: 3\n", 1},
		{"P1 keyed list: an integer key is not a position", "  keyed[3].rate: 2\n  keyed[#3].rate: 3\n", 0},
		{"P1 keyed list: one key", "  keyed[3]: { rate: 3 }\n  keyed[3].rate: 3\n", 1},
		{"P2 enum key bare and qualified", "  grades[low]: 2\n  grades[Grade.low]: 3\n", 1},
		{"P8 plain list index and position", "  xs[1]: 5\n  xs[#1]: 6\n", 1},
		{"P8 negative index", "  xs[-1]: 5\n  xs[1]: 6\n", 0},
		{"a computed key against a literal", "  names[k]: 2\n  names[\"k\"]: 3\n", 0},
		{"a computed key twice", "  names[k]: 2\n  names[k]: 3\n", 1},
	} {
		out := checkLayer(t, tc.amends)
		if got := strings.Count(out, e1908); got != tc.want || strings.Count(out, "error[") != got {
			t.Errorf("%s: %d %s, want %d and nothing else:\n%s", tc.name, got, e1908, tc.want, out)
		}
	}
}
