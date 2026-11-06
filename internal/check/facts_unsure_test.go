package check

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// TYPES.md §1, §6.6; DECISIONS 516: what unsure facts and patterns cover, and how they combine.
func TestUnsureFacts(t *testing.T) {
	const v, w, vf, vg, wf, vx, vxn = 7, 8, "7.f", "7.g", "8.f", "7.x", "7.x.n"
	anyPat := &pattern{root: anyRoot, tail: "f", below: w}
	pf := facts{anyPat.key(): {root: anyRoot, unsure: true, pat: anyPat}}
	segPat := &pattern{root: v, prefix: "7"}
	sf := facts{segPat.key(): {root: v, unsure: true, pat: segPat}}
	sure := facts{vf: {root: v}}
	kase := facts{vf: {root: v, kase: &types.CaseType{}}}
	unsureOf := func(cond int, side bool) facts { return facts{vf: {root: v, unsure: true}}.tagged(cond, side) }
	for _, tc := range []struct {
		name string
		got  facts
		want string // per probe 7.f 8.f 7.g 7.x 7.x.n: + sure, ? unsure, - none; then "case" if 7.f has one
	}{
		{"a root in error covers its tail on roots declared before", pf.tagged(1, true), "? - - - -"},
		{"a kill uncovers its root", pf.tagged(1, true).kill(v), "- - - - -"},
		{"a segment in error covers one segment after the prefix", sf, "? - ? ? -"},
		{"a kill of the prefix's root drops the pattern", sf.kill(v), "- - - - -"},
		{"sure ∩ covered is unsure", sure.intersect(pf.tagged(1, true)), "? - - - -"},
		{"sure ∪ unsure is sure", sure.union(unsureOf(1, true)), "+ - - - -"},
		{"unsure ∪ sure is sure", unsureOf(1, true).union(sure), "+ - - - -"},
		{"the two sides of one condition never meet", unsureOf(1, true).intersect(unsureOf(1, false)), "- - - - -"},
		{"a path and a pattern of opposite sides never meet", unsureOf(1, true).intersect(pf.tagged(1, false)), "- - - - -"},
		{"two conditions' unsure facts meet", unsureOf(1, true).intersect(unsureOf(2, false)), "? - - - -"},
		{"a pattern meets itself", pf.tagged(1, true).intersect(pf.tagged(1, true)), "? - - - -"},
		{"a pattern never meets its other side", pf.tagged(1, true).intersect(pf.tagged(1, false)), "- - - - -"},
		{"a kill removes an unsure fact", unsureOf(1, true).kill(v), "- - - - -"},
		{"an unsure case is no case", kase.intersect(unsureOf(1, true)), "? - - - -"},
		{"a sure case stays", kase.intersect(kase), "+ - - - - case"},
	} {
		if got := unsureShape(tc.got, []probe{{vf, v}, {wf, w}, {vg, v}, {vx, v}, {vxn, v}}); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

// probe is a path TestUnsureFacts looks up: its key and root.
type probe struct {
	key  string
	root int
}

// unsureShape describes how a holds each probe for TestUnsureFacts, then whether the first has a case.
func unsureShape(a facts, probes []probe) string {
	var parts []string
	for _, p := range probes {
		f, ok := a.lookup(p.key, p.root)
		switch {
		case !ok:
			parts = append(parts, "-")
		case f.unsure:
			parts = append(parts, "?")
		default:
			parts = append(parts, "+")
		}
	}
	if f, ok := a[probes[0].key]; ok && f.kase != nil {
		parts = append(parts, "case")
	}
	return strings.Join(parts, " ")
}
