package views_test

import (
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/views/control"
)

// readOnly holds a field for each read-only reason of VIEWMODEL.md C43 and each single-value
// form of C34.
const readOnly = `package a

enum One { only, retired gone }

record Ro {
  eq: Int(3..=3)
  fl: Float(0.5..=0.5)
  dur: Duration(1s..=1s)
  sum: Int where it == 2 + 3
  both: Int where it > 0 and 7 == it
  one: One
  dep: Int = 0 @deprecated("old")
  token: input String from env "TOKEN"
  ro: Int
  plain: Int
  maybe: Int(3..=3)?
  oldEq: Int(3..=3) = 3 @deprecated("old")
  roEq: Int(4..=4)
}

/// The one value holding the input.
let cfg: Ro = { eq: 3, fl: 0.5, dur: 1s, sum: 5, both: 7, one: only, ro: 1, plain: 1, roEq: 4 }

view Ro {
  ro { readonly: true }
  roEq { readonly: true }
}
`

// VIEWMODEL.md C34, C43: read-only for an input, a deprecation, a single admitted value (carried,
// J10) or the view property, the first of these when several apply; an optional has no single.
func TestReadOnly(t *testing.T) {
	x := demo(t, readOnly, "")
	rec := x.named(t, demoPkg, "Ro")
	r := x.resolver()
	for _, c := range []struct{ field, reason, single string }{
		{"eq", control.ReadonlySingle, "3"},
		{"fl", control.ReadonlySingle, "0.5"},
		{"dur", control.ReadonlySingle, "1000"},
		{"sum", control.ReadonlySingle, "5"},
		{"both", control.ReadonlySingle, "7"},
		{"one", control.ReadonlySingle, `"only"`},
		{"dep", control.ReadonlyDeprecated, ""},
		{"token", control.ReadonlyInput, ""},
		{"ro", control.ReadonlyView, ""},
		{"plain", "", ""},
		{"maybe", "", ""},
		{"oldEq", control.ReadonlyDeprecated, "3"},
		{"roEq", control.ReadonlySingle, "4"},
	} {
		reason, single := r.ReadOnly(field(t, rec, c.field))
		if reason != c.reason || string(single) != c.single {
			t.Errorf("C34 C43 %s: ReadOnly = %q %s, want %q %s", c.field, reason, single, c.reason, c.single)
		}
	}
}

// VIEWMODEL.md C46: in a table cell a switch is a checkbox, segmented and radio are selects,
// and the optional wrapper clears; every other control is the field's.
func TestCell(t *testing.T) {
	segment := &vm.ControlOptional{Unset: "segment"}
	for _, c := range []struct{ in, want vm.Control }{
		{vm.Control{Kind: control.CtlSwitch}, vm.Control{Kind: control.CtlCheckbox}},
		{vm.Control{Kind: control.CtlSegmented, Optional: segment}, vm.Control{Kind: control.CtlSelect, Optional: &vm.ControlOptional{Unset: "clear"}}},
		{vm.Control{Kind: control.CtlRadio}, vm.Control{Kind: control.CtlSelect}},
		{vm.Control{Kind: control.CtlNumber, Unit: "hp"}, vm.Control{Kind: control.CtlNumber, Unit: "hp"}},
	} {
		if got := control.Cell(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Cell(%s) = %s, want %s", text(c.in), text(got), text(c.want))
		}
	}
	if segment.Unset != "segment" {
		t.Error("Cell changed the control it was given")
	}
}

// sibling holds a ref into a collection of the enclosing instance (TYPES.md §10.2 level 1).
const sibling = `package a

record Tree {
  nodes: [Node] keyed by id
}

record Node {
  id: String
  parent: ref Node?
}
`

// VIEWMODEL.md C6, J12: a ref into a collection of the enclosing instance is a select over that
// collection, walked up from the declaring record, and its type expression names the field.
func TestSiblingRef(t *testing.T) {
	x := demo(t, sibling, "")
	node := x.named(t, demoPkg, "Node")
	got := canonical(t, x.resolver().Field(node, field(t, node, "parent")))
	want := decode(t, []byte(`{"kind":"select","source":{"sibling":{"up":1,"field":"nodes"}},"optional":{"unset":"clear"}}`))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parent control:\n got %s\nwant %s", text(got), text(want))
	}
	typ := member(canonical(t, x.model(t, demoPkg).Types["a.Node"].Fields[1]), "type", "of")
	want = decode(t, []byte(`{"kind":"ref","sibling":{"up":1,"field":"nodes"},"element":"a.Node","keyType":"string","count":0,"active":0}`))
	if !reflect.DeepEqual(typ, want) {
		t.Errorf("parent type:\n got %s\nwant %s", text(typ), text(want))
	}
}
