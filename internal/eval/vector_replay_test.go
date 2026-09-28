package eval_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// replaySrc has an instance the build verifies, which a let only a vector forces reaches again.
const replaySrc = `/// Flows.
package flow

/// K.
enum K { col, num }

/// Color.
enum Color { red, blue }

/// P.
type P(k: K) = match k {
  col => Color
  num => Int
}

/// Thing.
record Thing {
  /// K.
  k: K
  /// Payload.
  p: P(k)
}

/// Things.
let things: [Thing] = [{ k: num, p: red }]

/// The shared instance, forced first by a vector.
let alias: Thing = things[0]

/// A gate.
record Gate {
  /// Its weight.
  weight: Int

  /// The weight plus x, for the shared instance's kind.
  export fn plus(self, x: Int) -> Int {
    if alias.k == num {
      return weight + x
    }
    return 0
  }
}

/// The gate of the vectors.
let g: Gate = { weight: 1 }
`

// verifying verifies each value stage B reaches into own, and a vector's into the bags it is given.
type verifying struct {
	verifyInto
	ev  *eval.Evaluator
	own check.Bags
}

func (h *verifying) Verify(ctx context.Context, root eval.Root, v value.Value) bool {
	res, err := verify.New(h.ev, h.prog, h.own, nil).Check(ctx, root, v)
	return err == nil && res.Valid
}

// EVALUATION.md §1, §10.2: a vector's value reaching an instance the build verified reports its E3802 aside.
func TestVectorReplaysRememberedFindings(t *testing.T) {
	ctx := context.Background()
	p := parseFiles(t, []string{"flow/flow.canon"}, [][]byte{[]byte(replaySrc)})
	own := check.Bags{flowPkg: diag.NewBag(p.fs, flowPkg)}
	prog := check.Check(ctx, exampleProject(), p.files, own, eval.NewFolder(own, eval.Options{}))
	assertEmpty(t, own)
	h := &verifying{verifyInto: verifyInto{prog: prog}, own: own}
	ev := eval.New(prog, h, own, eval.Options{})
	h.ev = ev
	for _, name := range []string{"things", "g"} {
		if _, ok := ev.Force(ctx, eval.Root{Pkg: flowPkg, Name: name}); !ok {
			t.Fatalf("%s is poisoned", name)
		}
	}
	ev.BeginVerification(ctx)
	g, _ := ev.Force(ctx, eval.Root{Pkg: flowPkg, Name: "g"})
	o := ev.Vector(ctx, eval.Call{Fn: progFnIn(t, prog, flowPkg, "Gate", "plus"), Recv: g, Args: ints(1)}, eval.VectorMode{})
	if o.Code != diag.E3802.Def().Code {
		t.Errorf("%+v, want code %s", o, diag.E3802.Def().Code)
	}
}
