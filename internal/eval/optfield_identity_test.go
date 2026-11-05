package eval_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// optFieldLaw holds keyed lists and tables in optional record fields, beside a plain one, in two
// instances of A and in an applied record.
const optFieldLaw = `-- a/a.canon --
/// A.
package a

/// S.
record S {
  /// Name.
  name: String
  /// W.
  w: Int = 0
}

/// A.
record A {
  /// Id.
  id: String
  /// Optional keyed list.
  oo: ([S] keyed by name)?
  /// Optional table.
  tt: (table S)?
  /// Plain keyed list.
  pl: [S] keyed by name = []
}

/// Ev.
record Ev {
  /// Max.
  max: Int = 3
}

/// Applied.
record P(e: Ev) {
  /// Optional keyed list.
  oo: ([S] keyed by name)?
  /// Plain keyed list.
  pl: [S] keyed by name = []
}

/// Holder.
record Holder {
  /// Ev.
  ev: Ev = {}
  /// P.
  p: P(ev)
}

/// Two instances.
let aa: [A] keyed by id = [
  { id: "first", oo: [{ name: "x" }], tt: { x { name: "X" } }, pl: [{ name: "x" }] },
  { id: "second", oo: [{ name: "x" }, { name: "z" }] },
]

/// Holder.
let holder: Holder = { p: { oo: [{ name: "x" }], pl: [{ name: "x" }] } }
`

// TYPES.md §9.4, §10.2, DECISIONS 411: an optional field's element takes no field identity; a plain field's does.
func TestOptionalFieldIdentity(t *testing.T) {
	b := runBuild(t, fromArchive(t, txtar.Parse([]byte(optFieldLaw))), eval.Options{})
	for _, f := range b.bags["a"].Findings() {
		if f.Severity == diag.Error {
			t.Fatalf("a: %s %s", f.Code, f.Message)
		}
	}
	aa, ok := b.values[eval.Root{Pkg: "a", Name: "aa"}].(*value.List)
	if !ok || len(aa.Elems) != len([]string{"first", "second"}) {
		t.Fatalf("a.aa: %v, want a two-element list", b.values[eval.Root{Pkg: "a", Name: "aa"}])
	}
	first, _ := aa.Elems[0].(*value.Record)
	second, _ := aa.Elems[1].(*value.Record)
	p, _ := fieldValue(rootRecord(t, b, "holder"), "p").(*value.Record)
	if first == nil || second == nil || p == nil {
		t.Fatal("a.aa's elements or holder.p are no records")
	}
	for _, c := range []struct {
		name, field, key string
		owner            *value.Record
		fieldIdentity    bool
	}{
		{"aa.first.oo.x", "oo", "x", first, false},
		{"aa.second.oo.x", "oo", "x", second, false},
		{"aa.second.oo.z", "oo", "z", second, false},
		{"aa.first.tt.x", "tt", "x", first, false},
		{"aa.first.pl.x", "pl", "x", first, true},
		{"holder.p.oo.x", "oo", "x", p, false},
		{"holder.p.pl.x", "pl", "x", p, true},
	} {
		el := entryOf(c.owner, c.field, value.Key{S: c.key})
		if el == nil || el.Ident == nil {
			t.Errorf("%s: no element", c.name)
			continue
		}
		id := el.Ident
		isField := id.Coll != nil && id.Coll.Kind == types.CollField
		if isField != c.fieldIdentity || isField && (id.Coll.Owner != recordType(c.owner.T) || !slices.Equal(id.Coll.FieldPath, []string{c.field})) {
			t.Errorf("DECISIONS 411: %s: collection %+v, field identity %v", c.name, id.Coll, c.fieldIdentity)
		}
		if owned := id.Owner != nil && sameInstance(b.ev, id.Owner, c.owner); owned != c.fieldIdentity || !c.fieldIdentity && id.Owner != nil {
			t.Errorf("DECISIONS 411: %s: owner %p, owned by its instance %v", c.name, id.Owner, c.fieldIdentity)
		}
	}
}
