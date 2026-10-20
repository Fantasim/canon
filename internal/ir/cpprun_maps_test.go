package ir_test

import (
	"cmp"
	"strings"
	"testing"
)

const (
	cppRunMaps = "testdata/e2e/cpprun/maps.txtar"
	// mapsEmpty is what the probe prints of a Board whose maps and list are all empty but names.
	mapsEmpty = "levels={} small={} colours={} ranks={} byShelf={} tones={} deep={} rows=[] slots={}"
	// mapsDecoder and mapsFile are how Board's own decoders and DecodeBoardsFile, reading the file wrapped in a list, name a failure's place (CODEGEN.md §5.13: errors name <Fn>File).
	mapsDecoder = "Board: $"
	mapsFile    = "BoardsFile: $[0]"
)

// mapCase is one Board file: what Decode prints, and Parse where it differs (DECISIONS 361: file order, a repeated key); DecodeBoardsFile reads it as Parse does.
type mapCase struct{ name, body, decode, parse string }

// mapCases: every key kind in file order under Parse and in key byte order under Decode (DECISIONS 384, 290), a ref key read unchecked (CODEGEN.md §5.13), and each refusal at its path (WIRE.md §5.8).
var mapCases = []mapCase{
	{"good.json", `{"names": {"z": 1, "a": 2}, "levels": {"10": "ten", "-3": "minus"}, "small": {"7": "p", "-5": "m"},` +
		` "colours": {"red": 2, "BLUE": 1}, "ranks": {"200": 1, "1": 2}, "byShelf": {"s2": 1, "s1": 2}, "tones": {"green": 2, "all": 1},` +
		` "deep": {"q": {"9": "n", "1": "o"}, "p": {}}, "rows": [{"b": 1, "a": 2}, {}], "slots": {"y": {"n": 1}, "x": {"n": 2}}}`,
		"names={a=2,z=1} levels={-3=minus,10=ten} small={-5=m,7=p} colours={BLUE=1,red=2} ranks={low=2,high=1} byShelf={s1=2,s2=1}" +
			" tones={all=1,green=2} deep={p={},q={1=o,9=n}} rows=[{a=2,b=1}{}] slots={x=n2,y=n1}",
		"names={z=1,a=2} levels={10=ten,-3=minus} small={7=p,-5=m} colours={red=2,BLUE=1} ranks={high=1,low=2} byShelf={s2=1,s1=2}" +
			" tones={green=2,all=1} deep={q={9=n,1=o},p={}} rows=[{b=1,a=2}{}] slots={y=n1,x=n2}"},
	{"absent.json", `{"names": {}}`, "names={} " + mapsEmpty, ""},
	{"noentry.json", `{"names": {}, "byShelf": {"zz": 1}}`, "names={} levels={} small={} colours={} ranks={} byShelf={zz=1} tones={} deep={} rows=[] slots={}", ""},
	{"intkey.json", `{"names": {}, "levels": {"01": "x"}}`, "error Board: $.levels.01: expected a canonical decimal integer key", ""},
	{"negzero.json", `{"names": {}, "small": {"-0": "x"}}`, "error Board: $.small.-0: expected a canonical decimal integer key", ""},
	{"range.json", `{"names": {}, "small": {"300": "x"}}`, "error Board: $.small.300: expected an integer from -128 to 127", ""},
	{"deepkey.json", `{"names": {}, "deep": {"q": {"x": "y"}}}`, "error Board: $.deep.q.x: expected a canonical decimal integer key", ""},
	{"colour.json", `{"names": {}, "colours": {"pink": 1}}`, "error Board: $.colours.pink: unknown value pink", ""},
	{"rankname.json", `{"names": {}, "ranks": {"low": 1}}`, "error Board: $.ranks.low: expected a canonical decimal integer key", ""},
	{"rankcode.json", `{"names": {}, "ranks": {"3": 1}}`, "error Board: $.ranks.3: unknown value 3", ""},
	{"tone.json", `{"names": {}, "tones": {"loud": 1}}`, "error Board: $.tones.loud: unknown value loud", ""},
	{"row.json", `{"names": {}, "rows": [{"a": "x"}]}`, "error Board: $.rows[0].a: expected an integer", ""},
	{"slot.json", `{"names": {}, "slots": {"x": {}}}`, "error Board: $.slots.x.n: missing", ""},
	{"missing.json", `{}`, "error Board: $.names: missing", ""},
	{"array.json", `{"names": []}`, "error Board: $.names: expected an object", ""},
	{"dup.json", `{"names": {"a": 1, "a": 2}}`, "names={a=2} " + mapsEmpty, "error Board: $.names.a: duplicate key"},
}

// TestCppTypesMapsRun is DECISIONS 384 (CODEGEN.md §5.9, §5.13; WIRE.md §5.8) compiled and run: a cpp types-mode emit reads its maps as the data loader does, Parse and Decode<Fn>File in file order, Decode in key byte order, refs unchecked.
func TestCppTypesMapsRun(t *testing.T) {
	t.Parallel()
	files := make([]cppRunFile, len(mapCases))
	for i, c := range mapCases {
		parse := cmp.Or(c.parse, c.decode)
		file := strings.Replace(parse, mapsDecoder, mapsFile, 1)
		files[i] = cppRunFile{name: c.name, body: c.body, want: "decode " + c.decode + " | parse " + parse + " | file " + file}
	}
	cppRunCheck(t, cppRunMaps, func(string) []cppRunFile { return files })
}
