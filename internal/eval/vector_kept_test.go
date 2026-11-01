package eval_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// keptVectorSrc has a lookup comparing with a kept-map literal whose d holds an Int, which a translated method calls.
const keptVectorSrc = `/// Kept.
package kept

/// A kind.
enum K { num, sub }

/// A node.
record Node {
  /// Its kind.
  k: K
}

/// The nodes.
let nodes: table Node = { d { k: sub }, n { k: num } }

/// What a node holds.
type P(e: Node) = match e.k {
  num => Int
  sub => String
}

/// A holder.
record H {
  /// By node.
  by: {x in nodes: P(x)} = {}
}

/// The holders.
let hs: [H] = [{ by: { d: "x", n: 1 } }]

/// A lookup comparing with a literal never stored.
export fn bad() -> Bool {
  return hs[0].by != { d: 3 }
}

/// A potion.
record Potion {
  /// Its heal.
  heal: Int

  /// The heal plus x when bad holds.
  export fn healFor(self, x: Int) -> Int { return if bad() { heal + x } else { x } }
}

/// The potion of the vectors.
let pot: Potion = { heal: 5 }
`

const keptPkg = "kept"

// keptHost verifies values and kept literals through the evaluator it is given, as build's host does.
type keptHost struct {
	prog *check.Program
	bags check.Bags
	ev   *eval.Evaluator
}

func (*keptHost) Load(context.Context, *syntax.LoadExpr, types.Type) (value.Value, bool) {
	return nil, false
}

func (h *keptHost) Verify(ctx context.Context, root eval.Root, v value.Value) bool {
	return h.VerifyInto(ctx, h.ev, root, v, h.bags)
}

func (h *keptHost) VerifyLiterals(ctx context.Context, root eval.Root, lits []eval.KeptLiteral) {
	h.VerifyLiteralsInto(ctx, h.ev, root, lits, h.bags)
}

func (h *keptHost) VerifyInto(ctx context.Context, ev *eval.Evaluator, root eval.Root, v value.Value, bags check.Bags) bool {
	res, err := verify.New(ev, h.prog, bags, nil).Check(ctx, root, v)
	return settled(ev, root, res, err)
}

func (h *keptHost) VerifyLiteralsInto(ctx context.Context, ev *eval.Evaluator, root eval.Root, lits []eval.KeptLiteral, bags check.Bags) {
	vr := verify.New(ev, h.prog, bags, nil)
	for _, l := range lits {
		res, err := vr.CheckLiteral(ctx, root, l.Value, l.Type)
		settled(ev, root, res, err)
	}
}

// DECISIONS 408, CONFORMANCE.md §6.5: a vector's unstored kept-map literal is judged, its E3802 the outcome.
func TestVectorJudgesKeptLiterals(t *testing.T) {
	ctx := context.Background()
	p := parseFiles(t, []string{"kept/kept.canon"}, [][]byte{[]byte(keptVectorSrc)})
	own := check.Bags{keptPkg: diag.NewBag(p.fs, keptPkg)}
	prog := check.Check(ctx, exampleProject(), p.files, own, eval.NewFolder(own, eval.Options{}))
	assertEmpty(t, own)
	h := &keptHost{prog: prog, bags: own}
	h.ev = eval.New(prog, h, own, eval.Options{})
	h.ev.BeginVerification(ctx)
	pot, ok := h.ev.Force(ctx, eval.Root{Pkg: keptPkg, Name: "pot"})
	if !ok {
		t.Fatal("pot is poisoned")
	}
	assertEmpty(t, own)
	heal := progFnIn(t, prog, keptPkg, "Potion", "healFor")
	o := h.ev.Vector(ctx, eval.Call{Fn: heal, Recv: pot, Args: ints(1)}, eval.VectorMode{})
	if want := diag.E3802.Def().Code; o.Code != want || o.Value != nil {
		t.Errorf("healFor(1): %+v, want code %s and no value", o, want)
	}
	assertEmpty(t, own)
}
