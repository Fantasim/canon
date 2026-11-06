package canon_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// depDefaultSrc is p: dependent R.v, C.v, T.e and U.m default to a bare case, a case inside an
// `if`, a member and a key; S and S2 are plain controls. The layer amends q*, t*, s3 and u3-u4.
const depDefaultSrc = "package p\n\n" +
	"enum K {\n  a\n  b\n}\n\n" +
	"enum Color {\n  red\n}\n\n" +
	"variant Sh {\n  circle {\n    r: Int = 1\n    o: Int?\n  }\n  square\n}\n\n" +
	"variant Ro {\n  circle {\n    d: Int = 3\n  }\n}\n\n" +
	"record It {\n  n: Int = 0\n}\n\n" +
	"let items: table It = {\n  x {}\n  y {}\n}\n\n" +
	"type Pick(k: K) = match k {\n  a => Sh\n  b => Ro\n}\n\n" +
	"type PE(k: K) = match k {\n  a => Color\n  b => Int\n}\n\n" +
	"type PR(k: K) = match k {\n  a => ref items\n  b => ref items\n}\n\n" +
	"record R {\n  k: K = a\n  v: Pick(k) = circle\n}\n\n" +
	"record C {\n  k: K = a\n  v: Pick(k) = if k == a { circle } else { circle }\n}\n\n" +
	"record S {\n  sh: Sh = circle\n}\n\n" +
	"record S2 {\n  k: K = a\n  sh: Sh = if k == a { square } else { circle }\n}\n\n" +
	"record T {\n  k: K = a\n  e: PE(k) = red\n}\n\n" +
	"record U {\n  k: K = a\n  m: PR(k) = x\n}\n\n" +
	"let r: R = {}\n\n" +
	"let c: C = {}\n\n" +
	"let s: S = {}\n\n" +
	"let e: T = {}\n\n" +
	"let u: U = {}\n\n" +
	"let q: R = {}\n\n" +
	"let q2: S2 = {}\n\n" +
	"let q3: C = {}\n\n" +
	"let t: R = { v: square }\n\n" +
	"let t2: R = { v: square }\n\n" +
	"let s3: S = { sh: square }\n\n" +
	"let u3: U = { m: y }\n\n" +
	"let u4: U = {}\n\n" +
	"let r2: R = { ...r }\n\n" +
	"let c2: C = { ...c }\n\n" +
	"let s2: S = { ...s }\n\n" +
	"let u2: U = { ...u }\n\n" +
	"let cs: table C = {}\n\n" +
	"entry cs.one {}\n"

// depDefaultLayer is dev: it changes q*'s and u4's k, so their defaults re-derive, and writes the
// case bare into t and s3, inside an `if` into t2, and a key into u3.
const depDefaultLayer = "amend q {\n  k: b\n}\n\namend q2 {\n  k: b\n}\n\namend q3 {\n  k: b\n}\n\n" +
	"amend t {\n  v: circle\n}\n\namend t2 {\n  v: if true { circle } else { square }\n}\n\n" +
	"amend s3 {\n  sh: circle\n}\n\namend u3 {\n  m: x\n}\n\namend u4 {\n  k: b\n}\n"

// depDefaultMore is a second file: an edit of n re-evaluates it, replaying p.canon's values, and
// c4 spreads c across the files.
const depDefaultMore = "package p\n\nlet n: Int = 1\n\nlet c4: C = { ...c }\n"

// depDefaultLayerFile is the layer's file as openLayeredWith writes it.
const depDefaultLayerFile = "package p\nlayer dev\n\n" + depDefaultLayer

// srcAt is "line:col" in src of token, in the line that begins with line after its indentation;
// a "|" in token marks the column, the text before it only placing it.
func srcAt(t *testing.T, src, line, token string) string {
	t.Helper()
	before, at, marked := strings.Cut(token, "|")
	if !marked {
		before, at = "", token
	}
	for i, l := range strings.Split(src, "\n") {
		if !strings.HasPrefix(strings.TrimLeft(l, " "), line) {
			continue
		}
		if c := strings.Index(l, before+at); c >= 0 {
			return fmt.Sprintf("%d:%d", i+1, c+len(before)+1)
		}
	}
	t.Fatalf("no line %q holding %q", line, token)
	return ""
}

// hop is one origin as originText prints it: kind, file, "line:col" and layer.
type hop struct {
	kind, file, at, layer string
}

// hopsText is hops as originText prints an origin and its Via chain, replaced on the first.
func hopsText(replaced bool, hops ...hop) string {
	out := make([]string, len(hops))
	for i, h := range hops {
		out[i] = fmt.Sprintf("%s %s:%s layer %q replaced %v", h.kind, h.file, h.at, h.layer, replaced && i == 0)
	}
	return strings.Join(out, "\n")
}

// depDefaultRow is one value's origin, with the layer active or not.
type depDefaultRow struct {
	rule, path string
	on         bool
	want       string
}

// depDefaultRows are the origins TestDependentDefaultOrigins expects, found in the files.
func depDefaultRows(t *testing.T) []depDefaultRow {
	src, lay := "p/p.canon", "p/dev.layer.canon"
	in := func(line, token string) hop { return hop{"literal", src, srcAt(t, depDefaultSrc, line, token), ""} }
	def := func(line, token string) hop { return hop{"default", src, srcAt(t, depDefaultSrc, line, token), ""} }
	lit := func(line, token string) hop {
		return hop{"literal", lay, srcAt(t, depDefaultLayerFile, line, token), "dev"}
	}
	dev := func(h hop) hop { h.layer = "dev"; return h }
	as := func(kind string, h hop) hop { h.kind = kind; return h }
	vDef, cDef, cIn, cElse := def("v: Pick(k) = circle", "circle"), def("v: Pick(k) = if", "if"), in("v: Pick(k) = if", "{ |circle"), in("v: Pick(k) = if", "else { |circle")
	shDef, rDef, dDef, oDef := def("sh: Sh = circle", "circle"), def("r: Int", "1"), def("d: Int", "3"), def("o: Int?", "o")
	letOf := func(name string) hop { return in("let "+name+":", "{") }
	return []depDefaultRow{
		{"EVALUATION.md §13", "p:r.v", false, hopsText(false, vDef, letOf("r"))},
		{"EVALUATION.md §13", "p:r.v.r", false, hopsText(false, rDef, as("literal", vDef))},
		{"EVALUATION.md §13", "p:r.v.o", false, hopsText(false, oDef, as("literal", vDef))},
		{"EVALUATION.md §13 control", "p:s.sh", false, hopsText(false, shDef, letOf("s"))},
		{"EVALUATION.md §13 control", "p:s.sh.r", false, hopsText(false, rDef, as("literal", shDef))},
		{"EVALUATION.md §13 control", "p:s.sh.o", false, hopsText(false, oDef, as("literal", shDef))},
		{"EVALUATION.md §13 compound", "p:c.v", false, hopsText(false, cDef, letOf("c"))},
		{"EVALUATION.md §13 compound", "p:c.v.r", false, hopsText(false, rDef, cIn)},
		{"EVALUATION.md §13 member", "p:e.e", false, hopsText(false, def("e: PE(k)", "red"), letOf("e"))},
		{"EVALUATION.md §13 ref", "p:u.m", false, hopsText(false, def("m: PR(k)", "x"), letOf("u"))},
		{"EVALUATION.md §13", "p:q.v", false, hopsText(false, vDef, letOf("q"))},
		{"DECISIONS 464, 501", "p:q.v", true, hopsText(false, dev(vDef), letOf("q"))},
		{"DECISIONS 464, 501", "p:q.v.d", true, hopsText(false, dev(dDef), dev(as("literal", vDef)))},
		{"DECISIONS 464, 501 control", "p:q2.sh", true, hopsText(false, dev(def("sh: Sh = if", "if")), letOf("q2"))},
		{"DECISIONS 464, 501 control", "p:q2.sh.r", true, hopsText(false, dev(rDef), dev(in("sh: Sh = if", "|circle")))},
		{"DECISIONS 464, 501 compound", "p:q3.v.d", true, hopsText(false, dev(dDef), dev(cElse))},
		{"DECISIONS 464, 501 ref", "p:u4.m", true, hopsText(false, dev(def("m: PR(k)", "x")), letOf("u4"))},
		{"EVALUATION.md §13 layer", "p:t.v", true, hopsText(true, as("layer", lit("v: circle", "circle")))},
		{"EVALUATION.md §13 layer", "p:t.v.r", true, hopsText(false, dev(rDef), lit("v: circle", "circle"))},
		{"EVALUATION.md §13 layer compound", "p:t2.v", true, hopsText(true, as("layer", lit("v: if", "if")))},
		{"EVALUATION.md §13 layer compound", "p:t2.v.r", true, hopsText(false, dev(rDef), lit("v: if", "|circle"))},
		{"EVALUATION.md §13 layer control", "p:s3.sh", true, hopsText(true, as("layer", lit("sh: circle", "circle")))},
		{"EVALUATION.md §13 layer control", "p:s3.sh.r", true, hopsText(false, dev(rDef), lit("sh: circle", "circle"))},
		{"EVALUATION.md §13 layer ref", "p:u3.m", true, hopsText(true, as("layer", lit("m: x", "x")))},
		{"EVALUATION.md §13 spread", "p:r2.v", false, hopsText(false, as("spread", in("let r2:", "...")), vDef, letOf("r"))},
		{"EVALUATION.md §13 spread", "p:r2.v.r", false, hopsText(false, rDef, as("literal", vDef))},
		{"EVALUATION.md §13 spread compound", "p:c2.v.r", false, hopsText(false, rDef, cIn)},
		{"EVALUATION.md §13 spread compound", "p:c4.v.r", false, hopsText(false, rDef, cIn)},
		{"EVALUATION.md §13 spread control", "p:s2.sh", false, hopsText(false, as("spread", in("let s2:", "...")), shDef, letOf("s"))},
		{"EVALUATION.md §13 spread control", "p:s2.sh.r", false, hopsText(false, rDef, as("literal", shDef))},
		{"EVALUATION.md §13 compound entry", "p:cs[one].v.r", false, hopsText(false, rDef, cIn)},
		{"EVALUATION.md §13 compound entry", "p:cs[one].v", false, hopsText(false, cDef)},
		{"EVALUATION.md §13 spread ref", "p:u2.m", false, hopsText(false, as("spread", in("let u2:", "...")), def("m: PR(k)", "x"), letOf("u"))},
	}
}

// openDepDefault opens depDefaultSrc beside depDefaultMore under the layer, active when on; warm,
// after two edits of n, so p.canon's values are replayed (API.md W7).
func openDepDefault(t *testing.T, on, warm bool) *canon.Project {
	t.Helper()
	p := openLayeredWith(t, depDefaultSrc, map[string]string{"p/b.canon": depDefaultMore}, depDefaultLayer, on)
	for step := 2; warm && step <= 3; step++ {
		if _, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("p:n", canon.Int(int64(step)))}}); err != nil {
			t.Fatalf("API.md E19: %v", err)
		}
	}
	return p
}

// depDefaultHeadOnly are the rows whose via, a table entry's span, the queued EVAL-ENTRYSPAN fixes (EVALUATION.md §13).
var depDefaultHeadOnly = map[string]bool{"p:cs[one].v": true}

// EVALUATION.md §13, DECISIONS 464, 501: a defaulted dependent field has the origin a plain one has, warm as cold.
func TestDependentDefaultOrigins(t *testing.T) {
	rows := depDefaultRows(t)
	for _, warm := range []bool{false, true} {
		projects := map[bool]*canon.Project{}
		for _, c := range rows {
			p := projects[c.on]
			if p == nil {
				p = openDepDefault(t, c.on, warm)
				projects[c.on] = p
			}
			o := mustValue(t, p, c.path).Origin
			if depDefaultHeadOnly[c.path] {
				o.Via = nil
			}
			if got := originText(&o); got != c.want {
				t.Errorf("%s: %s (layer %v, warm %v) origin\n%s\nwant\n%s", c.rule, c.path, c.on, warm, got, c.want)
			}
		}
	}
}
