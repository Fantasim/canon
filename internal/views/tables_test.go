package views_test

import "testing"

// tables are collections of records whose views declare columns and filters, and one without.
const tables = `package a

enum Tier { low, high }

record Tag {
  label: String
}

let tags: table Tag = {
  t1 { label: "one" }
  t2 { label: "two" }
}

variant Shape {
  circle { r: Int }
  square { side: Int }
}

record Sub {
  x: Int
}

record Row {
  code: String
  tier: Tier
  on: Bool
  size: Int?
  fixed: Int(3..=3)
  names: [String]
  shape: Shape
  tag: ref tags
  labels: [ref tags]
  sub: Sub
  gone: Int = 0 @deprecated("Old")
  quiet: Int = 0
}

view Row {
  title "{code}"
  singular "a row"
  columns { code 120, tier, on, size 16, fixed, names, shape, sub, gone, tag }
  filters { tier multi, on, size, shape, labels, tag }
  quiet { hidden: true }
}

record Plain {
  a: Int
  b: Int
  hid: Int = 0
  c: Int
  old: Int = 0 @deprecated("Old")
  d: Int
  sub: Sub
  e: Int
  f: Int
  g: Int
}

view Plain {
  hid { hidden: true }
}

let rows: [Row] keyed by code = []
let plains: table Plain = {}
let list: [Plain] = []
`

// VIEWMODEL.md T1, T3, T6, T7, T8, C46: a keyed list of records is a table keyed by its key
// field; the key column's width goes to the entry column; each declared column has its mode,
// `edit` ones their cell control.
func TestTableColumns(t *testing.T) {
	ctl := demo(t, tables, "").model(t, demoPkg).Values["a:rows"].Control
	expect(t, "T1 T3", []any{ctl.Key, ctl.Orderable, ctl.Singular, ctl.Search}, `["code",true,"a:Row.singular","a:rows"]`)
	expect(t, "T6 T7 T8 C46", ctl.Columns, `[
	 {"field":"$entry","width":120,"mode":"entry"},
	 {"field":"tier","mode":"edit","cell":{"kind":"select","source":{"enum":"a.Tier"}}},
	 {"field":"on","mode":"edit","cell":{"kind":"checkbox"}},
	 {"field":"size","width":16,"mode":"edit","cell":{"kind":"number","optional":{"unset":"clear"}}},
	 {"field":"fixed","mode":"value"},
	 {"field":"names","mode":"count"},
	 {"field":"shape","mode":"case"},
	 {"field":"sub","mode":"text"},
	 {"field":"gone","mode":"value"},
	 {"field":"tag","mode":"edit","cell":{"kind":"select","source":{"collection":"a:tags"}}}]`)
}

// VIEWMODEL.md T11, T12, T14: each filter's kind and control: a choice, with `multi` checkboxes
// for at most 6 choices; a case; a bool; a range, `none`
// for an optional field; a contains over a list of refs.
func TestTableFilters(t *testing.T) {
	ctl := demo(t, tables, "").model(t, demoPkg).Values["a:rows"].Control
	expect(t, "T11 T12", ctl.Filters, `[
	 {"field":"tier","kind":"choice","multi":true,"control":{"kind":"checkboxes","source":{"enum":"a.Tier"}}},
	 {"field":"on","kind":"bool","control":{"kind":"segmented","source":{"bool":true}}},
	 {"field":"size","kind":"range","none":true,"control":{"kind":"range","element":{"kind":"number"}}},
	 {"field":"shape","kind":"case","control":{"kind":"segmented","source":{"cases":"a.Shape"}}},
	 {"field":"labels","kind":"contains","control":{"kind":"select","source":{"collection":"a:tags"}}},
	 {"field":"tag","kind":"choice","control":{"kind":"select","source":{"collection":"a:tags"}}}]`)
}

// VIEWMODEL.md T1, T6, T7: a table is keyed by `$id`, with an entry column; without `columns`
// the first 6 scalar fields, but hidden, deprecated and non-scalar ones; a plain list has no key
// and, without a title, no entry column.
func TestTableDefaults(t *testing.T) {
	m := demo(t, tables, "").model(t, demoPkg)
	auto := `{"field":"a","mode":"edit","cell":{"kind":"number"}},{"field":"b","mode":"edit","cell":{"kind":"number"}},
	 {"field":"c","mode":"edit","cell":{"kind":"number"}},{"field":"d","mode":"edit","cell":{"kind":"number"}},
	 {"field":"e","mode":"edit","cell":{"kind":"number"}},{"field":"f","mode":"edit","cell":{"kind":"number"}}`
	expect(t, "T1 T6 T7 table", m.Values["a:plains"].Control, `{"kind":"table","of":"a.Plain","key":"$id","orderable":true,
	 "columns":[{"field":"$entry","mode":"entry"},`+auto+`],"search":"a:plains"}`)
	expect(t, "T1 T7 list", m.Values["a:list"].Control, `{"kind":"table","of":"a.Plain","orderable":true,"columns":[`+auto+`]}`)
}

// VIEWMODEL.md T2, 12.6 `sources`: rows loaded by `load.dir` or declared as entries in other
// files do not move (API.md reason `order`).
func TestTableOrder(t *testing.T) {
	x := examples(t)
	if ctl := x.model(t, pipelinePkg).Values[potionsValue].Control; ctl.Orderable {
		t.Error("T2: a keyed list loaded by load.dir is orderable")
	}
	v := x.model(t, "game.items").Values["game.items:items"]
	if v.Control.Orderable || v.Sources.Entries == nil || *v.Sources.Entries != 2 {
		t.Errorf("T2: game.items:items orderable %v, entries %v", v.Control.Orderable, v.Sources.Entries)
	}
}

// variants is a table of variants whose view names case fields in its columns and filters.
const variants = `package a

enum Tier { low, high }

variant Shape {
  circle { r: Int, tier: Tier }
  square { r: Int, side: String }
}

view Shape {
  title "Shape"
  columns { r 50, side }
  filters { tier, r }
}

let shapes: [Shape] = [circle { r: 1, tier: low }]
`

// VIEWMODEL.md T6a, T9, 2 (field key), 12.5 `columns`: a table of variants has the case column
// `$case` after the entry column; its view's columns and filters name case fields, each keyed
// `<case>.<field>` for every case holding it (rows lacking it show an empty cell, T9).
func TestTableOfVariants(t *testing.T) {
	m := demo(t, variants, "").model(t, demoPkg)
	for _, e := range wholeSchema(t).Validate(written(t, m)) {
		t.Errorf("V2: %v", e)
	}
	ctl := m.Values["a:shapes"].Control
	expect(t, "T6a T9", ctl.Columns, `[{"field":"$entry","mode":"entry"},{"field":"$case","mode":"case"},
	 {"field":"circle.r","width":50,"mode":"edit","cell":{"kind":"number"}},
	 {"field":"square.r","width":50,"mode":"edit","cell":{"kind":"number"}},
	 {"field":"square.side","mode":"edit","cell":{"kind":"input"}}]`)
	expect(t, "T6a T11 T13", ctl.Filters, `[
	 {"field":"circle.tier","kind":"choice","control":{"kind":"segmented","source":{"enum":"a.Tier"}}},
	 {"field":"circle.r","kind":"range","control":{"kind":"range","element":{"kind":"number"}}},
	 {"field":"square.r","kind":"range","control":{"kind":"range","element":{"kind":"number"}}}]`)
}
