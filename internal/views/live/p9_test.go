package live_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/live"
)

// keyed holds maps whose key types are a literal union, directly and through the arm a dependent
// type computes for the instance.
var keyed = map[string]string{
	"a/a.canon": `package a

enum Kind { color, other }

enum Color { red, blue }

type P(k: Kind) = match k {
  color => Color
  other => String
}

type SK(k: Kind) = P(k) | "none"

type W(k: Kind) = match k {
  color => {SK(k): Int}
  other => Int
}

record Holder {
  k: Kind
  plain: {String | "none": Int} = {}
  dep: W(k)
}

let holder: Holder = { k: color, plain: { "none": 1, "other": 2 }, dep: { "none": 3, red: 4 } }
`,
}

// API.md P9, V6 (log-2026-09-29 "P9 quoting rule", U13): a map's element headings are keyed by
// the path verification and the rules write: a literal of the key type declared at the map,
// computed for the instance when dependent, is a JSON string; any other word stays bare.
func TestMapKeyHeadings(t *testing.T) {
	p := demo(t, "", keyed)
	holder := p.let(t, "a", "holder").(*value.Record)
	res := at(t, p.input(), live.Target{Value: holder, Name: "holder"})
	same(t, "API.md P9 fields", slices.Sorted(maps.Keys(res.Headings)), `["dep[\"none\"]","dep[red]","plain[\"none\"]","plain[other]"]`)
	f := fieldNamed(holder.T, "plain")
	res = at(t, p.input(), live.Target{Value: fieldValue(t, holder, "plain"), Name: "plain", Field: f, Decl: holder.T})
	same(t, "API.md P9 field target", slices.Sorted(maps.Keys(res.Headings)), `["[\"none\"]","[other]"]`)
}

// fieldNamed is the field name of the record type t.
func fieldNamed(t types.Type, name string) *types.Field {
	for _, f := range t.Base().(*types.RecordType).Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}
