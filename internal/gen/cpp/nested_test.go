package cppgen_test

const (
	nestValueFile = "value.json"
	nestItemsOld  = `"items": [{"a": 1, "b": 1}, {"a": 2, "b": 2}]`
	// nestGoodDump is strict_main's dump of testdata/main/nest/good/value.json's Float32 bits: midpoint tokens just above or below, a tie, an integer token and the largest decimal under the overflow boundary.
	nestGoodDump = "f32s=3fc00000,bf800001,5d800001,7f7fffff a.b=3f800001 a/b=3f800000 shape.r=40000001 $scale=00000001,3f000000"
	// nestMaxOld is the f32s element nestf32list pushes over the Float32 overflow boundary.
	nestMaxOld = "3.4028235677973366e38]"
)

// nestCases are log-2026-09-24 A1's duplicate-id rule over every keyable() type (TYPES.md §9.1) but String, plus the decode-vs-duplicate order.
func nestCases() []strictCase {
	return []strictCase{
		{"nestgood", "log-2026-09-24 A1, A1 C++ Float32 (every Float32 place; \"a.b\" and a.b read their own tokens)", nestValueFile, `"$schema"`, `"$schema"`, "", nestGoodDump, nestFile},
		{"nestslot", "TYPES.md §9.1, log-2026-09-24 A1 (plain enum key)", nestValueFile,
			`"slots": [{"size": "small"}, {"size": "medium"}]`, `"slots": [{"size": "small"}, {"size": "medium"}, {"size": "small"}]`,
			`value.slots[2].size: duplicate id "small" (first at value.slots[0])`, "", nestFile},
		{"nestbin", "TYPES.md §9.1, log-2026-09-24 A1 (@json(codes) enum key)", nestValueFile,
			`"bins": [{"grade": 1}, {"grade": 2}]`, `"bins": [{"grade": 1}, {"grade": 2}, {"grade": 1}]`,
			`value.bins[2].grade: duplicate id 1 (first at value.bins[0])`, "", nestFile},
		{"nestticket", "TYPES.md §9.1, log-2026-09-24 A1 (ref key)", nestValueFile,
			`"tickets": [{"label": "a"}, {"label": "b"}]`, `"tickets": [{"label": "a"}, {"label": "b"}, {"label": "a"}]`,
			`value.tickets[2].label: duplicate id "a" (first at value.tickets[0])`, "", nestFile},
		{"nestitem", "TYPES.md §9.1, log-2026-09-24 A1 (Int key)", nestValueFile,
			nestItemsOld, `"items": [{"a": 1, "b": 1}, {"a": 2, "b": 2}, {"a": 1, "b": 9}]`,
			`value.items[2].a: duplicate id 1 (first at value.items[0])`, "", nestFile},
		{"nesttwodups", "log-2026-09-24 A1 (the first duplicate in row order, not in key order)", nestValueFile,
			nestItemsOld, `"items": [{"a": 2, "b": 1}, {"a": 1, "b": 2}, {"a": 2, "b": 3}, {"a": 1, "b": 4}]`,
			`value.items[2].a: duplicate id 2 (first at value.items[0])`, "", nestFile},
		{"nestspecial", "log-2026-09-24 A1 §b (key field's full wire path)", nestValueFile,
			`"specials": [{"legacy": {"a": 1}}, {"legacy": {"a": 2}}]`,
			`"specials": [{"legacy": {"a": 1}}, {"legacy": {"a": 2}}, {"legacy": {"a": 1}}]`,
			`value.specials[2].legacy.a: duplicate id 1 (first at value.specials[0])`, "", nestFile},
		{"nestlatererror", "log-2026-09-24 A1 §a (a later row's decode error wins)", nestValueFile,
			nestItemsOld, `"items": [{"a": 1, "b": 1}, {"a": 2, "b": 2}, {"a": 1, "b": 3}, {"a": 4, "b": "nope"}]`,
			`value.items[3].b: expected an integer`, "", nestFile},
		{"nestrowbad", "log-2026-09-24 A1 §a (the duplicate row's own decode error wins)", nestValueFile,
			nestItemsOld, `"items": [{"a": 1, "b": 1}, {"a": 2, "b": 2}, {"a": 1, "b": "nope"}]`,
			`value.items[2].b: expected an integer`, "", nestFile},
	}
}
