package ir_test

import (
	"strings"
	"testing"
)

const (
	cppRunIDKeys = "testdata/e2e/cpprun/own_id_keys.txtar"
	// idKeysData and idKeysHolder are the data files dt's and qr's json emits write, under the build's directory.
	idKeysData   = "dt/out/slot.json"
	idKeysHolder = "qr/out/holder.json"
	// idKeysSlot is a Slot whose every key names an item, and idKeysShown how the probe prints it.
	idKeysSlot = `{"item": "sword", "maybe": "shield", "many": ["sword"], "rows": [["sword"], []], "byName": {"a": "shield"}, "counts": {"sword": 2}, "optional": [null, "shield"], "fallback": "shield", ` +
		`"u": "sword", "ou": "x", "ul": ["x", "shield"], "entries": [{"item": "sword", "n": 1}, {"item": "shield", "n": 0}], "vs": [{"k": "plain", "v": 1}, {"k": "pick", "v": "shield"}], "$best": "sword"}`
	idKeysShown = "item=sword maybe=shield many=[sword] rows=[[sword],[]] byName={a=shield} counts={sword=2} optional=[none,shield] fallback=shield " +
		"u=sword ou=x ul=[x,shield] entries=[sword:1,shield:0] vs=[plain:1,pick:shield] best=sword"
	// idKeysBag is the bag of the holder, a bk.Bag baked by its owner, and idKeysBagShown how the probe prints it.
	idKeysBag      = `{"rows": [{"item": "shield", "n": 2}]}`
	idKeysBagShown = " bag=[shield:2]"
	// idKeysBaked is the slot bb bakes through dt's make hook, ids as enum members.
	idKeysBaked = "item=shield maybe=sword many=[sword] rows=[] byName={} counts={} optional=[] fallback=shield u=shield ou=none ul=[x] entries=[] vs=[] best=shield"
	// idKeysPick is a Pick of a types file, and idKeysPickShown how the probe prints it.
	idKeysPick      = `{"item": "shield", "many": ["sword", "shield"]}`
	idKeysPickShown = "item=shield many=[sword,shield]"
	idKeysDup       = `value.entries[1].item: duplicate id "sword" (first at value.entries[0])`
)

// idKeyCase is idKeysSlot with one position changed, from → to, and the error the probe prints for it.
type idKeyCase struct{ name, from, to, want string }

// idKeyCases: every position of a plain ref and a union over one that a loader reads in a class of its own package (CODEGEN.md §5.8, DECISIONS 312, 387), and a keyed list's duplicate key, with Go's texts.
var idKeyCases = []idKeyCase{
	{"item.json", `"item": "sword"`, `"item": "zz"`, "value.item: unknown value zz"},
	{"maybe.json", `"maybe": "shield"`, `"maybe": "zz"`, "value.maybe: unknown value zz"},
	{"many.json", `"many": ["sword"]`, `"many": ["zz"]`, "value.many[0]: unknown value zz"},
	{"rows.json", `"rows": [["sword"], []]`, `"rows": [[], ["zz"]]`, "value.rows[1][0]: unknown value zz"},
	{"byname.json", `"byName": {"a": "shield"}`, `"byName": {"a": "zz"}`, "value.byName.a: unknown value zz"},
	{"counts.json", `"counts": {"sword": 2}`, `"counts": {"zz": 2}`, "value.counts.zz: unknown value zz"},
	{"optional.json", `"optional": [null, "shield"]`, `"optional": [null, "zz"]`, "value.optional[1]: unknown value zz"},
	{"fallback.json", `"fallback": "shield"`, `"fallback": "zz"`, "value.fallback: unknown value zz"},
	{"u.json", `"u": "sword"`, `"u": "zz"`, "value.u: unknown value zz"},
	{"ou.json", `"ou": "x"`, `"ou": "zz"`, "value.ou: unknown value zz"},
	{"ul.json", `"ul": ["x", "shield"]`, `"ul": ["x", "zz"]`, "value.ul[1]: unknown value zz"},
	{"entry.json", `{"item": "shield", "n": 0}`, `{"item": "zz", "n": 0}`, "value.entries[1].item: unknown value zz"},
	{"dup.json", `{"item": "shield", "n": 0}`, `{"item": "sword", "n": 0}`, idKeysDup},
	{"vs.json", `{"k": "pick", "v": "shield"}`, `{"k": "pick", "v": "zz"}`, "value.vs[1].v: unknown value zz"},
	{"best.json", `"$best": "sword"`, `"$best": "zz"`, "value.$best: unknown value zz"},
}

// idKeyHolderCases: qr reads dt's Slot and bk's baked Bag with their owners' layouts, a key naming no item is `no entry`, a duplicate key of the bag's keyed list is Go's text (CODEGEN.md §2.8).
var idKeyHolderCases = []idKeyCase{
	{"h_item.json", `"item": "sword"`, `"item": "zz"`, "value.slot.item: no entry zz"},
	{"h_vs.json", `{"k": "pick", "v": "shield"}`, `{"k": "pick", "v": "zz"}, {"k": "plain", "v": 2}`, "value.slot.vs[1].v: no entry zz"},
	{"h_bag.json", `"item": "shield", "n": 2`, `"item": "zz", "n": 2`, "value.bag.rows[0].item: no entry zz"},
	{"h_dup.json", idKeysBag, `{"rows": [{"item": "shield", "n": 2}, {"item": "shield", "n": 3}]}`, `value.bag.rows[1].item: duplicate id "shield" (first at value.bag.rows[0])`},
}

// TestCppOwnIDKeysRun is CODEGEN.md §5.8 (key type) and DECISIONS 312, 387 compiled and run: a class of this package holds a ref into another package's baked table as the owner's id enum, and its data loader and types decoder check the key through the enum, as Go's `lib.ParseItemID` does, with the text `unknown value <key>`; another package's reader of such a class says `no entry <key>`.
func TestCppOwnIDKeysRun(t *testing.T) {
	t.Parallel()
	cppRunCheck(t, cppRunIDKeys, func(dir string) []cppRunFile {
		head := `{"$schema": "` + cppRunSchema(t, dir, idKeysData) + `", "value": `
		heldHead := `{"$schema": "` + cppRunSchema(t, dir, idKeysHolder) + `", "value": {"slot": `
		files := []cppRunFile{
			{name: "data:../" + idKeysData, want: idKeysShown},
			{name: "holder:../" + idKeysHolder, want: idKeysShown + idKeysBagShown},
			{name: "baked", want: idKeysBaked},
			{name: "pick.json", body: idKeysPick, want: "decode " + idKeysPickShown + " | parse " + idKeysPickShown},
			{name: "t_item.json", body: `{"item": "zz"}`, want: "decode error Pick: $.item: unknown value zz | parse error Pick: $.item: unknown value zz"},
			{name: "t_many.json", body: `{"item": "sword", "many": ["sword", "zz"]}`, want: "decode error Pick: $.many[1]: unknown value zz | parse error Pick: $.many[1]: unknown value zz"},
			{name: "wrap.json", body: `{}`, quiet: true},
			{name: "wrap:wrap.json", want: "decode p=sword | parse p=sword"},
			{name: "wrap_bad.json", body: `{"p": {"item": "zz"}}`, quiet: true},
			{name: "wrap:wrap_bad.json", want: "decode error Wrap: $.p.item: no entry zz | parse error Wrap: $.p.item: no entry zz"},
		}
		for _, c := range idKeyCases {
			slot := strings.Replace(idKeysSlot, c.from, c.to, 1)
			files = append(files, cppRunFile{name: c.name, body: head + slot + "}", quiet: true},
				cppRunFile{name: "data:" + c.name, want: "error " + c.name + ": " + c.want})
		}
		for _, c := range idKeyHolderCases {
			slot, bag := idKeysSlot, idKeysBag
			if strings.HasPrefix(c.name, "h_bag") || strings.HasPrefix(c.name, "h_dup") {
				bag = strings.Replace(bag, c.from, c.to, 1)
			} else {
				slot = strings.Replace(slot, c.from, c.to, 1)
			}
			files = append(files, cppRunFile{name: c.name, body: heldHead + slot + `, "bag": ` + bag + "}}", quiet: true},
				cppRunFile{name: "holder:" + c.name, want: "error " + c.name + ": " + c.want})
		}
		return files
	})
}
