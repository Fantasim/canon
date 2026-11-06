package canon_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// slotKeySrc is p: each fn assigns into a slot of a var copied from a keyed list or a table, the
// first time (a copy) or the second (the collection the frame owns, updated in place).
const slotKeySrc = "package p\n\nrecord U {\n  n: String\n  w: Int = 0\n}\n\nrecord H {\n  uu: [U] keyed by n\n}\n\n" +
	"record R {\n  k: Int\n  w: Int = 0\n}\n\nrecord Item {\n  size: Int = 1\n}\n\nrecord V {\n  n: String\n  m: String = \"\"\n}\n\n" +
	"let byName: [V] keyed by n = [{ n: \"x\", m: \"p\" }]\n\nlet byMark: [V] keyed by m = [{ n: \"x\", m: \"q\" }]\n\nlet tv: table V = { z { n: \"x\" } }\n\n" +
	"let h: H = { uu: [{ n: \"x\" }, { n: \"y\" }] }\n\nlet rows: [R] keyed by k = [{ k: 1 }, { k: 2 }]\n\n" +
	"let items: table Item = { sword {}, shield { size: 2 } }\n\nlet other: table Item = { sword { size: 5 } }\n\n" +
	"fn movedU(hh: H) -> [U] keyed by n {\n  var xs: [U] keyed by n = hh.uu\n  xs[\"x\"] = hh.uu.y\n  return xs\n}\n\n" +
	"fn litU(hh: H) -> [U] keyed by n {\n  var xs: [U] keyed by n = hh.uu\n  xs[\"x\"] = { n: \"z\" }\n  return xs\n}\n\n" +
	"fn twiceU(hh: H) -> [U] keyed by n {\n  var xs: [U] keyed by n = hh.uu\n  xs[\"y\"] = { n: \"y\", w: 1 }\n  xs[\"x\"] = hh.uu.y\n  return xs\n}\n\n" +
	"fn nestedU(hh: H) -> {String: [U] keyed by n} {\n  var m: {String: [U] keyed by n} = { \"a\": hh.uu }\n  m[\"a\"][\"x\"] = { n: \"y\" }\n  return m\n}\n\n" +
	"fn keptU(hh: H) -> [U] keyed by n {\n  var xs: [U] keyed by n = hh.uu\n  xs[\"x\"] = { n: \"x\", w: 3 }\n  return xs\n}\n\n" +
	"fn intR() -> [R] keyed by k {\n  var xs: [R] keyed by k = rows\n  xs[1] = { k: 2, w: 4 }\n  return xs\n}\n\n" +
	"fn movedT() -> table Item {\n  var ts: table Item = items\n  ts[\"sword\"] = items.shield\n  return ts\n}\n\n" +
	"fn ownT() -> table Item {\n  var ts: table Item = items\n  ts[\"sword\"] = other.sword\n  return ts\n}\n\n" +
	"fn freshT() -> table Item {\n  var ts: table Item = items\n  ts[\"sword\"] = { size: 3 }\n  return ts\n}\n\n" +
	"fn twiceT() -> table Item {\n  var ts: table Item = items\n  ts[\"shield\"] = { size: 4 }\n  ts[\"sword\"] = { size: 3 }\n  return ts\n}\n\n" +
	"fn fromTable() -> [V] keyed by n {\n  var xs: [V] keyed by n = byName\n  xs[\"x\"] = tv.z\n  return xs\n}\n\n" +
	"fn fromOther() -> [V] keyed by n {\n  var xs: [V] keyed by n = byName\n  xs[\"x\"] = byMark.q\n  return xs\n}\n\n" +
	"let a: [U] keyed by n = movedU(h)\n\nlet b: [U] keyed by n = litU(h)\n\nlet c: [U] keyed by n = twiceU(h)\n\n" +
	"let d: {String: [U] keyed by n} = nestedU(h)\n\nlet e: [R] keyed by k = intR()\n\nlet f: table Item = movedT()\n\n" +
	"let g: [V] keyed by n = fromTable()\n\nlet i: [V] keyed by n = fromOther()\n\n" +
	"let pk: [Bool] = [keptU(h).x.w == 3, keptU(h).y == h.uu.y, keptU(h).x == h.uu.x]\n\n" +
	"let po: [Bool] = [ownT().sword == other.sword, ownT().sword == items.sword, items.contains(ownT().sword), other.contains(ownT().sword)]\n\n" +
	"let pf: [Bool] = [freshT().sword == items.sword, items.contains(freshT().sword), freshT().sword.id == \"sword\", freshT().sword.size == 3, " +
	"twiceT().sword == items.sword, twiceT().shield == items.shield, twiceT().sword.id == \"sword\", twiceT().shield.size == 4]\n"

// slotKeyCase is a let of slotKeySrc whose fn fills a slot with another key: the slot's key and the
// element's, its identity key first, then its key field.
type slotKeyCase struct{ path, key, got string }

var slotKeyCases = []slotKeyCase{
	{"a", "x", "y"}, {"b", "x", "z"}, {"c", "x", "y"}, {"d", "x", "y"},
	{"e", "1", "2"}, {"f", "sword", "shield"}, {"g", "x", "z"}, {"i", "x", "q"},
}

// namesKeys reports a message whose words name c's element key before its slot key.
func namesKeys(msg string, c slotKeyCase) bool {
	words := strings.Fields(strings.ReplaceAll(msg, ",", " "))
	g, k := slices.Index(words, c.got), slices.Index(words, c.key)
	return g >= 0 && k > g
}

// EVALUATION.md §4.1, §7.1; DECISIONS 512: another key, by identity or key field, is a hard E4004.
func TestAssignedSlotKeepsItsKey(t *testing.T) {
	res, err := openLayered(t, slotKeySrc, "", false).Check(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	code := string(diag.E4004.Def().Code)
	var errs []canon.Finding
	for _, f := range res.Findings {
		if f.Severity == canon.SeverityError {
			errs = append(errs, f)
		}
	}
	if len(errs) != len(slotKeyCases) {
		t.Fatalf("EVALUATION.md §4.1; DECISIONS 512: %d errors %v, want %d", len(errs), errs, len(slotKeyCases))
	}
	for i, c := range slotKeyCases {
		if f := errs[i]; f.Code != code || f.Path != c.path || !namesKeys(f.Message, c) {
			t.Errorf("EVALUATION.md §4.1; DECISIONS 512: %s %s %q, want %s at %s, key %s for %s", f.Code, f.Path, f.Message, code, c.path, c.got, c.key)
		}
	}
}

// EVALUATION.md §4.1; TYPES.md §6.3; DECISIONS 510, 512: a table's slot never passes on its identity.
func TestAssignedTableSlotIdentity(t *testing.T) {
	p := openLayered(t, slotKeySrc, "", false)
	checkValues(t, p, "EVALUATION.md §4.1; DECISIONS 512", map[string]string{
		"p:pk": "[true, true, false]",
		"p:po": "[true, false, false, true]",
		"p:pf": "[false, false, true, true, false, false, true, true]",
	})
}
