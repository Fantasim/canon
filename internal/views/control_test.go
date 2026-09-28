package views_test

import (
	"reflect"
	"testing"
)

// typeRules is a record holding one field per row of VIEWMODEL.md §4.3 and §4.4.
const typeRules = `package a

local enum Small { a, b, c }
local enum Mid { a, b, c, d, e }
local enum Big { a, b, c, d, e, f, g, h, i, j, k }
local enum Worn { a, b, c, d, retired e }
local enum Flags @codes(UInt8) { x = 1, y = 2 }

local variant Shape {
  circle { r: Int }
  square { side: Int }
}

local record Tag {
  label: String
}

local let tags: table Tag = {
  t1 { label: "one" }
  t2 { label: "two" }
}

local record Pt {
  x: Int
  y: Int
}

local record Wide {
  a: Int
  b: Int
  c: Int
  d: Int
  e: Int
  f: Int
  g: Int
}

local record Item {
  on: Bool
  maybe: Bool?
  count: Int
  small: Int(1..=20)
  big: Int(1..=21)
  byte: UInt8
  ratio: Float(0.0..1.0)
  wait: Duration(0s..=1h) @json(unit: s)
  long: Duration
  name: String(1..=20)
  code: String(/^[A-Z]+$/)
  s: Small
  m: Mid
  b: Big
  w: Worn
  shape: Shape
  tag: ref tags
  lit: Small | "none"
  never: Never?
  pt: Pt
  wide: Wide
  set: [Small] where it.isUnique()
  bits: [Flags] @json(bits)
  smalls: [Small]
  refs: [ref tags]
  rng: [Int] where it.len() == 2 and it[0] < it[1]
  loose: [Int] where it[1] >= it[0] and it.len() == 2
  pos: [Int](1..=3)
  words: [String]
  pts: [Pt]
  keyed: [Pt] keyed by x
  nested: [[Int]]
  row: {Small: Int}
  cards: {String: Pt}
  kv: {String: Int}
  opt: Int? = 3
  optNone: Int? = none
  optSmall: Small?
}
`

// VIEWMODEL.md §4.3 C7–C33, §4.4 C35–C39, C4, C5, X12: each field's control by the type rules.
func TestTypeRules(t *testing.T) {
	x := demo(t, typeRules, "")
	item := x.named(t, demoPkg, "Item")
	r := x.resolver()
	for _, c := range []struct{ field, want, rule string }{
		{"on", `{"kind":"switch"}`, "C7"},
		{"maybe", `{"kind":"segmented","source":{"bool":true},"optional":{"unset":"segment"}}`, "C35"},
		{"count", `{"kind":"number"}`, "C12"},
		{"small", `{"kind":"number","min":1,"max":20,"stepper":true}`, "C12 stepper"},
		{"big", `{"kind":"number","min":1,"max":21}`, "C12 no stepper"},
		{"byte", `{"kind":"number","min":0,"max":255}`, "C12 sized limits"},
		{"ratio", `{"kind":"number","min":0,"max":1,"maxExclusive":true}`, "C12 float"},
		{"wait", `{"kind":"duration","min":0,"max":3600000,"units":["s","m","h"]}`, "C13 X12"},
		{"long", `{"kind":"duration","units":["ms","s","m","h","d"]}`, "C13 X12 unbounded"},
		{"name", `{"kind":"input","minLen":1,"maxLen":20}`, "C11"},
		{"code", `{"kind":"input","pattern":"^[A-Z]+$"}`, "C11 pattern"},
		{"s", `{"kind":"segmented","source":{"enum":"a.Small"}}`, "C4 C8"},
		{"m", `{"kind":"select","source":{"enum":"a.Mid"}}`, "C4 C8"},
		{"b", `{"kind":"search","source":{"enum":"a.Big"}}`, "C4 C8"},
		{"w", `{"kind":"segmented","source":{"enum":"a.Worn"}}`, "C3 active members"},
		{"shape", `{"kind":"variant","of":"a.Shape","selector":{"kind":"segmented","source":{"cases":"a.Shape"}}}`, "C9"},
		{"tag", `{"kind":"select","source":{"collection":"a:tags"}}`, "C5 C10"},
		{"lit", `{"kind":"segmented","source":{"enum":"a.Small"},"pinned":["none"]}`, "C15 D9"},
		{"never", `{"kind":"never"}`, "C17 C39"},
		{"pt", `{"kind":"section","of":"a.Pt"}`, "C18"},
		{"wide", `{"kind":"card","of":"a.Wide"}`, "C19 C31"},
		{"set", `{"kind":"checkboxes","source":{"enum":"a.Small"}}`, "C20 C32"},
		{"bits", `{"kind":"checkboxes","source":{"enum":"a.Flags"}}`, "C20 C32 bits"},
		{"smalls", `{"kind":"chips","source":{"enum":"a.Small"}}`, "C21"},
		{"refs", `{"kind":"chips","source":{"collection":"a:tags"}}`, "C22"},
		{"rng", `{"kind":"range","element":{"kind":"number"},"strict":true}`, "C23 C33"},
		{"loose", `{"kind":"range","element":{"kind":"number"}}`, "C23 C33 any order"},
		{"pos", `{"kind":"positional","element":{"kind":"number"},"count":3,"minCount":1}`, "C24"},
		{"words", `{"kind":"tags","element":{"kind":"input"},"orderable":true}`, "C25 T2 T20"},
		{"pts", `{"kind":"table","of":"a.Pt"}`, "C26"},
		{"keyed", `{"kind":"table","of":"a.Pt"}`, "C26"},
		{"nested", `{"kind":"list","element":{"kind":"tags","element":{"kind":"number"},"orderable":true},"orderable":true}`, "C27 T2"},
		{"row", `{"kind":"enumRow","source":{"enum":"a.Small"},"value":{"kind":"number"}}`, "C28"},
		{"cards", `{"kind":"cards","of":"a.Pt","keyControl":{"kind":"input"},"value":{"kind":"section","of":"a.Pt"}}`, "C29"},
		{"kv", `{"kind":"map","keyControl":{"kind":"input"},"value":{"kind":"number"}}`, "C30"},
		{"opt", `{"kind":"number","optional":{"unset":"clear","threeState":true}}`, "C35 C36"},
		{"optNone", `{"kind":"number","optional":{"unset":"clear"}}`, "C37"},
		{"optSmall", `{"kind":"segmented","source":{"enum":"a.Small"},"optional":{"unset":"segment"}}`, "C35"},
	} {
		got := canonical(t, r.Field(item, field(t, item, c.field)))
		if want := decode(t, []byte(c.want)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s %s:\n got %s\nwant %s", c.rule, c.field, text(got), c.want)
		}
	}
}
