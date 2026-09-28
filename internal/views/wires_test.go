package views_test

import (
	"reflect"
	"testing"
)

// wires uses every wire annotation of a field definition, an ordered enum and an input field.
const wires = `package a

enum Level ordered { low, high }

enum Flags @codes(UInt8) { x = 1, y = 2 }

record StatBonus {
  attribute: Int
  value: Int
}

variant Style {
  melee { reach: Int }
  ranged { range: Int }
}

record Gear {
  level: Level
  flags: [Flags] = [] @json(bits)
  stats: [StatBonus](..=6) = [] @json(pairs: ["dst{i}", "adj{i}"])
  style: Style @json(inline)
  token: input String from env "GEAR_TOKEN"
}

/// The gear.
let gear: Gear = { level: low, style: melee { reach: 1 } }
`

// VIEWMODEL.md C32 and the Field and wire of 12.3: bits, pairs, inline, an input (never
// required) and an ordered enum.
func TestWireDefinitions(t *testing.T) {
	m := demo(t, wires, "").model(t, demoPkg)
	if !m.Types["a.Level"].Ordered {
		t.Error("a.Level is not ordered")
	}
	got := canonical(t, m.Types["a.Gear"].Fields[1:])
	want := decode(t, []byte(`[
		{"name":"flags","type":{"kind":"list","of":{"kind":"enum","ref":"a.Flags"},"unique":true},"default":[],"wire":{"name":"flags","bits":true}},
		{"name":"stats","type":{"kind":"list","of":{"kind":"record","ref":"a.StatBonus"},"max":6},"default":[],
			"wire":{"name":"dst{i}","pairs":{"keys":["dst{i}","adj{i}"],"slots":6}}},
		{"name":"style","type":{"kind":"variant","ref":"a.Style"},"required":true,"wire":{"name":"style","inline":true}},
		{"name":"token","type":{"kind":"string"},"wire":{"name":"token"},"input":{"env":"GEAR_TOKEN"}}]`))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Gear fields:\n got %s\nwant %s", text(got), text(want))
	}
}

// VIEWMODEL.md C9, L17: an @json(inline) variant field's control is marked inline.
func TestInlineVariantControl(t *testing.T) {
	x := demo(t, wires, "")
	gear := x.named(t, demoPkg, "Gear")
	got := canonical(t, x.resolver().Field(gear, field(t, gear, "style")))
	want := decode(t, []byte(`{"kind":"variant","of":"a.Style","inline":true,"selector":{"kind":"segmented","source":{"cases":"a.Style"}}}`))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("style control:\n got %s\nwant %s", text(got), text(want))
	}
}
