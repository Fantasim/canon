package tsgen_test

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
)

// rangeBase is a Probe every refinement accepts (testdata/ts/range_types.txtar).
const rangeBase = `{"n":5,"u8":7,"i8":7,"ex":4,"f":0.5,"f32":0.0625,"d":2000,"s":"ab","tags":[1,2],"m":{"a":1},"mi":{"50":"x"},"me":{"a":1},"mr":{"a":{"v":1}},"opts":[null,3],"lit":"auto","sub":{"v":1},"shape":{"kind":"circle","r":1},"dsel":"b","dep":3,"rows":[{"id":1},{"id":2}],"flags":3,"n0":1,"w0":1,"n1":2,"w1":2,"ds":1,"p":{"q":1}}`

// rangeCase is rangeBase with from replaced by to, and the line the driver prints for it.
type rangeCase struct{ name, from, to, want string }

const (
	rangeOK      = "ok"
	valueIn      = "expected a value in "
	countIn      = "expected an element count in "
	bytesIn      = "expected a byte length in "
	rangeWidth   = "integer out of range"
	rangeKey     = "expected a canonical decimal integer key"
	rangeProbe   = "Probe"
	rangeBounds  = "Bounds"
	rangeItemsID = "items"
)

// rangeErr is the driver's line for a refusal at path of the types-mode decoder of root.
func rangeErr(root, path, message string) string {
	return "error " + root + ": $." + path + ": " + message
}

// probeErr is rangeErr of Probe.
func probeErr(path, message string) string { return rangeErr(rangeProbe, path, message) }

// numberCases are the integer, float, Float32 and Duration positions of rangeCases.
func numberCases() []rangeCase {
	return []rangeCase{
		{"int-lo-in", `"n":5`, `"n":1`, rangeOK}, {"int-hi-in", `"n":5`, `"n":120`, rangeOK},
		{"int-lo-out", `"n":5`, `"n":0`, probeErr("n", valueIn+"1..=120")}, {"int-hi-out", `"n":5`, `"n":121`, probeErr("n", valueIn+"1..=120")},
		{"width-first", `"i8":7`, `"i8":200`, probeErr("i8", rangeWidth)},
		{"fraction-first", `"n":5`, `"n":0.5`, probeErr("n", "expected an integer")},
		{"u8-top", `"u8":7`, `"u8":255`, rangeOK}, {"u8-width", `"u8":7`, `"u8":256`, probeErr("u8", rangeWidth)},
		{"i8-lo-out", `"i8":7`, `"i8":-1`, probeErr("i8", valueIn+"0..=300")}, {"i8-top", `"i8":7`, `"i8":127`, rangeOK},
		{"end-in", `"ex":4`, `"ex":-9`, rangeOK}, {"end-out", `"ex":4`, `"ex":5`, probeErr("ex", valueIn+"..5")},
		{"float-lo-in", `"f":0.5`, `"f":0`, rangeOK}, {"float-hi-out", `"f":0.5`, `"f":1`, probeErr("f", valueIn+"0..1")},
		{"float-lo-out", `"f":0.5`, `"f":-0.25`, probeErr("f", valueIn+"0..1")},
		{"f32-tenth", `"f32":0.0625`, `"f32":0.1`, probeErr("f32", valueIn+"..=0.1")}, {"f32-below", `"f32":0.0625`, `"f32":0.09999`, rangeOK},
		{"dur-lo-in", `"d":2000`, `"d":1000`, rangeOK}, {"dur-lo-out", `"d":2000`, `"d":999`, probeErr("d", valueIn+"1s..=1h")},
		{"dur-hi-in", `"d":2000`, `"d":3600000`, rangeOK}, {"dur-hi-out", `"d":2000`, `"d":3600001`, probeErr("d", valueIn+"1s..=1h")},
	}
}

// collectionCases are the strings, lists, maps, optionals and unions of rangeCases.
func collectionCases() []rangeCase {
	return []rangeCase{
		{"str-two-byte", `"s":"ab"`, `"s":"éé"`, rangeOK}, {"str-two-byte-out", `"s":"ab"`, `"s":"ééé"`, probeErr("s", bytesIn+"1..=4")},
		{"str-four-byte", `"s":"ab"`, `"s":"😀"`, rangeOK}, {"str-four-byte-out", `"s":"ab"`, `"s":"😀a"`, probeErr("s", bytesIn+"1..=4")},
		{"str-empty", `"s":"ab"`, `"s":""`, probeErr("s", bytesIn+"1..=4")},
		{"list-count-in", `"tags":[1,2]`, `"tags":[1,2,3]`, rangeOK}, {"list-count-out", `"tags":[1,2]`, `"tags":[1,2,3,4]`, probeErr("tags", countIn+"..=3")},
		{"list-elem-out", `"tags":[1,2]`, `"tags":[6]`, probeErr("tags[0]", valueIn+"0..=5")},
		{"list-elem-first", `"tags":[1,2]`, `"tags":[1,6,1,1]`, probeErr("tags[1]", valueIn+"0..=5")},
		{"map-count-out", `"m":{"a":1}`, `"m":{"a":1,"b":2,"c":3}`, probeErr("m", countIn+"..=2")},
		{"map-count-in", `"m":{"a":1}`, `"m":{"a":1,"b":2}`, rangeOK},
		{"map-key-out", `"m":{"a":1}`, `"m":{"abcd":1}`, probeErr("m.abcd", bytesIn+"1..=3")},
		{"map-value-out", `"m":{"a":1}`, `"m":{"a":10}`, probeErr("m.a", valueIn+"0..=9")},
		{"map-int-key-out", `"mi":{"50":"x"}`, `"mi":{"101":"x"}`, probeErr("mi.101", valueIn+"..=100")},
		{"map-int-key-width", `"mi":{"50":"x"}`, `"mi":{"300":"x"}`, probeErr("mi.300", rangeWidth)},
		{"map-int-key-negative", `"mi":{"50":"x"}`, `"mi":{"-1":"x"}`, probeErr("mi.-1", rangeWidth)},
		{"opt-elem-out", `"opts":[null,3]`, `"opts":[9]`, probeErr("opts[0]", valueIn+"0..=5")},
		{"opt-null", `"n":5,`, `"n":5,"opt":null,`, rangeOK}, {"opt-out", `"n":5,`, `"n":5,"opt":9,`, probeErr("opt", valueIn+"0..=5")},
		{"opt-in", `"n":5,`, `"n":5,"opt":5,`, rangeOK},
		{"marker-none", `"n":5,`, `"n":5,"mk":-1,`, rangeOK}, {"marker-out", `"n":5,`, `"n":5,"mk":6,`, probeErr("mk", valueIn+"0..=5")},
		{"union-literal", `"lit":"auto"`, `"lit":"auto"`, rangeOK}, {"union-in", `"lit":"auto"`, `"lit":"abcdefg"`, rangeOK},
		{"union-out", `"lit":"auto"`, `"lit":"abc"`, probeErr("lit", bytesIn+"6..=8")},
		// CODEGEN.md §5.13, log-2026-10-31: a member under key k is `<parent>.<k>`, an empty key included; the three targets agree on `$.m.`, `$.me.` and `$.mr..v`.
		{"map-key-empty", `"m":{"a":1}`, `"m":{"":1}`, probeErr("m.", bytesIn+"1..=3")},
		{"map-value-empty-key", `"me":{"a":1}`, `"me":{"":10}`, probeErr("me.", valueIn+"0..=9")},
		{"map-record-empty-key", `"mr":{"a":{"v":1}}`, `"mr":{"":{"v":4}}`, probeErr("mr..v", valueIn+"0..=3")},
	}
}

// structCases are the records, cases, defaults, pairs and bits of rangeCases. A pairs list never holds more than its upper bound (the slots are that bound), so only its lower bound is reachable.
func structCases() []rangeCase {
	return []rangeCase{
		{"default-unchecked", `"n":5`, `"n":5`, rangeOK}, {"default-given", `"n":5,`, `"n":5,"dfl":9,`, probeErr("dfl", valueIn+"1..=5")},
		{"record-field", `"sub":{"v":1}`, `"sub":{"v":4}`, probeErr("sub.v", valueIn+"0..=3")},
		{"case-field", `"r":1`, `"r":3`, probeErr("shape.r", valueIn+"0..=2")},
		{"pairs-value", `"w1":2`, `"w1":9`, probeErr("w1", valueIn+"0..=3")},
		{"bits-count", `"flags":3`, `"flags":7`, probeErr("flags", countIn+"1..=2")}, {"bits-in", `"flags":3`, `"flags":6`, rangeOK},
		{"bits-lo", `"flags":3`, `"flags":0`, probeErr("flags", countIn+"1..=2")}, {"bits-lo-in", `"flags":3`, `"flags":4`, rangeOK},
		{"pairs-lo", `,"n0":1,"w0":1,"n1":2,"w1":2`, ``, probeErr("slots", countIn+"1..=2")}, {"pairs-lo-in", `,"n1":2,"w1":2`, ``, rangeOK},
	}
}

// positionCases are the dependent branch and the keyed list of CODEGEN.md §5.15, WIRE.md §5.7: elements first, then duplicates, then the count.
func positionCases() []rangeCase {
	return []rangeCase{
		{"dep-str-in", `"dsel":"b","dep":3`, `"dsel":"a","dep":"abc"`, rangeOK}, {"dep-str-lo", `"dsel":"b","dep":3`, `"dsel":"a","dep":"a"`, probeErr("dep", bytesIn+"2..=3")},
		{"dep-str-hi", `"dsel":"b","dep":3`, `"dsel":"a","dep":"abcd"`, probeErr("dep", bytesIn+"2..=3")},
		{"dep-int-lo", `"dep":3`, `"dep":-1`, probeErr("dep", valueIn+"0..=5")}, {"dep-int-hi", `"dep":3`, `"dep":6`, probeErr("dep", valueIn+"0..=5")},
		{"dep-int-in", `"dep":3`, `"dep":5`, rangeOK},
		{"keyed-count-out", `{"id":2}]`, `{"id":2},{"id":3}]`, probeErr("rows", countIn+"..=2")},
		{"keyed-elem-out", `{"id":1}`, `{"id":10}`, probeErr("rows[0].id", valueIn+"0..=9")},
		{"keyed-elem-first", `{"id":1},{"id":2}`, `{"id":10},{"id":2},{"id":3}`, probeErr("rows[0].id", valueIn+"0..=9")},
		{"keyed-dup-first", `{"id":1},{"id":2}`, `{"id":1},{"id":1},{"id":3}`, probeErr("rows[1].id", "duplicate id 1 (first at $.rows[0])")},
		{"keyed-elem-before-dup", `{"id":1},{"id":2}`, `{"id":1},{"id":1},{"id":10}`, probeErr("rows[2].id", valueIn+"0..=9")},
		{"keyed-reordered", `{"id":1},{"id":2}`, `{"id":2},{"id":1}`, rangeOK},
	}
}

// unitCases are a Duration read in seconds, judged in milliseconds after its unit (CODEGEN.md §5.15).
func unitCases() []rangeCase {
	return []rangeCase{
		{"unit-lo-in", `"ds":1`, `"ds":1`, rangeOK}, {"unit-hi-in", `"ds":1`, `"ds":2`, rangeOK},
		{"unit-hi-out", `"ds":1`, `"ds":3`, probeErr("ds", valueIn+"1s..=2s")}, {"unit-lo-out", `"ds":1`, `"ds":0`, probeErr("ds", valueIn+"1s..=2s")},
		{"unit-fraction", `"ds":1`, `"ds":1.5`, rangeOK}, {"unit-fraction-out", `"ds":1`, `"ds":2.5`, probeErr("ds", valueIn+"1s..=2s")},
	}
}

// textCases are the @text result of the package: parseCapsFile of a list of 0..=5 integers, at most two (CODEGEN.md §5.15).
var textCases = []struct{ name, text, want string }{
	{"caps-ok", `[0,5]`, "ok"}, {"caps-empty", `[]`, "ok"},
	{"caps-count", `[1,2,3]`, "error CapsFile: $: expected an element count in ..=2"},
	{"caps-elem", `[1,6]`, "error CapsFile: $[1]: expected a value in 0..=5"},
	{"caps-lo", `[-1]`, "error CapsFile: $[0]: expected a value in 0..=5"},
	{"caps-elem-first", `[9,1,1]`, "error CapsFile: $[0]: expected a value in 0..=5"},
}

// rangeCases are CODEGEN.md §5.15 at every position: both sides of each bound, the order of failures, what is never checked.
func rangeCases() []rangeCase {
	cases := []rangeCase{{"base", "", "", rangeOK}}
	return slices.Concat(cases, numberCases(), collectionCases(), structCases(), positionCases(), unitCases(), wireCases(), nullCases())
}

// keysBase is a Keys every key is unique in (testdata/ts/range_keys.txtar).
const keysBase = `{"medals":[{"r":1},{"r":2}],"entries":[{"m":1},{"m":2}],"slots":[{"side":"left"},{"side":"RIGHT"}],"picks":[{"n":1},{"n":2}],"bigs":[{"b":9007199254740993},{"b":2}],"groups":[{"rows":[{"id":1},{"id":2}]}],"byName":{"x":[{"id":1}]}}`

func keysErr(path, message string) string { return rangeErr("Keys", path, message) }

// keysCases are the repeated keys of every key type: a key as the data wire writes it (WIRE.md §7.3) and the earlier row's path without its name (CODEGEN.md §5.15).
func keysCases() []rangeCase {
	return []rangeCase{
		{"base", "", "", rangeOK},
		{"code-dup", `{"r":1},{"r":2}`, `{"r":1},{"r":1}`, keysErr("medals[1].r", "duplicate id 1 (first at $.medals[0])")},
		{"ref-code-dup", `{"m":1},{"m":2}`, `{"m":2},{"m":2}`, keysErr("entries[1].m", "duplicate id 2 (first at $.entries[0])")},
		{"wire-dup", `{"side":"left"},{"side":"RIGHT"}`, `{"side":"RIGHT"},{"side":"RIGHT"}`, keysErr("slots[1].side", `duplicate id "RIGHT" (first at $.slots[0])`)},
		{"ref-int-dup", `{"n":1},{"n":2}`, `{"n":2},{"n":2}`, keysErr("picks[1].n", "duplicate id 2 (first at $.picks[0])")},
		{"bigint-dup", `{"b":9007199254740993},{"b":2}`, `{"b":9007199254740993},{"b":9007199254740993}`, keysErr("bigs[1].b", "duplicate id 9007199254740993 (first at $.bigs[0])")},
		{"nested-dup", `{"rows":[{"id":1},{"id":2}]}`, `{"rows":[{"id":1},{"id":1}]}`, keysErr("groups[0].rows[1].id", "duplicate id 1 (first at $.groups[0].rows[0])")},
		{"map-key-colon-dup", `"byName":{"x":[{"id":1}]}`, `"byName":{"a: b":[{"id":1},{"id":1}]}`, keysErr("byName.a: b[1].id", "duplicate id 1 (first at $.byName.a: b[0])")},
		{"reordered", `{"r":1},{"r":2}`, `{"r":2},{"r":1}`, rangeOK},
		{"code-map-ok", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byRank":{"1":1},"byMedal":{"2":1}`, rangeOK},
		{"code-map-leading-zero", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byRank":{"01":1}`, keysErr("byRank.01", rangeKey)},
		{"code-map-minus-zero", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byRank":{"-0":1}`, keysErr("byRank.-0", rangeKey)},
		{"code-map-plus", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byRank":{"+1":1}`, keysErr("byRank.+1", rangeKey)},
		{"code-map-empty", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byRank":{"":1}`, keysErr("byRank.", rangeKey)},
		{"code-map-space", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byRank":{" 1":1}`, keysErr("byRank. 1", rangeKey)},
		{"code-map-hex", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byRank":{"0x1":1}`, keysErr("byRank.0x1", rangeKey)},
		{"code-map-exponent", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byRank":{"1e0":1}`, keysErr("byRank.1e0", rangeKey)},
		{"code-map-unsafe", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byRank":{"9007199254740993":1}`, keysErr("byRank.9007199254740993", rangeSafe())},
		{"ref-code-map-leading-zero", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byMedal":{"01":1}`, keysErr("byMedal.01", rangeKey)},
		{"ref-code-map-minus-zero", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byMedal":{"-0":1}`, keysErr("byMedal.-0", rangeKey)},
		{"ref-code-map-plus", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byMedal":{"+1":1}`, keysErr("byMedal.+1", rangeKey)},
		{"ref-code-map-empty", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byMedal":{"":1}`, keysErr("byMedal.", rangeKey)},
		// WIRE.md §5.3, CODEGEN.md §5.13: a code no member has reads `unknown value <n>` in every target, as a field value and as a map key.
		{"code-field-unknown", `{"r":1},{"r":2}`, `{"r":3},{"r":2}`, keysErr("medals[0].r", "unknown value 3")},
		{"code-map-key-unknown", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byRank":{"3":1}`, keysErr("byRank.3", "unknown value 3")},
		{"code-map-key-negative-width", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byRank":{"-7":1}`, keysErr("byRank.-7", rangeWidth)},
		{"ref-code-map-key-unknown", `"byName":{"x":[{"id":1}]}`, `"byName":{"x":[{"id":1}]},"byMedal":{"3":1}`, keysErr("byMedal.3", "unknown value 3")},
	}
}

// boundsBase is a Bounds every refinement accepts (testdata/ts/range_bounds.txtar).
const boundsBase = `{"big":5,"huge":5,"f":0,"f32":0.0625,"d":2000,"s":"ab","tags":[1],"m":{"a":1},"opts":[1],"ex":4}`

func boundsErr(path, message string) string { return rangeErr(rangeBounds, path, message) }

// boundsCases are the positions only a bigint, a bound past 2^53 or an integer key can reach.
func boundsCases() []rangeCase {
	return []rangeCase{
		{"base", "", "", rangeOK},
		{"big-top", `"big":5`, `"big":10000000000000000`, rangeOK}, {"big-past", `"big":5`, `"big":10000000000000001`, boundsErr("big", valueIn+"1..=10000000000000000")},
		{"big-lo", `"big":5`, `"big":0`, boundsErr("big", valueIn+"1..=10000000000000000")},
		{"huge-safe", `"huge":5`, `"huge":9007199254740991`, rangeOK},
		{"huge-lo", `"huge":5`, `"huge":-1`, boundsErr("huge", valueIn+"0..=10000000000000000")},
		{"bigs-out", `"ex":4`, `"ex":4,"bigs":[5]`, boundsErr("bigs[0]", valueIn+"..5")},
		{"bigk-in", `"ex":4`, `"ex":4,"bigk":{"5":1}`, rangeOK}, {"bigk-out", `"ex":4`, `"ex":4,"bigk":{"6":1}`, boundsErr("bigk.6", valueIn+"0..=5")},
		{"bytes-in", `"ex":4`, `"ex":4,"bytes":{"255":1}`, rangeOK}, {"bytes-width", `"ex":4`, `"ex":4,"bytes":{"300":1}`, boundsErr("bytes.300", rangeWidth)},
		{"bytes-negative", `"ex":4`, `"ex":4,"bytes":{"-1":1}`, boundsErr("bytes.-1", rangeWidth)},
		{"bytes-key-leading-zero", `"ex":4`, `"ex":4,"bytes":{"01":1}`, boundsErr("bytes.01", rangeKey)},
		{"bytes-key-minus-zero", `"ex":4`, `"ex":4,"bytes":{"-0":1}`, boundsErr("bytes.-0", rangeKey)},
		{"bytes-key-plus", `"ex":4`, `"ex":4,"bytes":{"+1":1}`, boundsErr("bytes.+1", rangeKey)},
		{"bytes-key-text", `"ex":4`, `"ex":4,"bytes":{"x":1}`, boundsErr("bytes.x", rangeKey)},
		{"bytes-key-unsafe", `"ex":4`, `"ex":4,"bytes":{"9007199254740993":1}`, boundsErr("bytes.9007199254740993", rangeSafe())},
		{"bytes-key-unsafe-negative", `"ex":4`, `"ex":4,"bytes":{"-9007199254740993":1}`, boundsErr("bytes.-9007199254740993", rangeSafe())},
		{"bytes-key-unsafe-noncanonical", `"ex":4`, `"ex":4,"bytes":{"09007199254740993":1}`, boundsErr("bytes.09007199254740993", rangeKey)},
		{"bigk-key-leading-zero", `"ex":4`, `"ex":4,"bigk":{"01":1}`, boundsErr("bigk.01", rangeKey)},
		{"bigk-key-minus-zero", `"ex":4`, `"ex":4,"bigk":{"-0":1}`, boundsErr("bigk.-0", rangeKey)},
		{"bigk-key-text", `"ex":4`, `"ex":4,"bigk":{"x":1}`, boundsErr("bigk.x", rangeKey)},
	}
}

// rangeSafe is E8303's message, the one TypeScript's own safe-integer limit is refused with (CODEGEN.md §8.2).
func rangeSafe() string { return diag.E8303.Def().Variants[0].Template }

const (
	rangeMissing = "missing"
	rangeNull    = "null"
	rangeString  = "expected a string"
	rangeInteger = "expected an integer"
)

// wireCases are the failures of the wire's own shape, in the bytes every target writes (CODEGEN.md §5.13, WIRE.md §5.5, §5.6): a required key absent or null, a variant tag absent, null, unknown or not a string.
func wireCases() []rangeCase {
	const shape, slots = `"shape":{"kind":"circle","r":1}`, `"n0":1,"w0":1,"n1":2,"w1":2`
	return []rangeCase{
		{"required-missing", `"n":5,`, ``, probeErr("n", rangeMissing)},
		{"required-null", `"n":5`, `"n":null`, probeErr("n", rangeNull)},
		{"nested-missing", `"sub":{"v":1}`, `"sub":{}`, probeErr("sub.v", rangeMissing)},
		{"nested-null", `"sub":{"v":1}`, `"sub":{"v":null}`, probeErr("sub.v", rangeNull)},
		{"tag-missing", shape, `"shape":{"r":1}`, probeErr("shape.kind", rangeMissing)},
		{"tag-null", shape, `"shape":{"kind":null,"r":1}`, probeErr("shape.kind", rangeNull)},
		{"tag-not-string", shape, `"shape":{"kind":3,"r":1}`, probeErr("shape.kind", rangeString)},
		{"tag-unknown", shape, `"shape":{"kind":"blob","r":1}`, probeErr("shape.kind", "unknown case blob")},
		{"case-field-missing", shape, `"shape":{"kind":"circle"}`, probeErr("shape.r", rangeMissing)},
		// WIRE.md §5.14, CODEGEN.md §5.13: a pairs slot is refused at its first key present, as `incomplete pairs slot`, `null in a pairs slot` or `pairs slot after an empty one`.
		{"pairs-incomplete-key", slots, `"n0":1,"w0":1,"n1":2`, probeErr("n1", "incomplete pairs slot")},
		{"pairs-incomplete-value", slots, `"n0":1,"w0":1,"w1":2`, probeErr("w1", "incomplete pairs slot")},
		{"pairs-null-key", slots, `"n0":1,"w0":1,"n1":null,"w1":2`, probeErr("n1", "null in a pairs slot")},
		{"pairs-null-value", slots, `"n0":1,"w0":1,"n1":2,"w1":null`, probeErr("n1", "null in a pairs slot")},
		{"pairs-null-alone", slots, `"n0":1,"w0":1,"n1":null`, probeErr("n1", "incomplete pairs slot")},
		{"pairs-after-empty", slots, `"n1":2,"w1":2`, probeErr("n1", "pairs slot after an empty one")},
		{"pairs-null-after-empty", slots, `"n1":null,"w1":2`, probeErr("n1", "null in a pairs slot")},
	}
}

// nullCases are a null where a value is expected, `null` at its path in every target (WIRE.md §5.4, CODEGEN.md §5.13): a list element, a map value, a record, the top level, and a step of a @json(path:); an optional position reads it as none. `null-list`, `null-record` and `null-variant` pin the member-null read (decGet), not the readers' null check.
func nullCases() []rangeCase {
	const tail = `"ds":1`
	return []rangeCase{
		{"null-list-element", `"tags":[1,2]`, `"tags":[1,null]`, probeErr("tags[1]", rangeNull)},
		{"null-list", `"tags":[1,2]`, `"tags":null`, probeErr("tags", rangeNull)},
		{"null-map-value", `"me":{"a":1}`, `"me":{"a":null}`, probeErr("me.a", rangeNull)},
		{"null-map-record-value", `"mr":{"a":{"v":1}}`, `"mr":{"a":null}`, probeErr("mr.a", rangeNull)},
		{"null-record", `"sub":{"v":1}`, `"sub":null`, probeErr("sub", rangeNull)},
		{"null-variant", `"shape":{"kind":"circle","r":1}`, `"shape":null`, probeErr("shape", rangeNull)},
		{"null-top", rangeBase, `null`, "error " + rangeProbe + ": $: " + rangeNull},
		{"null-optional-element", `"opts":[null,3]`, `"opts":[null,null]`, rangeOK},
		{"null-optional-map-value", tail, tail + `,"mo":{"a":null}`, rangeOK},
		{"null-optional-map-range", tail, tail + `,"mo":{"a":10}`, probeErr("mo.a", valueIn+"0..=9")},
		{"path-required-absent", `,"p":{"q":1}`, ``, probeErr("p.q", rangeMissing)},
		{"path-required-leaf-null", `"q":1`, `"q":null`, probeErr("p.q", rangeNull)},
		{"path-required-step-null", `"p":{"q":1}`, `"p":null`, probeErr("p", rangeNull)},
		{"path-required-step-int", `"p":{"q":1}`, `"p":3`, probeErr("p", "expected an object")},
		{"path-required-range", `"q":1`, `"q":10`, probeErr("p.q", valueIn+"0..=9")},
		{"path-defaulted-empty", tail, tail + `,"pd":{}`, rangeOK},
		{"path-defaulted-leaf", tail, tail + `,"pd":{"q":3}`, rangeOK},
		{"path-defaulted-leaf-null", tail, tail + `,"pd":{"q":null}`, probeErr("pd.q", rangeNull)},
		{"path-defaulted-step-null", tail, tail + `,"pd":null`, probeErr("pd", rangeNull)},
		{"path-defaulted-range", tail, tail + `,"pd":{"q":10}`, probeErr("pd.q", valueIn+"0..=9")},
		{"path-optional-leaf-null", tail, tail + `,"po":{"q":null}`, rangeOK},
		{"path-optional-step-null", tail, tail + `,"po":null`, probeErr("po", rangeNull)},
		{"path-optional-range", tail, tail + `,"po":{"q":10}`, probeErr("po.q", valueIn+"0..=9")},
	}
}

// enumCases are the enum positions of range_keys: a member by wire, a code that is no integer, and a required key absent or null (WIRE.md §5.3, CODEGEN.md §5.13).
func enumCases() []rangeCase {
	const slots, medals, byName = `{"side":"left"},{"side":"RIGHT"}`, `{"r":1},{"r":2}`, `"byName":{"x":[{"id":1}]}`
	return []rangeCase{
		{"enum-unknown-member", slots, `{"side":"mid"},{"side":"RIGHT"}`, keysErr("slots[0].side", "unknown value mid")},
		{"enum-not-string", slots, `{"side":3},{"side":"RIGHT"}`, keysErr("slots[0].side", rangeString)},
		{"enum-case-differs", slots, `{"side":"right"},{"side":"RIGHT"}`, keysErr("slots[0].side", "unknown value right")},
		{"code-not-integer", medals, `{"r":"high"},{"r":2}`, keysErr("medals[0].r", rangeInteger)},
		{"code-fraction", medals, `{"r":1.5},{"r":2}`, keysErr("medals[0].r", rangeInteger)},
		{"code-missing", medals, `{},{"r":2}`, keysErr("medals[0].r", rangeMissing)},
		{"code-null", medals, `{"r":null},{"r":2}`, keysErr("medals[0].r", rangeNull)},
		{"default-null", byName, byName + `,"byRank":null`, keysErr("byRank", rangeNull)},
		{"code-key-width", byName, byName + `,"byRank":{"300":1}`, keysErr("byRank.300", rangeWidth)},
	}
}
