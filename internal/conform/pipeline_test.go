package conform_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// potion is the pipeline example's Potion with CONFORMANCE.md §6.6's two translated methods.
const potion = `/// Potions.
package pipeline

/// A healing potion.
record Potion {
  /// Its name.
  name: String
  /// Hit points restored.
  heal: Int(1..=100_000)

  /// What a player missing missingHp gets back.
  export fn healFor(self, missingHp: Int) -> Int { return min(heal, max(missingHp, 0)) }

  /// The damage a thrown potion does at level.
  export fn damageAt(self, level: Int(1..=150)) -> Int { return heal * level / 100 }
}

/// The potion of the tests.
let p: Potion = { name: "small", heal: 500 }

/// The same heal, another name.
let q: Potion = { name: "large", heal: 500 }

test "healFor never overheals and never goes negative" {
  expect p.healFor(200) == 200
  expect p.healFor(9000) == 500
  expect q.healFor(-5) == 0
  expect p.damageAt(10) == 50
}

emit cpp { out: "@features/pipeline" }
emit ts { out: "@features/pipeline/potion.ts" }
`

// row is one vector as CONFORMANCE.md §6.6 tabulates it: heal, argument, Go/C++ and TS results.
type row struct {
	heal, arg    int64
	want, tsWant int64
	code, tsCode diag.Code
}

var (
	codeRange = diag.E3204.Def().Code
	codeTS    = diag.E8303.Def().Code
)

// pipeline builds the potion world with its test calls in evaluation order: three of healFor
// (one on q, whose projection equals p's) and one of damageAt.
func pipeline(t *testing.T) (*world, *reference) {
	t.Helper()
	return pipelineOf(t, potion)
}

// pipelineOf is pipeline over another source of the same declarations.
func pipelineOf(t *testing.T, src string) (*world, *reference) {
	t.Helper()
	w := newWorld(t, "pipeline/potion.canon", src)
	p, q := w.value(t, "pipeline", "p"), w.value(t, "pipeline", "q")
	heal, healObj := w.fn(t, "pipeline", "Potion", "healFor")
	damage, damageObj := w.fn(t, "pipeline", "Potion", "damageAt")
	reads := []*ir.Read{{Name: "heal", Path: []string{"heal"}, Type: ir.TypeRef{Kind: types.Int, Bits: types.IntType.Bits, Signed: true}}}
	heal.Reads, damage.Reads = reads, reads
	ref := &reference{w: w, calls: []conform.Call{
		{Fn: healObj, Recv: p, Args: []value.Value{integer(200)}},
		{Fn: healObj, Recv: p, Args: []value.Value{integer(9000)}},
		{Fn: healObj, Recv: q, Args: []value.Value{integer(-5)}},
		{Fn: damageObj, Recv: p, Args: []value.Value{integer(10)}},
	}}
	return w, ref
}

// CONFORMANCE.md §6.6: the worked examples, vector by vector, computed by the evaluator.
func TestWorkedExamples(t *testing.T) {
	w, ref := pipeline(t)
	if err := conform.Fill(context.Background(), w.prog, w.pkgs, ref, w.bags); err != nil {
		t.Fatal(err)
	}
	heal, _ := w.fn(t, "pipeline", "Potion", "healFor")
	compare(t, "healFor", heal.Vectors, []row{
		{500, 200, 200, 200, "", ""},
		{500, 9000, 500, 500, "", ""},
		{500, -5, 0, 0, "", ""},
		{500, intMin, 0, 0, "", codeTS},
		{500, -1, 0, 0, "", ""},
		{500, 0, 0, 0, "", ""},
		{500, 1, 1, 1, "", ""},
		{500, 499, 499, 499, "", ""},
		{500, 500, 500, 500, "", ""},
		{500, 501, 500, 500, "", ""},
		{500, intMax, 500, 0, "", codeTS},
	})
	damage, _ := w.fn(t, "pipeline", "Potion", "damageAt")
	compare(t, "damageAt", damage.Vectors, []row{
		{500, 10, 50, 50, "", ""},
		{500, intMin, 0, 0, codeRange, codeTS},
		{500, -1, 0, 0, codeRange, codeRange},
		{500, 0, 0, 0, codeRange, codeRange},
		{500, 1, 5, 5, "", ""},
		{500, 2, 10, 10, "", ""},
		{500, 149, 745, 745, "", ""},
		{500, 150, 750, 750, "", ""},
		{500, 151, 0, 0, codeRange, codeRange},
		{500, 499, 0, 0, codeRange, codeRange},
		{500, 500, 0, 0, codeRange, codeRange},
		{500, 501, 0, 0, codeRange, codeRange},
		{500, intMax, 0, 0, codeRange, codeTS},
	})
	if out := w.findings(t); strings.Contains(out, "error[") {
		t.Errorf("the worked examples report errors:\n%s", out)
	}
}

// compare compares vectors with the rows of a table, in order.
func compare(t *testing.T, fn string, got []*ir.Vector, want []row) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d vectors, want %d: %s", fn, len(got), len(want), text(got))
	}
	for i, r := range want {
		v := got[i]
		if len(v.Recv) != 1 || intOf(v.Recv[0]) != r.heal || len(v.Args) != 1 || intOf(v.Args[0]) != r.arg {
			t.Errorf("%s vector %d: inputs %s, want heal=%d, %d", fn, i+1, text(got[i:i+1]), r.heal, r.arg)
		}
		if v.Code != r.code || v.TSCode != r.tsCode {
			t.Errorf("%s vector %d: codes %q/%q, want %q/%q", fn, i+1, v.Code, v.TSCode, r.code, r.tsCode)
		}
		if r.code == "" && intOf(v.Want) != r.want || r.tsCode == "" && intOf(v.TSWant) != r.tsWant {
			t.Errorf("%s vector %d: results %s/%s, want %d/%d", fn, i+1, textOf(v.Want), textOf(v.TSWant), r.want, r.tsWant)
		}
	}
}

// intOf is an Int value's integer; a sentinel no row uses for anything else.
func intOf(v value.Value) int64 {
	if n, ok := v.(*value.Int); ok {
		return n.V
	}
	return intMin + 1
}

// text is vectors as `recv | args -> want [code] / tsWant [tsCode]` lines.
func text(vs []*ir.Vector) string {
	var b strings.Builder
	for _, v := range vs {
		b.WriteString("\n  ")
		for _, r := range v.Recv {
			b.WriteString(textOf(r) + " ")
		}
		b.WriteString("|")
		for _, a := range v.Args {
			b.WriteString(" " + textOf(a))
		}
		b.WriteString(" -> " + textOf(v.Want) + " [" + string(v.Code) + "] / " + textOf(v.TSWant) + " [" + string(v.TSCode) + "]")
	}
	return b.String()
}

func textOf(v value.Value) string {
	if v == nil {
		return "-"
	}
	return v.CanonText()
}
