package format_test

import "testing"

// idempotenceCase is a minimal input derived from a progen counterexample under
// internal/testkit/progen/testdata/counterexamples/ (their "open format" lines now dropped).
type idempotenceCase struct {
	name, rule, in string
}

// idempotenceCases are one row per root cause the progen suites found and shrank.
var idempotenceCases = []idempotenceCase{
	{
		name: "emptyListStaysSingleLine",
		rule: "FORMATTER.md §6.1: an empty list is \"{}\" unless it holds a comment",
		in:   "package p\n\nfn f() -> Bool {\n  return x in {\n  }\n}\n",
	},
	{
		name: "joinsLineKeepsComma",
		rule: "FORMATTER.md §6.1, §8.1, DECISIONS 211 (extends 179 past \".\")",
		in:   "package p\n\nenum E {\n  a @since(1),\n  in\n}\n",
	},
	{
		name: "flatConstructDropsForcedBit",
		rule: "FORMATTER.md §6.2, DECISIONS 169: expressions inside a type are always flat",
		in:   "package p\n\nrecord Deck {\n  b: _ where { fire {\n  } }() @ts()\n}\n",
	},
	{
		name: "fitsAmbientFlatWinsOverForcedBit",
		rule: "FORMATTER.md §7.1, DECISIONS 170/212: fits() judges a group as the printer prints it",
		in: "package a\n\nlocal type T(stepSize: {count: Item}) = _ where match -a {\n" +
			"  type(it), fire, none => {} == \" \" + \"\"\n}[12 is IK1_WEAPON and  label - 1_000_000]\n",
	},
}

// TestIdempotenceRegressions guards fmt(fmt(x)) == fmt(x) and reparse-to-the-same-tree against
// the root causes the progen suites found (internal/testkit/progen).
func TestIdempotenceRegressions(t *testing.T) {
	for _, c := range idempotenceCases {
		t.Run(c.name, func(t *testing.T) {
			in := parse(t, "a/a.canon", []byte(c.in))
			out := mustFile(t, in.file)
			checkFormatted(t, "a/a.canon", in, out)
		})
	}
}
