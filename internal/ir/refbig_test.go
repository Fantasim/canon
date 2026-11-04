package ir_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// refBigSource is a chain of refs three deep whose leaf key, Leaf.n, is @ts(bigint), and a chain whose leaf key is not.
const refBigSource = `package a

/// A leaf row.
record Leaf {
  /// Its key.
  n: Int @ts(bigint)
}

/// The leaves.
let leaves: [Leaf] keyed by n = []

/// A row keyed by a ref.
record Mid {
  /// Its key.
  k: ref leaves
}

/// The rows.
let mids: [Mid] keyed by k = []

/// A row keyed by a ref to a row keyed by a ref.
record Top {
  /// Its key.
  t: ref mids
}

/// The top rows.
let tops: [Top] keyed by t = []

/// A plain row.
record Plain {
  /// Its key.
  p: Int
}

/// The plain rows.
let plains: [Plain] keyed by p = []

/// A row keyed by a ref to a plain row.
record Over {
  /// Its key.
  o: ref plains
}

/// The rows over plain rows.
let overs: [Over] keyed by o = []

/// A holder.
record Holder {
  /// A ref one deep.
  one: ref leaves
  /// A ref two deep.
  two: ref mids
  /// A ref three deep.
  three: ref tops
  /// A ref two deep to a plain leaf.
  plain: ref overs
}

emit ts { out: "@features/a/a.ts", mode: types }
`

// CODEGEN.md §4.1, DECISIONS 278, 456: RefTarget.BigInt is the chain's leaf key field's @ts(bigint), at every level of the chain, and false for a leaf without it.
func TestRefTargetBigIntAtEveryLevel(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(refBigSource))
	pkgs := w.build(t)
	if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	holder := recordIn(t, pkgs, "Holder")
	cases := []struct {
		field string
		depth int
		want  bool
	}{
		{"one", 1, true},
		{"two", 2, true},
		{"three", 3, true},
		{"plain", 2, false},
	}
	for _, c := range cases {
		ref := fieldIn(t, holder, c.field).Type
		for level := range c.depth {
			if ref.Ref == nil || ref.Ref.BigInt != c.want {
				t.Errorf("%s: level %d of %d: RefTarget = %+v, want BigInt %v", c.field, level+1, c.depth, ref.Ref, c.want)
			}
			if ref.Key != nil {
				ref = *ref.Key
			}
		}
	}
}

// recordIn is the record named name of pkgs.
func recordIn(t *testing.T, pkgs []*ir.Package, name string) *ir.Record {
	t.Helper()
	for _, p := range pkgs {
		for _, ty := range p.Types {
			if r, ok := ty.(*ir.Record); ok && r.Name == name {
				return r
			}
		}
	}
	t.Fatalf("no record %s", name)
	return nil
}

// fieldIn is r's field named name.
func fieldIn(t *testing.T, r *ir.Record, name string) *ir.Field {
	t.Helper()
	for _, f := range r.Fields {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no field %s.%s", r.Name, name)
	return nil
}
