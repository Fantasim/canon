package eval_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// entrySrc is the translated fn of DECISIONS 311's report, two refined parameters, and a
// method reading a field past TypeScript's safe range.
const entrySrc = `/// Entry.
package entry

/// a plus b.
export fn f(a: Int(0..), b: Int(0..=5)) -> Int { return a + b }

/// A gauge.
record G {
  /// 2^60.
  big: Int

  /// big scaled, plus a.
  export fn scaled(self, a: Int(0..=5)) -> Int { return big / 1_048_576 + a }
}

/// A gauge.
let g: G = { big: 1_152_921_504_606_846_976 }
`

// CONFORMANCE.md §2.3, §4, DECISIONS 311: each parameter passes representability, then its range, before the next.
func TestVectorEntryPerParameter(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "entry/entry.canon", entrySrc)
	f := fnObj(t, b, "entry", "", "f")
	for _, c := range []gaugeCase{
		{"a's range before b's representability", "f", ints(-1, intMin), 0, 0, codeRange, codeRange},
		{"a unsafe first", "f", ints(pow60, intMin), 0, 0, codeRange, codeTS},
		{"b unsafe after a valid a", "f", ints(1, pow60), 0, 0, codeRange, codeTS},
		{"b's range", "f", ints(1, 6), 0, 0, codeRange, codeRange},
		{"both valid", "f", ints(1, 2), 3, 3, "", ""},
	} {
		call := eval.Call{Fn: f, Args: c.args}
		t.Run(c.name, func(t *testing.T) {
			expectOutcome(t, b.ev.Vector(context.Background(), call, eval.VectorMode{Steps: vectorCap}), c.want, c.code)
			expectOutcome(t, b.ev.Vector(context.Background(), call, eval.VectorMode{Steps: vectorCap, TS: true}), c.tsWant, c.tsCode)
		})
	}
	assertEmpty(t, b.bags)
}

// CONFORMANCE.md §2.3, DECISIONS 311: the reads of self are the first parameters, checked before the declared ones.
func TestVectorEntrySelfFirst(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "entry/entry.canon", entrySrc)
	call := eval.Call{Fn: fnObj(t, b, "entry", "G", "scaled"), Recv: b.values[eval.Root{Pkg: "entry", Name: "g"}], Args: ints(-1)}
	expectOutcome(t, b.ev.Vector(context.Background(), call, eval.VectorMode{Steps: vectorCap}), 0, codeRange)
	expectOutcome(t, b.ev.Vector(context.Background(), call, eval.VectorMode{Steps: vectorCap, TS: true}), 0, codeTS)
	assertEmpty(t, b.bags)
}
