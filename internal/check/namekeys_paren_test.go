package check_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

// parenBase holds an optional and a required keyed list in a record field, read bare.
const parenBase = `package a

local record S {
  name: String
  w: Int = 0
}

local record A {
  id: String
  oo: ([S] keyed by name)?
  pp: [S] keyed by name = []
}

local record Bag {
  subs: [S] keyed by name
}

local let nb: Bag = { subs: [{ name: "x" }] }

local let aa: [A] keyed by id = [{ id: "second", oo: [{ name: "x" }], pp: [{ name: "x" }] }]

local let bare: S? = aa.second.oo?.x

local let barePP: S = aa.second.pp.x

local let bareNB: S = nb.subs.x
`

// IMPLEMENTATION-PLAN §4.7, DECISIONS 171, 477: parentheses are transparent, so a receiver or a step in them keys `.x` as the bare form does.
func TestNameKeysThroughParens(t *testing.T) {
	for _, tc := range []struct{ name, value, witness string }{
		{"paren then ?.", "S? = (aa.second.oo)?.x", "bare"},
		{"paren then !", "S = (aa.second.oo)!.x", "bare"},
		{"force in the paren", "S = (aa.second.oo!).x", "bare"},
		{"paren around a step", "S? = ((aa.second).oo)?.x", "bare"},
		{"double paren", "S = ((aa.second.pp)).x", "barePP"},
		{"paren around the force of a paren", "S = (((aa.second.oo)!)).x", "bare"},
		{"force mid-path", "S = aa.second!.pp.x", "barePP"},
		{"optional step mid-path", "S? = aa.second?.pp?.x", "barePP"},
		{"optional step, then a bare key", "S? = aa.second?.oo?.x", "bare"},
		{"force on the let", "S = nb!.subs.x", "bareNB"},
		{"optional step on the let", "S? = nb?.subs?.x", "bareNB"},
		{"parens around a mid-path force", "S = (aa.second!).pp.x", "barePP"},
	} {
		src := parenBase + "\nlocal let probe: " + tc.value + "\n"
		prog, files, out := checkSet(t, [][2]string{{"a/a.canon", src}})
		if strings.Contains(out, "error[") {
			t.Errorf("%s: findings:\n%s", tc.name, out)
		}
		for _, g := range gaps(files[0], prog.Info, func(syntax.Decl) bool { return false }) {
			t.Errorf("%s: %s", tc.name, g)
		}
		value, ok := letValue(t, files[0], "probe").(*syntax.SelectorExpr)
		if !ok {
			t.Fatalf("%s: probe's value is no selector", tc.name)
		}
		want := prog.Info.Keys[letValue(t, files[0], tc.witness)]
		if got := prog.Info.Keys[value]; want == nil || got != want {
			t.Errorf("%s: Keys[%s] = %v, want %v (the bare form's)", tc.name, tc.value, got, want)
		}
	}
}
