package views_test

import (
	"reflect"
	"testing"
)

// studioSrc is a studio package: a unit, a default widget and two plain ones (VIEWMODEL.md
// 3.8, G22).
const studioSrc = `/// The studio.
package studio

/// Menus.
enum Menu { items }

/// Icons.
enum Icon { gem, gear }

/// Tones.
enum Tone { info, danger }

/// A unit.
record UnitSpec {
  /// Appended.
  suffix: String = ""
}

/// Units.
let units: table UnitSpec = {
  hp { suffix: " HP" }
}

/// A time of day.
record Clock {
  /// Hours.
  h: Int
  /// Minutes.
  m: Int
}

/// One input for a time of day.
widget clock(value: Clock) default

/// Numbered steps.
widget ordered_steps(value: [_])

/// A weight and its share.
widget weight_share(value: Int, siblings: [Int])
`

// hinted gives each field a `control:`, `widget:` or `unit:` (VIEWMODEL.md §3.5).
const hinted = `package a

import studio

local enum Small { a, b, c }

local variant Shape {
  circle { r: Int }
  square { side: Int }
}

local record Tag {
  label: String
}

local let tags: table Tag = {
  t1 { label: "one" }
}

local record Hinted {
  a: Bool
  b: Small
  c: Shape
  d: ref tags
  e: [Small]
  f: String
  g: String
  h: Int
  i: Duration(0s..=1m)
  j: Int(0..=100)
  k: String(/^#[0-9a-fA-F]{6}$/)
  l: UInt32
  m: [Int]
  n: Int
  o: Int?
  p: [Int]
  q: studio.Clock
  r: studio.Clock
  s: Int
  t: {String: Int}
  u: {Small: Int}
  v: studio.Clock?
  w: Bool?
}

view Hinted {
  a { control: checkbox }
  b { control: radio }
  c { control: select }
  d { control: search }
  e { control: chips }
  f { control: textarea }
  g { control: code, placeholder: "code" }
  h { control: stepper }
  i { control: number }
  group more "More" {
    j { control: slider }
  }
  j { unit: hp }
  k { control: color }
  l { control: color }
  m { control: text }
  n { unit: hp, placeholder: "zero" }
  o { control: number, unit: hp }
  p { widget: ordered_steps, unit: hp }
  r { control: text }
  s { widget: weight_share }
  t { unit: hp }
  u { unit: hp }
  w { control: switch }
}
`

// VIEWMODEL.md C1, 4.5, C41, G22, C45, C2: hints resolve to their kinds, a widget carries its
// fallback, a default widget applies without a hint, a unit sits on the control showing the
// number, on whichever line it is given.
func TestHintsAndWidgets(t *testing.T) {
	x := demo(t, hinted, studioSrc)
	rec := x.named(t, demoPkg, "Hinted")
	r := x.resolver()
	for _, c := range []struct{ field, want, rule string }{
		{"a", `{"kind":"checkbox"}`, "§4.5 checkbox"},
		{"b", `{"kind":"radio","source":{"enum":"a.Small"}}`, "§4.5 radio"},
		{"c", `{"kind":"variant","of":"a.Shape","selector":{"kind":"select","source":{"cases":"a.Shape"}}}`, "§4.5 case selector"},
		{"d", `{"kind":"search","source":{"collection":"a:tags"}}`, "§4.5 search on a ref"},
		{"e", `{"kind":"chips","source":{"enum":"a.Small"}}`, "§4.5 chips"},
		{"f", `{"kind":"textarea"}`, "§4.5 textarea"},
		{"g", `{"kind":"code"}`, "§4.5 code"},
		{"h", `{"kind":"number","stepper":true}`, "§4.5 stepper"},
		{"i", `{"kind":"duration","min":0,"max":60000,"units":["ms","s","m"]}`, "§4.5 number on a Duration"},
		{"j", `{"kind":"slider","min":0,"max":100,"unit":"hp"}`, "§4.5 slider, C45 on another line"},
		{"k", `{"kind":"color"}`, "§4.5 color on the pattern"},
		{"l", `{"kind":"color"}`, "§4.5 color on UInt32"},
		{"m", `{"kind":"text"}`, "§4.5 text"},
		{"n", `{"kind":"number","unit":"hp"}`, "C45"},
		{"o", `{"kind":"number","unit":"hp","optional":{"unset":"clear"}}`, "C2 C35"},
		{"p", `{"kind":"widget","widget":"ordered_steps","fallback":{"kind":"tags","element":{"kind":"number","unit":"hp"}}}`, "C41 C45 element"},
		{"q", `{"kind":"widget","widget":"clock","fallback":{"kind":"section","of":"studio.Clock"}}`, "G22 C41"},
		{"r", `{"kind":"text"}`, "G22 a hint wins over a default widget"},
		{"s", `{"kind":"widget","widget":"weight_share","siblings":true,"fallback":{"kind":"number"}}`, "C41 G21"},
		{"t", `{"kind":"map","keyControl":{"kind":"input"},"value":{"kind":"number","unit":"hp"}}`, "C45 map value"},
		{"u", `{"kind":"enumRow","source":{"enum":"a.Small"},"value":{"kind":"number","unit":"hp"}}`, "C45 enumRow value"},
		{"v", `{"kind":"widget","widget":"clock","optional":{"unset":"clear"},"fallback":{"kind":"section","of":"studio.Clock","optional":{"unset":"clear"}}}`, "G20 T? C2"},
		{"w", `{"kind":"switch","optional":{"unset":"clear"}}`, "C35 a hint on Bool?"},
	} {
		got := canonical(t, r.Field(rec, field(t, rec, c.field)))
		if want := decode(t, []byte(c.want)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s %s:\n got %s\nwant %s", c.rule, c.field, text(got), c.want)
		}
	}
}

// VIEWMODEL.md 3.5: `placeholder` takes a resolved input, textarea, code, number, segmented,
// select or search, and nothing else.
func TestTakesPlaceholder(t *testing.T) {
	x := demo(t, hinted, studioSrc)
	rec := x.named(t, demoPkg, "Hinted")
	r := x.resolver()
	for _, c := range []struct {
		field string
		want  bool
	}{
		{"f", true}, {"g", true}, {"h", true}, {"n", true}, {"o", true}, {"d", true}, {"b", false},
		{"a", false}, {"c", false}, {"i", false}, {"j", false}, {"m", false}, {"p", false}, {"q", false}, {"t", false},
	} {
		if got := r.TakesPlaceholder(field(t, rec, c.field)); got != c.want {
			t.Errorf("TakesPlaceholder(%s) = %v, want %v", c.field, got, c.want)
		}
	}
}
