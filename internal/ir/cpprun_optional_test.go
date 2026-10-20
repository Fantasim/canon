package ir_test

import (
	"cmp"
	"testing"
)

const (
	cppRunOptional = "testdata/e2e/cpprun/optional_elems.txtar"
	// optionalSheet is the sheet ob bakes and od's json emit writes, as the probe prints it.
	optionalSheet = "ints=[1,none,3] words=[none,w] cells=[n4,none] counts={a=none,b=2} byMood={calm=none,angry=n5} moods=[none,angry] waits=[5000ms,none]"
	// optionalData is the probe's argument loading od's data file, from the cases directory.
	optionalData = "data:../od/out/sheet.json"
	// optionalEmpty is a types file whose required lists and maps are empty.
	optionalEmpty = `"ints": [], "words": [], "cells": [], "counts": {}, "byMood": {}`
)

// optionalCases are types files: a present value and a null at every position, Decode's maps in key byte order and Parse's in file order (DECISIONS 384), defaults holding none (CODEGEN.md §5.13), and the refusals at their paths.
var optionalCases = []mapCase{
	{"full.json", `{"ints": [null, 2], "words": ["x", null], "cells": [null, {"n": 3}], "counts": {"z": 1, "a": null},` +
		` "byMood": {"calm": {"n": 6}, "angry": null}, "moods": ["calm", null], "waits": [null, 250]}`,
		"ints=[none,2] words=[x,none] cells=[none,n3] counts={a=none,z=1} byMood={angry=none,calm=n6} moods=[calm,none] waits=[none,250ms]",
		"ints=[none,2] words=[x,none] cells=[none,n3] counts={z=1,a=none} byMood={calm=n6,angry=none} moods=[calm,none] waits=[none,250ms]"},
	{"absent.json", "{" + optionalEmpty + "}", "ints=[] words=[] cells=[] counts={} byMood={} moods=[none,angry] waits=[5000ms,none]", ""},
	{"nulls.json", `{"ints": [null], "words": [null], "cells": [null], "counts": {"a": null}, "byMood": {"calm": null}, "moods": [null], "waits": [null]}`,
		"ints=[none] words=[none] cells=[none] counts={a=none} byMood={calm=none} moods=[none] waits=[none]", ""},
	{"elem.json", `{"ints": [null, "x"], "words": [], "cells": [], "counts": {}, "byMood": {}}`, "error Sheet: $.ints[1]: expected an integer", ""},
	{"cell.json", `{"ints": [], "words": [], "cells": [{}], "counts": {}, "byMood": {}}`, "error Sheet: $.cells[0].n: missing", ""},
	{"mood.json", "{" + optionalEmpty + `, "moods": ["sad"]}`, "error Sheet: $.moods[0]: unknown value sad", ""},
	{"value.json", `{"ints": [], "words": [], "cells": [], "counts": {"a": "x"}, "byMood": {}}`, "error Sheet: $.counts.a: expected an integer", ""},
	{"wait.json", "{" + optionalEmpty + `, "waits": [0.5]}`, "error Sheet: $.waits[0]: expected a whole number of milliseconds", ""},
	{"list.json", `{"ints": null, "words": [], "cells": [], "counts": {}, "byMood": {}}`, "error Sheet: $.ints: null", ""},
}

// TestCppOptionalElementsRun is DECISIONS 385 (CODEGEN.md §4.2, WIRE.md §5.4) compiled and run: a cpp emit holds optional list elements and map values as std::optional in baked, data and types mode alike, none written and read as std::nullopt.
func TestCppOptionalElementsRun(t *testing.T) {
	t.Parallel()
	files := []cppRunFile{{name: "baked", want: optionalSheet + " some=[1,none]"}, {name: optionalData, want: optionalSheet}}
	for _, c := range optionalCases {
		files = append(files, cppRunFile{name: c.name, body: c.body, want: "decode " + c.decode + " | parse " + cmp.Or(c.parse, c.decode)})
	}
	cppRunCheck(t, cppRunOptional, func(string) []cppRunFile { return files })
}
