package eval_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"golang.org/x/tools/txtar"
)

// devLayer and nextLayer are the layers the cases activate, in this order.
const (
	devLayer  = "dev"
	nextLayer = "next"
)

// keyedLayered declares cb, with a keyed list by String (oo), by Int (ii), by a ref (rr), by
// a String its key field's default derives (tt) and of elements holding a bound ref (pp), and
// the values a layer line computes a key from.
const keyedLayered = `package a

local record S {
  k: String
  w: Int = 0
}

local record I {
  n: Int
  w: Int = 0
}

local record Item {
  size: Int = 1
}

local let items: table Item = { sword {}, shield {}, bow {} }

local record R {
  it: ref items
  w: Int = 0
}

local record T {
  id: String
  k: String = id
}

local record Tier {
  rate: Int = 1
}

local record P {
  k: String
  pick: ref Tier = low
}

local record B {
  oo: [S] keyed by k
  ii: [I] keyed by n
  rr: [R] keyed by it
  tt: [T] keyed by k
  tiers: table Tier = { low {}, high {} }
  pp: [P] keyed by k = [{ k: "x" }, { k: "y" }]
}

local let kx: String = "x"

local let ky: String = "y"

local let kq: String = "q"

local let kr: String = "r"

local let three: Int = 3

local let bow2: ref items = bow

local let shield2: ref items = shield

local fn mk(key: String) -> S { return { k: key, w: 5 } }

local let cb: B = {
  oo: [{ k: "x" }, { k: "y" }]
  ii: [{ n: 1 }, { n: 2 }]
  rr: [{ it: sword }, { it: shield }]
  tt: [{ id: "y" }]
}

local let readY: Int = cb.oo.y.w
`

// layerLine is the file of package a declaring layer name, amending cb with lines.
func layerLine(name, lines string) txtar.File {
	return txtar.File{Name: "a/" + name + ".canon", Data: []byte("package a\nlayer " + name + "\n\namend cb {\n  " + lines + "\n}\n")}
}

// EVALUATION.md §9.3 (E1913, evaluation form): a keyed-list element keeps its segment's key once every line of the let is applied, replaced whole, its key field amended or re-derived, named by position or its key computed; hard, set by the layer.
func TestAmendKeepsElementKeyWhenApplied(t *testing.T) {
	e1913 := "error[" + string(diag.E1913.Def().Code) + "]"
	for _, tc := range []struct {
		name, lines string
		want        int
		next        string // a second layer's lines, stacked after the first
		says        string // what the finding says, when it matters
	}{
		{"String key computed by a call", "oo.y: mk(kx)", 1, "", ""},
		{"String key computed by a call, the segment's key", "oo.y: mk(ky)", 0, "", ""},
		{"String key read from a let", "oo.y: { k: kx }", 1, "", ""},
		{"String key read from a let, in parentheses: at the key field", "oo.y: ({ k: kx })", 1, "", "a/dev.canon:5:15\n"},
		{"String key read from a let, the segment's key", "oo.y: { k: ky, w: 5 }", 0, "", ""},
		{"String key field in place, computed", "oo.y.k: kx", 1, "", ""},
		{"String key by position", "oo[#1]: { k: \"z\" }", 1, "", ""},
		{"String key by position, its own key", "oo[#1]: { k: \"y\", w: 5 }", 0, "", ""},
		{"Int key computed", "ii[2]: { n: three }", 1, "", ""},
		{"Int key computed, the segment's key", "ii[2]: { n: 1 + 1, w: 5 }", 0, "", ""},
		{"ref key read from a let", "rr[shield]: { it: bow2 }", 1, "", ""},
		{"ref key read from a let, the segment's entry", "rr[shield]: { it: shield2, w: 5 }", 0, "", ""},
		{"ref key field in place, computed", "rr[shield].it: bow2", 1, "", ""},
		{"another field of the element", "oo.y.w: 5", 0, "", ""},
		{"a key re-derived from the field a line amends", "tt.y.id: \"z\"", 1, "", ""},
		{"a key re-derived, then a later line restoring it", "tt.y.id: \"z\"\n  tt.y.k: \"y\"", 0, "", ""},
		{"a key restored, then a later line amending the field it derives from", "tt.y.k: \"y\"\n  tt.y.id: \"z\"", 0, "", ""},
		{"a key changed, then another field of the element", "oo.y.k: kx\n  oo.y.w: 5", 1, "", ""},
		{"a key changed, then a later layer restoring it", "oo.y.k: kx", 0, "oo.y.k: ky", ""},
		{"a key kept, then a later layer changing it", "oo.y.k: ky", 1, "oo.y.k: kx", ""},
		{"two elements holding a bound ref, rekeyed by two lines", "pp.x.k: kq\n  pp.y.k: kr", 1, "", "amend path pp.x.k: the element's key q is not x,"},
		{"an element rekeyed, then another's bound ref amended", "pp.x.k: kq\n  pp.y.pick: low", 1, "", "amend path pp.x.k: the element's key q is not x,"},
	} {
		a := &txtar.Archive{Files: []txtar.File{{Name: "a/a.canon", Data: []byte(keyedLayered)}, layerLine(devLayer, tc.lines)}}
		layers := []string{devLayer}
		if tc.next != "" {
			a.Files = append(a.Files, layerLine(nextLayer, tc.next))
			layers = append(layers, nextLayer)
		}
		out := runBuild(t, fromArchive(t, a), eval.Options{Layers: layers}).findings(t)
		if got := strings.Count(out, e1913); got != tc.want || strings.Count(out, "error[") != got || !strings.Contains(out, tc.says) {
			t.Errorf("%s: %d %s, want %d saying %q and no other error:\n%s", tc.name, got, e1913, tc.want, tc.says, out)
		}
	}
}
