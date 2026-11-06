package check_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// keyedBase declares cfg with a keyed list by String (oo), by Int (ii), by a ref (rr) and by an
// enum (gg), and a table and an enum of the same shapes as their keys'.
const keyedBase = `package a

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

local let others: table Item = { axe {}, bow {} }

local record R {
  it: ref items
  w: Int = 0
}

local enum Grade { low, mid }

local enum Other { low, mid }

local record G {
  g: Grade
  w: Int = 0
}

local record Cfg {
  oo: [S] keyed by k
  ii: [I] keyed by n
  rr: [R] keyed by it
  gg: [G] keyed by g
}

local let kx: String = "x"

local let cfg: Cfg = {
  oo: [{ k: "x" }, { k: "y" }]
  ii: [{ n: 1 }, { n: 2 }]
  rr: [{ it: sword }, { it: shield }]
  gg: [{ g: low }]
}
`

// EVALUATION.md §9.3 (E1913, static form): a literal key field, of the element whole or alone, names its segment's key; a computed key, a position or a key of another type waits for evaluation.
func TestAmendKeepsElementKey(t *testing.T) {
	e1913 := errorOf(diag.E1913.Def().Code)
	for _, tc := range []struct {
		name, amends string
		want         int
		also         diag.Code // the one other error the line has, its own finding
	}{
		{"String key, another key", "  oo.y: { k: \"z\", w: 5 }\n", 1, ""},
		{"String key, a key in use", "  oo.y: { k: \"x\" }\n", 1, ""},
		{"String key, a string segment", "  oo[\"y\"]: { k: \"z\" }\n", 1, ""},
		{"String key, the segment's key", "  oo.y: { k: \"y\", w: 5 }\n", 0, ""},
		{"String key field in place", "  oo.y.k: \"z\"\n", 1, ""},
		{"String key field in place, the same key", "  oo.y.k: \"y\"\n", 0, ""},
		{"Int key, another key", "  ii[2]: { n: 3 }\n", 1, ""},
		{"Int key, the segment's key", "  ii[2]: { n: 2, w: 5 }\n", 0, ""},
		{"Int key field in place", "  ii[2].n: 1\n", 1, ""},
		{"ref key, another entry", "  rr[shield]: { it: bow }\n", 1, ""},
		{"ref key, a word segment and a string", "  rr.shield: { it: \"bow\" }\n", 1, ""},
		{"ref key, the segment's entry", "  rr[shield]: { it: shield, w: 5 }\n", 0, ""},
		{"ref key field in place", "  rr[shield].it: sword\n", 1, ""},
		{"enum key, another member", "  gg[low]: { g: mid }\n", 1, ""},
		{"enum key, a member of another enum", "  gg[low]: { g: Other.mid }\n", 0, diag.E3002.Def().Code},
		{"ref key, an entry of another table", "  rr[shield]: { it: others.axe }\n", 0, ""},
		{"ref key, a same-named entry of another table", "  rr[shield]: { it: others.bow }\n", 0, ""},
		{"a computed key is left to evaluation", "  oo.y: { k: kx }\n", 0, ""},
		{"a position is left to evaluation", "  oo[#1]: { k: \"z\" }\n", 0, ""},
		{"another field of the element", "  oo.y.w: 5\n", 0, ""},
	} {
		out := checkLayerOn(t, keyedBase, tc.amends)
		got, others := strings.Count(out, e1913), 0
		if tc.also != "" {
			others = strings.Count(out, errorOf(tc.also))
		}
		if got != tc.want || others != min(1, len(tc.also)) || strings.Count(out, errorOf("")) != got+others {
			t.Errorf("%s: %d %s, want %d and no other error than %q:\n%s", tc.name, got, e1913, tc.want, tc.also, out)
		}
	}
}

// errorOf is the header of an error finding with code c, of any error for "".
func errorOf(c diag.Code) string {
	if c == "" {
		return "error["
	}
	return "error[" + string(c) + "]"
}
