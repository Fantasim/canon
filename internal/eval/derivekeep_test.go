package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// keepSrc is p, a record held at one place, whose costly default big reads n, whose keyed list oo
// reads nothing and whose tag reads id.
const keepSrc = `package a

local record S {
  k: String
  w: Int = 0
}

local record P {
  id: String
  n: Int = 1
  big: Int = [i for i in 0..40].filter(i => i >= n).len()
  oo: [S] keyed by k = [{ k: "x" }, { k: "y" }]
  tag: String = id
}

local let p: P = { id: "a" }
`

// keepCase is a line amending p, the value it leaves and whether it re-evaluates big.
type keepCase struct {
	line, want string
	costly     bool
}

// EVALUATION.md §9.3 step 3, DECISIONS 501: a line re-evaluates only the later defaults whose inputs it changed.
func TestDeriveKeepsUnchangedDefaults(t *testing.T) {
	const costly = 40 // big's steps at least
	steps := func(line string, layers []string) (int64, string) {
		layer := "package a\nlayer " + devLayer + "\n\namend p {\n  " + line + "\n}\n"
		b := buildFiles(t, eval.Options{Layers: layers}, "a/a.canon", keepSrc, "a/"+devLayer+".canon", layer)
		assertBuilt(t, b)
		return b.ev.StepsSpent(), b.values[eval.Root{Pkg: "a", Name: "p"}].CanonText()
	}
	off, _ := steps(`id: "z"`, nil)
	for _, c := range []keepCase{
		{`id: "z"`, `P{id: "z", n: 1, big: 39, oo: [S{k: "x", w: 0}, S{k: "y", w: 0}], tag: "z"}`, false},
		{`n: 5`, `P{id: "a", n: 5, big: 35, oo: [S{k: "x", w: 0}, S{k: "y", w: 0}], tag: "a"}`, true},
	} {
		on, got := steps(c.line, []string{devLayer})
		if got != c.want {
			t.Errorf("EVALUATION.md §9.3: %s gives %s, want %s", c.line, got, c.want)
		}
		if spent := on - off; spent >= costly != c.costly {
			t.Errorf("DECISIONS 501: %s costs %d steps more than no layer, want big re-evaluated %v", c.line, spent, c.costly)
		}
	}
}
