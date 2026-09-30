package edit_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// itemScopeSrc has collections whose field gives its items a unit, an int encoding or bits.
const itemScopeSrc = `/// P.
package d

/// A mark.
enum Mark @codes(UInt8) { a = 1, b = 2, c = 4 }

/// A tag.
record Tag {
  /// Waits.
  waits: {String: Duration} = {} @json(unit: s)
  /// Waits by group.
  deep: {String: {String: Duration}} = {} @json(unit: s)
  /// Lists.
  ls: [Duration] = [] @json(unit: s)
  /// Flags.
  fl: [Bool] = [] @json(int)
  /// Marks.
  mk: [Mark] = [] @json(bits)
}

/// Loaded.
let tag: Tag = load("t.json")
`

func itemScopeJSON(waits, deep, ls, fl, mk string) string {
	return "{\n  \"waits\": {\n    \"a\": " + waits + "\n  },\n  \"deep\": {\n    \"a\": {\n" + deep + "\n    }\n  },\n" +
		"  \"ls\": [\n    " + ls + "\n  ],\n  \"fl\": [\n    " + fl + "\n  ],\n  \"mk\": " + mk + "\n}\n"
}

func itemScopeFS() mapFS {
	return mapFS{
		"law/project.canon": file(projectCanon), "law/d/d.canon": file(itemScopeSrc),
		"law/d/t.json": file(itemScopeJSON("5", "      \"b\": 5", "1", "0", "3")),
	}
}

// WIRE.md 4.1, 5.1-5.3, API.md M1, M8, E22 (log-2026-09-29 M4 B11): an element or entry below a
// field is written in that field's unit and int encoding, an element of a bits list through the
// list's one number, whatever the depth; each Undo restores the file byte for byte.
func TestItemWireScope(t *testing.T) {
	for _, c := range []struct {
		name, want string
		op         edit.Operation
	}{
		{"set, unit entry", itemScopeJSON("7", "      \"b\": 5", "1", "0", "3"),
			edit.Operation{Kind: edit.OpSet, Path: `tag.waits["a"]`, Value: edit.Source("7s")}},
		{"set, deeper unit entry", itemScopeJSON("5", "      \"b\": 7", "1", "0", "3"),
			edit.Operation{Kind: edit.OpSet, Path: `tag.deep["a"]["b"]`, Value: edit.Source("7s")}},
		{"add entry, deeper unit", itemScopeJSON("5", "      \"b\": 5,\n      \"c\": 7", "1", "0", "3"),
			edit.Operation{Kind: edit.OpAddEntry, Path: `tag.deep["a"]`, Key: edit.Key("c"), Value: edit.Source("7s")}},
		{"set, unit element", itemScopeJSON("5", "      \"b\": 5", "7", "0", "3"),
			edit.Operation{Kind: edit.OpSet, Path: "tag.ls[0]", Value: edit.Source("7s")}},
		{"set, int element", itemScopeJSON("5", "      \"b\": 5", "1", "1", "3"),
			edit.Operation{Kind: edit.OpSet, Path: "tag.fl[0]", Value: edit.Bool(true)}},
		{"set, bits element", itemScopeJSON("5", "      \"b\": 5", "1", "0", "6"),
			edit.Operation{Kind: edit.OpSet, Path: "tag.mk[0]", Value: edit.Member("c")}},
	} {
		s := open(t, itemScopeFS(), nil, "", "d")
		ops := []edit.Operation{c.op}
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := string(written(itemScopeFS(), plan)["law/d/t.json"].Data); got != c.want {
			t.Errorf("%s: t.json is\n%s\nwant\n%s", c.name, got, c.want)
		}
		roundTrip(t, c.name, itemScopeFS(), ops)
	}
}
