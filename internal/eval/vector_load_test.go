package eval_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// loadSrc reads a loaded value only through a precomputed method a translated one calls.
const loadSrc = `/// Loads.
package loads

/// A loaded base.
let base: Int = load("@resource/base.json")

/// A counter.
record Counter {
  /// Its step.
  step: Int

  /// The loaded base.
  export fn baseValue(self) -> Int { return base }

  /// The base plus x.
  export fn plus(self, x: Int) -> Int { return baseValue() + x }
}

/// The counter of the vectors.
let c: Counter = { step: 1 }
`

const (
	loadsPkg   = "loads"
	loadedBase = 41
)

// loadHost serves every load a base value, reporting a finding where it is told to.
type loadHost struct {
	own   check.Bags
	given []check.Bags
	loads int
	fail  bool // LoadInto reports an error into the bags it is given
}

func (h *loadHost) Load(_ context.Context, e *syntax.LoadExpr, _ types.Type) (value.Value, bool) {
	h.loads++
	return &value.Int{V: loadedBase, T: types.IntType}, true
}

func (h *loadHost) Verify(context.Context, eval.Root, value.Value) bool { return true }

// loadInto serves a load, and verifies, into the bags it is given.
type loadInto struct {
	*loadHost
}

func (h loadInto) LoadInto(_ context.Context, _ *syntax.LoadExpr, _ types.Type, bags check.Bags) (value.Value, bool) {
	h.given = append(h.given, bags)
	if h.fail {
		diag.E4102.At(source.Span{}).Report(bags[loadsPkg])
	}
	return &value.Int{V: loadedBase, T: types.IntType}, true
}

func (h loadInto) VerifyInto(context.Context, *eval.Evaluator, eval.Root, value.Value, check.Bags) bool {
	return true
}

// loadCase is one host of TestVectorLoads: whether it loads into given bags, and fails there.
type loadCase struct {
	into, fail bool
}

// DECISIONS 204: a load first forced inside a vector reports into discarded bags, never the
// host's, its first error the vector's code; without LoadInto the vector has no outcome.
func TestVectorLoads(t *testing.T) {
	ctx := context.Background()
	p := parseFiles(t, []string{"loads/loads.canon"}, [][]byte{[]byte(loadSrc)})
	folded := check.Bags{}
	prog := check.Check(ctx, exampleProject(), p.files, folded, eval.NewFolder(folded, eval.Options{}))
	assertEmpty(t, folded)
	for _, c := range []loadCase{{into: true}, {into: true, fail: true}, {}} {
		lh := &loadHost{own: check.Bags{loadsPkg: diag.NewBag(p.fs, loadsPkg)}, fail: c.fail}
		var h eval.Host = lh
		if c.into {
			h = loadInto{lh}
		}
		ev := eval.New(prog, h, lh.own, eval.Options{})
		recv, ok := ev.Force(ctx, eval.Root{Pkg: loadsPkg, Name: "c"})
		if !ok {
			t.Fatal("c is poisoned")
		}
		o := ev.Vector(ctx, eval.Call{Fn: progFnIn(t, prog, loadsPkg, "Counter", "plus"), Recv: recv, Args: ints(1)}, eval.VectorMode{})
		if lh.loads != 0 || c.into && (len(lh.given) != 1 || lh.given[0][loadsPkg] == lh.own[loadsPkg]) {
			t.Errorf("%+v: %d own loads, %d loads into bags", c, lh.loads, len(lh.given))
		}
		checkLoadOutcome(t, c, o)
		assertEmpty(t, lh.own)
		if _, ok := ev.Force(ctx, eval.Root{Pkg: loadsPkg, Name: "base"}); !ok || lh.loads != 1 {
			t.Errorf("%+v: base after the vector: %t, %d loads; want it forced afresh", c, ok, lh.loads)
		}
	}
}

// checkLoadOutcome fails unless o is the loaded base plus one, the load's error, or no outcome.
func checkLoadOutcome(t *testing.T, c loadCase, o eval.Outcome) {
	t.Helper()
	switch {
	case c.fail:
		expectOutcome(t, o, 0, diag.E4102.Def().Code)
	case c.into:
		expectOutcome(t, o, loadedBase+1, "")
	case o.Value != nil || o.Code != "" || o.Exceeded != eval.NoLimit:
		t.Errorf("%+v: %+v, want no outcome", c, o)
	}
}
