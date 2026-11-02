package eval_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// maxQuietBudget bounds quietBudget's search.
const maxQuietBudget = 512

// keptBinderDecls stores, at a dependent map spelled with binder B, the union of a kept map with a literal typed against the kept map's own spelling, x.
const keptBinderDecls = `local record Node {
  k: E
}

local let nodes: table Node = { p { k: a }, q { k: c } }

local type P(e: Node) = match e.k {
  a => Int
  b => Int
  c => String
}

local record H {
  by: {x in nodes: P(x)} = {}
}

local let h: H = { by: { p: 1 } }
local let x: {B in nodes: P(B)} = h.by.union({ q: "s" })`

// EVALUATION.md §5, DECISIONS 446 (amends 408): two dependent-map types that differ only in a binder's name are one type for the residue walk, so the spelling changes no budget.
func TestKeptLiteralBinderSpelling(t *testing.T) {
	same := quietBudget(t, strings.ReplaceAll(keptBinderDecls, "B", "x"))
	other := quietBudget(t, strings.ReplaceAll(keptBinderDecls, "B", "y"))
	if same != other {
		t.Errorf("stored at {y in nodes: P(y)} needs %d steps, at {x in nodes: P(x)} %d: want the same", other, same)
	}
}

// storedRecordDecls stores every record literal with a dependent field it builds: in a let, a
// list, a table, an entry, a field of another record and a field's default, which is copied.
const storedRecordDecls = `local record Node {
  k: E
}

local let nodes: table Node = { p { k: a }, q { k: c } }

local type P(e: Node) = match e.k {
  a => Int
  b => Int
  c => String
}

local record W {
  n: ref nodes
  v: P(n)
}

local record Box {
  w: W = { n: q, v: "d" }
  ws: [W] = []
}

local let x: W = { n: p, v: 1 }
local let ws: [W] = [W { n: p, v: 2 }, { n: q, v: "s" }]
local let t: table W = { one { n: p, v: 3 } }
local let b: Box = { ws: [{ n: q, v: "t" }] }
local let es: table W = {}

entry es.two { n: q, v: "u" }`

// storedRecordSteps is the budget storedRecordDecls needed before DECISIONS 446, measured: a
// program storing all its records keeps its budget.
const storedRecordSteps = 60

// EVALUATION.md §5, DECISIONS 446: a stored record is judged by its root's walk alone, so a program storing all its records keeps its budget (a pin).
func TestStoredRecordsKeepTheirBudget(t *testing.T) {
	if got := quietBudget(t, storedRecordDecls); got != storedRecordSteps {
		t.Errorf("storing every record needs %d steps, want %d as before DECISIONS 446", got, storedRecordSteps)
	}
}

// The budgets TestKeptLiteralsInOrder sweeps: from the first that reaches the literals to the first that judges them all.
const (
	sweepFrom = 35
	sweepTo   = 54
)

// EVALUATION.md §5, DECISIONS 408, 446: independent literals are judged in recording order, so where a budget runs out among them is as before (sweep.txt, written by the code before DECISIONS 446).
func TestKeptLiteralsInOrder(t *testing.T) {
	golden.Run(t, "testdata/dependent/order_*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		var out strings.Builder
		prev := ""
		for budget := int64(sweepFrom); budget <= sweepTo; budget++ {
			if f := runBuild(t, fromArchive(t, c.Archive), eval.Options{Budget: budget}).findings(t); f != prev {
				fmt.Fprintf(&out, "=== budget %d\n%s\n", budget, f)
				prev = f
			}
		}
		return []byte(out.String())
	}, golden.Expected(sweepFile))
}

// sweepFile is the archive file holding a budget sweep's findings.
const sweepFile = "sweep.txt"

// quietBudget is the smallest budget under which decl's whole build, stage B's residue walk included, reports no E4401.
func quietBudget(t *testing.T, decl string) int64 {
	t.Helper()
	src := []byte(enumHead + decl + "\n")
	for budget := int64(1); budget < maxQuietBudget; budget++ {
		b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{src}), eval.Options{Budget: budget})
		if len(codes(b.bags["a"], diag.E4401.Def().Code)) == 0 {
			return budget
		}
	}
	t.Fatalf("%s: no budget under %d builds it", decl, maxQuietBudget)
	return 0
}
