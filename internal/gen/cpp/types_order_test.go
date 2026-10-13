package cppgen_test

import "strings"

// documentOrderFiles are the texts of CODEGEN.md §5.13's "first failure in document order" and "a number valid by JSON is never a syntax error" (DECISIONS 361), read by C++ `Parse`: a failure is placed where it becomes certain, even inside a string or a document a later syntax error leaves unclosed.
var documentOrderFiles = []typesFile{
	{name: "ord_dup.json", arg: "x", text: `{"a":1,"a":2,`, parseOnly: true, parse: "ord_dup.json: error Extras: $.a: duplicate key\n"},
	{name: "ord_dupkey.json", arg: "x", text: `{"é": 1, "é": 2}`, parseOnly: true, parse: "ord_dupkey.json: error Extras: $.é: duplicate key\n"},
	{name: "ord_depth.json", arg: "x", text: strings.Repeat("[", 600), parseOnly: true,
		parse: "ord_depth.json: error Extras: $" + strings.Repeat("[0]", 512) + ": nested deeper than 512 levels\n"},
	{name: "ord_utf8open.json", arg: "x", text: "{\"a\": \"\xff", parseOnly: true, parse: "ord_utf8open.json: error Extras: $.a: not UTF-8\n"},
	{name: "ord_utf8split.json", arg: "x", text: "{\"a\": \"\xe2\x82", parseOnly: true, parse: "ord_utf8split.json: error Extras: invalid JSON at byte 9\n"},
	{name: "ord_utf8out.json", arg: "x", text: "{\"a\": 1} \xff", parseOnly: true, parse: "ord_utf8out.json: error Extras: invalid JSON at byte 9\n"},
	{name: "ord_low.json", arg: "x", text: `{"a": "\udc00`, parseOnly: true, parse: "ord_low.json: error Extras: $.a: unpaired surrogate\n"},
	{name: "ord_high.json", arg: "x", text: `{"a": "\ud800 x`, parseOnly: true, parse: "ord_high.json: error Extras: $.a: unpaired surrogate\n"},
	{name: "ord_lowutf8.json", arg: "x", text: "{\"a\": \"\\udc00\xff\"}", parseOnly: true, parse: "ord_lowutf8.json: error Extras: $.a: unpaired surrogate\n"},
	{name: "ord_highutf8.json", arg: "x", text: "{\"a\": \"\\ud800x\xff\"}", parseOnly: true, parse: "ord_highutf8.json: error Extras: $.a: unpaired surrogate\n"},
	{name: "ord_highslash.json", arg: "x", text: "{\"a\": \"\\ud800\\n\xff\"}", parseOnly: true, parse: "ord_highslash.json: error Extras: $.a: unpaired surrogate\n"},
	{name: "ord_utf8low.json", arg: "x", text: "{\"a\": \"\xff\\udc00\"}", parseOnly: true, parse: "ord_utf8low.json: error Extras: $.a: not UTF-8\n"},
	{name: "ord_utf8high.json", arg: "x", text: "{\"a\": \"\xff\\ud800\"}", parseOnly: true, parse: "ord_utf8high.json: error Extras: $.a: not UTF-8\n"},
	{name: "ord_highcut.json", arg: "x", text: `{"a": "\ud800\`, parseOnly: true, parse: "ord_highcut.json: error Extras: invalid JSON at byte 14\n"},
	{name: "ord_keycut.json", arg: "x", text: `{"a":1,"a`, parseOnly: true, parse: "ord_keycut.json: error Extras: invalid JSON at byte 9\n"},
	{name: "ord_ucut.json", arg: "x", text: `{"a": "\udc`, parseOnly: true, parse: "ord_ucut.json: error Extras: invalid JSON at byte 11\n"},
	{name: "ord_ucut2.json", arg: "x", text: `{"a": "\ud800\u0`, parseOnly: true, parse: "ord_ucut2.json: error Extras: invalid JSON at byte 16\n"},
	{name: "ord_keyutf8.json", arg: "x", text: "{\"\xff\": 1}", parseOnly: true, parse: "ord_keyutf8.json: error Extras: $: not UTF-8\n"},
	{name: "ord_keysurrogate.json", arg: "x", text: `{"b": {"\ud800": 1}}`, parseOnly: true, parse: "ord_keysurrogate.json: error Extras: $.b: unpaired surrogate\n"},
	{name: "ord_overlong.json", arg: "x", text: "{\"a\": \"\xc0\x80\"}", parseOnly: true, parse: "ord_overlong.json: error Extras: $.a: not UTF-8\n"},
	{name: "num_ignored.json", arg: "x", text: `{"zz": 1e400, "mode": "auto"}`, parseOnly: true, parse: "num_ignored.json: reqMp=7 label=x mode=auto"},
	{name: "num_ignoredint.json", arg: "x", text: `{"zz": [` + strings.Repeat("9", 400) + `, -1E999, 1e-999]}`, parseOnly: true,
		parse: "num_ignoredint.json: reqMp=7 label=x mode=authoritative"},
	{name: "num_float.json", arg: "c", from: `"left": 6800`, to: `"left": 1e400`, parseOnly: true,
		parse: "num_float.json: error EventConfig: $.events[0].spawnRegion.left: number out of range\n"},
	{name: "num_floatneg.json", arg: "c", from: `"left": 6800`, to: `"left": -1e400`, parseOnly: true,
		parse: "num_floatneg.json: error EventConfig: $.events[0].spawnRegion.left: number out of range\n"},
	{name: "num_int.json", arg: "c", from: `"levelMax": 60`, to: `"levelMax": 1e400`, parseOnly: true,
		parse: "num_int.json: error EventConfig: $.events[1].levelMax: expected an integer\n"},
	{name: "num_float32.json", arg: "x", text: `{"ratio": 1e400}`, parseOnly: true, parse: "num_float32.json: error Extras: $.ratio: expected a number that fits Float32\n"},
}
