package ir_test

import (
	"strings"
	"testing"
)

// CODEGEN.md §4.1, DECISIONS 278, 456: a ref keyed by a ref is an integer of its chain's leaf key; the leaf's @ts(bigint) makes every ref of the chain a bigint, so a value past Number.MAX_SAFE_INTEGER held by a ref two deep, or three, is not E8101.
func TestE8101RefChainLeafBigint(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(`package a

/// A leaf row.
record Leaf {
  /// Its key.
  n: Int @ts(bigint)
}

/// The leaves.
let leaves: [Leaf] keyed by n = [ { n: 9_007_199_254_740_993 } ]

/// A row keyed by a ref.
record Mid {
  /// Its key.
  k: ref leaves
}

/// The rows.
let mids: [Mid] keyed by k = [ { k: 9_007_199_254_740_993 } ]

/// A row keyed by a ref to a row keyed by a ref.
record Top {
  /// Its key.
  t: ref mids
}

/// The top rows.
let tops: [Top] keyed by t = [ { t: 9_007_199_254_740_993 } ]

/// A holder.
record Holder {
  /// A ref two deep.
  two: ref mids
  /// A ref three deep.
  three: ref tops
}

/// The holder.
let holder: Holder = { two: 9_007_199_254_740_993, three: 9_007_199_254_740_993 }

emit ts { out: "out/a.ts", values: [holder] }
`))
	w.add(t, "a.holder.json", []byte(`{"two": 9007199254740993, "three": 9007199254740993}`))
	w.build(t)
	if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
		t.Errorf("a ref chain whose leaf key is @ts(bigint) must hold unsafe integers:\n%s", out)
	}
}
