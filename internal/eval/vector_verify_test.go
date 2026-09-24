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

// flowSrc has a dangling ref that only a precomputed method, called by a translated one, reads.
const flowSrc = `/// Flows.
package flow

/// A status.
record Status {
  /// Its label.
  label: String
}

/// The statuses.
let statuses: stable table Status = {
  open { label: "Open" }
}

/// A status no table holds.
let initial: ref Status = "draft"

/// A gate.
record Gate {
  /// Its weight.
  weight: Int

  /// Whether the initial status is itself.
  export fn settled(self) -> Bool { return initial == initial }

  /// The weight plus x, once settled.
  export fn plus(self, x: Int) -> Int {
    if settled() {
      return weight + x
    }
    return 0
  }
}

/// The gate of the vectors.
let g: Gate = { weight: 1 }
`

const flowPkg = "flow"

// verifyInto verifies through the evaluator it is given, into the bags it is given.
type verifyInto struct {
	served
	prog *check.Program
}

func (h verifyInto) VerifyInto(ctx context.Context, ev *eval.Evaluator, root eval.Root, v value.Value, bags check.Bags) bool {
	res, err := verify.New(ev, h.prog, bags, nil).Check(ctx, root, v)
	if err != nil {
		return false
	}
	if res.Poisoned {
		ev.Poison(root)
	}
	for _, u := range res.Unbound {
		ev.ReportUnbound(root, u.Ref, u.Path)
	}
	return res.Valid
}

// EVALUATION.md §1: a value first forced in a vector is verified aside, its first error the code; else no outcome.
func TestVectorVerifies(t *testing.T) {
	ctx := context.Background()
	p := parseFiles(t, []string{"flow/flow.canon"}, [][]byte{[]byte(flowSrc)})
	own := check.Bags{flowPkg: diag.NewBag(p.fs, flowPkg)}
	prog := check.Check(ctx, exampleProject(), p.files, own, eval.NewFolder(own, eval.Options{}))
	assertEmpty(t, own)
	plus := progFnIn(t, prog, flowPkg, "Gate", "plus")
	for _, c := range []struct {
		host eval.Host
		code diag.Code
	}{
		{verifyInto{prog: prog}, diag.E3501.Def().Code},
		{served{}, ""},
	} {
		ev := eval.New(prog, c.host, own, eval.Options{})
		g, ok := ev.Force(ctx, eval.Root{Pkg: flowPkg, Name: "g"})
		if !ok {
			t.Fatal("g is poisoned")
		}
		o := ev.Vector(ctx, eval.Call{Fn: plus, Recv: g, Args: ints(1)}, eval.VectorMode{})
		if o.Code != c.code || o.Value != nil || o.Exceeded != eval.NoLimit {
			t.Errorf("%T: %+v, want code %q and no value", c.host, o, c.code)
		}
		assertEmpty(t, own)
	}
}

// progFnIn is the declared object of method name of record owner of package pkg.
func progFnIn(t *testing.T, prog *check.Program, pkg, owner, name string) check.Object {
	t.Helper()
	return fnObj(t, &build{checked: prog}, pkg, owner, name)
}
