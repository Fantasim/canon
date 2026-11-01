package gogen_test

import "slices"

// rangeBase is a Probe every refinement accepts.
const rangeBase = `{"n":5,"u8":7,"i8":7,"ex":4,"f":0.5,"f32":0.0625,"d":2000,"s":"ab","tags":[1,2],"m":{"a":1},"mi":{"50":"x"},"opts":[null,3],"lit":"auto","sub":{"v":1},"shape":{"kind":"circle","r":1},"dsel":"b","dep":3,"rows":[{"id":1},{"id":2}],"flags":3,"n0":1,"w0":1,"n1":2,"w1":2,"ds":1,"me":{"a":1},"mr":{"a":{"v":1}},"ml":{"a":[1]},"mrows":{"a":[{"id":1}]},"mm":{"a":{"b":1}}}`

// rangeCase is rangeBase with from replaced by to, and the line the driver prints for it.
type rangeCase struct{ name, from, to, want string }

const (
	rangeOK      = "ok"
	valueIn      = "expected a value in "
	countIn      = "expected an element count in "
	bytesIn      = "expected a byte length in "
	rangeWidth8  = "expected an integer from -128 to 127"
	rangeWidthU8 = "expected an integer from 0 to 255"
)

// rangeErr is the driver's line for a refusal at path.
func rangeErr(path, message string) string { return "error Probe: $." + path + ": " + message }

// numberCases are the integer, float, Float32 and Duration positions of rangeCases.
func numberCases() []rangeCase {
	return []rangeCase{
		{"int-lo-in", `"n":5`, `"n":1`, rangeOK}, {"int-hi-in", `"n":5`, `"n":120`, rangeOK},
		{"int-lo-out", `"n":5`, `"n":0`, rangeErr("n", valueIn+"1..=120")}, {"int-hi-out", `"n":5`, `"n":121`, rangeErr("n", valueIn+"1..=120")},
		{"width-first", `"i8":7`, `"i8":200`, rangeErr("i8", rangeWidth8)},
		{"fraction-first", `"n":5`, `"n":0.5`, rangeErr("n", "expected an integer")},
		{"u8-top", `"u8":7`, `"u8":255`, rangeOK}, {"u8-width", `"u8":7`, `"u8":256`, rangeErr("u8", rangeWidthU8)},
		{"i8-lo-out", `"i8":7`, `"i8":-1`, rangeErr("i8", valueIn+"0..=300")}, {"i8-top", `"i8":7`, `"i8":127`, rangeOK},
		{"end-in", `"ex":4`, `"ex":-9`, rangeOK}, {"end-out", `"ex":4`, `"ex":5`, rangeErr("ex", valueIn+"..5")},
		{"float-lo-in", `"f":0.5`, `"f":0`, rangeOK}, {"float-hi-out", `"f":0.5`, `"f":1`, rangeErr("f", valueIn+"0..1")},
		{"float-lo-out", `"f":0.5`, `"f":-0.25`, rangeErr("f", valueIn+"0..1")},
		{"f32-tenth", `"f32":0.0625`, `"f32":0.1`, rangeErr("f32", valueIn+"..=0.1")}, {"f32-below", `"f32":0.0625`, `"f32":0.09999`, rangeOK},
		{"dur-lo-in", `"d":2000`, `"d":1000`, rangeOK}, {"dur-lo-out", `"d":2000`, `"d":999`, rangeErr("d", valueIn+"1s..=1h")},
		{"dur-hi-in", `"d":2000`, `"d":3600000`, rangeOK}, {"dur-hi-out", `"d":2000`, `"d":3600001`, rangeErr("d", valueIn+"1s..=1h")},
	}
}

// collectionCases are the strings, lists, maps, optionals and unions of rangeCases.
func collectionCases() []rangeCase {
	return []rangeCase{
		{"str-two-byte", `"s":"ab"`, `"s":"éé"`, rangeOK}, {"str-two-byte-out", `"s":"ab"`, `"s":"ééé"`, rangeErr("s", bytesIn+"1..=4")},
		{"str-four-byte", `"s":"ab"`, `"s":"😀"`, rangeOK}, {"str-four-byte-out", `"s":"ab"`, `"s":"😀a"`, rangeErr("s", bytesIn+"1..=4")},
		{"str-empty", `"s":"ab"`, `"s":""`, rangeErr("s", bytesIn+"1..=4")},
		{"list-count-in", `"tags":[1,2]`, `"tags":[1,2,3]`, rangeOK}, {"list-count-out", `"tags":[1,2]`, `"tags":[1,2,3,4]`, rangeErr("tags", countIn+"..=3")},
		{"list-elem-out", `"tags":[1,2]`, `"tags":[6]`, rangeErr("tags[0]", valueIn+"0..=5")},
		{"list-elem-first", `"tags":[1,2]`, `"tags":[1,6,1,1]`, rangeErr("tags[1]", valueIn+"0..=5")},
		{"map-count-out", `"m":{"a":1}`, `"m":{"a":1,"b":2,"c":3}`, rangeErr("m", countIn+"..=2")},
		{"map-count-in", `"m":{"a":1}`, `"m":{"a":1,"b":2}`, rangeOK},
		{"map-key-out", `"m":{"a":1}`, `"m":{"abcd":1}`, rangeErr("m.abcd", bytesIn+"1..=3")},
		{"map-value-out", `"m":{"a":1}`, `"m":{"a":10}`, rangeErr("m.a", valueIn+"0..=9")},
		{"map-int-key-out", `"mi":{"50":"x"}`, `"mi":{"101":"x"}`, rangeErr("mi.101", valueIn+"..=100")},
		{"opt-elem-out", `"opts":[null,3]`, `"opts":[9]`, rangeErr("opts[0]", valueIn+"0..=5")},
		{"opt-null", `"n":5,`, `"n":5,"opt":null,`, rangeOK}, {"opt-out", `"n":5,`, `"n":5,"opt":9,`, rangeErr("opt", valueIn+"0..=5")},
		{"opt-in", `"n":5,`, `"n":5,"opt":5,`, rangeOK},
		{"marker-none", `"n":5,`, `"n":5,"mk":-1,`, rangeOK}, {"marker-out", `"n":5,`, `"n":5,"mk":6,`, rangeErr("mk", valueIn+"0..=5")},
		{"union-literal", `"lit":"auto"`, `"lit":"auto"`, rangeOK}, {"union-in", `"lit":"auto"`, `"lit":"abcdefg"`, rangeOK},
		{"union-out", `"lit":"auto"`, `"lit":"abc"`, rangeErr("lit", bytesIn+"6..=8")},
	}
}

// structCases are the records, cases, defaults, pairs and bits of rangeCases.
func structCases() []rangeCase {
	return []rangeCase{
		{"default-unchecked", `"n":5`, `"n":5`, rangeOK}, {"default-given", `"n":5,`, `"n":5,"dfl":9,`, rangeErr("dfl", valueIn+"1..=5")},
		{"record-field", `"sub":{"v":1}`, `"sub":{"v":4}`, rangeErr("sub.v", valueIn+"0..=3")},
		{"case-field", `"r":1`, `"r":3`, rangeErr("shape.r", valueIn+"0..=2")},
		{"pairs-count", `"w1":2`, `"w1":2,"n2":1,"w2":1`, rangeErr("slots", countIn+"1..=2")},
		{"pairs-value", `"w1":2`, `"w1":9`, rangeErr("w1", valueIn+"0..=3")},
		{"bits-count", `"flags":3`, `"flags":7`, rangeErr("flags", countIn+"1..=2")}, {"bits-in", `"flags":3`, `"flags":6`, rangeOK},
		{"bits-lo", `"flags":3`, `"flags":0`, rangeErr("flags", countIn+"1..=2")}, {"bits-lo-in", `"flags":3`, `"flags":4`, rangeOK},
		{"pairs-lo", `,"n0":1,"w0":1,"n1":2,"w1":2`, ``, rangeErr("slots", countIn+"1..=2")}, {"pairs-lo-in", `,"n1":2,"w1":2`, ``, rangeOK},
	}
}

// emptyKeyCases are CODEGEN.md §5.13, log-2026-10-31 (EMPTY-KEY): a member under map key k is `<parent>.<k>`, an empty key included. The paths are the TypeScript decoder's; the integer-key message is the canonical one, which TS-KEYMSG adopts.
func emptyKeyCases() []rangeCase {
	return []rangeCase{
		{"map-key-empty", `"m":{"a":1}`, `"m":{"":1}`, rangeErr("m.", bytesIn+"1..=3")},
		{"map-value-empty-key", `"me":{"a":1}`, `"me":{"":10}`, rangeErr("me.", valueIn+"0..=9")},
		{"map-record-empty-key", `"mr":{"a":{"v":1}}`, `"mr":{"":{"v":4}}`, rangeErr("mr..v", valueIn+"0..=3")},
		{"map-list-empty-key", `"ml":{"a":[1]}`, `"ml":{"":[1,6]}`, rangeErr("ml.[1]", valueIn+"0..=5")},
		{"map-keyed-dup-empty-key", `"mrows":{"a":[{"id":1}]}`, `"mrows":{"":[{"id":1},{"id":1}]}`, "error Probe: $.mrows.[1].id: duplicate id 1 (first at $.mrows.[0])"},
		{"map-in-map-both-empty", `"mm":{"a":{"b":1}}`, `"mm":{"":{"":10}}`, rangeErr("mm..", valueIn+"0..=9")},
		{"map-in-map-outer-empty", `"mm":{"a":{"b":1}}`, `"mm":{"":{"b":10}}`, rangeErr("mm..b", valueIn+"0..=9")},
		{"map-in-map-inner-empty", `"mm":{"a":{"b":1}}`, `"mm":{"a":{"":10}}`, rangeErr("mm.a.", valueIn+"0..=9")},
		{"map-int-key-empty", `"mi":{"50":"x"}`, `"mi":{"":"x"}`, rangeErr("mi.", "expected a canonical decimal integer key")},
		// CODEGEN.md §5.13: a map decodes per entry in document order, the key then its value: the first failure is the first entry's value, not a later key.
		{"map-int-key-after-bad-value", `"mi":{"50":"x"}`, `"mi":{"1":5,"a":"x"}`, rangeErr("mi.1", "expected a string")},
	}
}

// positionCases are the dependent branch and the keyed list of CODEGEN.md §5.15.
func positionCases() []rangeCase {
	const dupRow = "error Probe: $.rows[1].id: duplicate id 1 (first at $.rows[0])"
	return []rangeCase{
		{"dep-str-in", `"dsel":"b","dep":3`, `"dsel":"a","dep":"abc"`, rangeOK}, {"dep-str-lo", `"dsel":"b","dep":3`, `"dsel":"a","dep":"a"`, rangeErr("dep", bytesIn+"2..=3")},
		{"dep-str-hi", `"dsel":"b","dep":3`, `"dsel":"a","dep":"abcd"`, rangeErr("dep", bytesIn+"2..=3")},
		{"dep-int-lo", `"dep":3`, `"dep":-1`, rangeErr("dep", valueIn+"0..=5")}, {"dep-int-hi", `"dep":3`, `"dep":6`, rangeErr("dep", valueIn+"0..=5")},
		{"dep-int-in", `"dep":3`, `"dep":5`, rangeOK},
		{"keyed-count-out", `{"id":2}]`, `{"id":2},{"id":3}]`, rangeErr("rows", countIn+"..=2")},
		{"keyed-elem-out", `{"id":1}`, `{"id":10}`, rangeErr("rows[0].id", valueIn+"0..=9")},
		{"keyed-elem-first", `{"id":1},{"id":2}`, `{"id":10},{"id":2},{"id":3}`, rangeErr("rows[0].id", valueIn+"0..=9")},
		{"keyed-dup-first", `{"id":1},{"id":2}`, `{"id":1},{"id":1},{"id":3}`, dupRow},
		{"keyed-elem-before-dup", `{"id":1},{"id":2}`, `{"id":1},{"id":1},{"id":10}`, rangeErr("rows[2].id", valueIn+"0..=9")},
	}
}

// unitCases are a Duration read in seconds, judged in milliseconds after its unit (CODEGEN.md §5.15).
func unitCases() []rangeCase {
	return []rangeCase{
		{"unit-lo-in", `"ds":1`, `"ds":1`, rangeOK}, {"unit-hi-in", `"ds":1`, `"ds":2`, rangeOK},
		{"unit-hi-out", `"ds":1`, `"ds":3`, rangeErr("ds", valueIn+"1s..=2s")}, {"unit-lo-out", `"ds":1`, `"ds":0`, rangeErr("ds", valueIn+"1s..=2s")},
		{"unit-fraction", `"ds":1`, `"ds":1.5`, rangeOK}, {"unit-fraction-out", `"ds":1`, `"ds":2.5`, rangeErr("ds", valueIn+"1s..=2s")},
	}
}

// textCases are the @text result of the package: Decode<Fn>File of a list of 0..=5 integers, at most two (CODEGEN.md §5.15).
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
	return slices.Concat(cases, numberCases(), collectionCases(), structCases(), positionCases(), unitCases(), emptyKeyCases())
}
