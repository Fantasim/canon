package format_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

// threeKeys is a broken table literal of three entries.
const threeKeys = "package p\n\nlet c: table T = {\n  w { w: 0 }\n  x { w: 1 }\n  y { w: 2 }\n}\n"

// keysTable is a broken table literal of two entries and a comment on the first.
const keysTable = "package p\n\nlet c: table T = {\n  x { w: 1 } // one\n  y { w: 2 }\n}\n"

// entryNamed is the predicate for the entry of a file named name.
func entryNamed(name string) func(syntax.Node) bool {
	return func(n syntax.Node) bool {
		e, ok := n.(*syntax.EntryItem)
		return ok && e.Key.Name == name
	}
}

// keyOf is the key of the entry of f named name.
func keyOf(t *testing.T, f *syntax.File, name string) syntax.Node {
	t.Helper()
	return find(t, f, func(n syntax.Node) bool {
		e, ok := n.(*syntax.EntryItem)
		return ok && e.Key.Name == name
	}).(*syntax.EntryItem).Key
}

// keysCases are keys that need a comma above them (GRAMMAR.md §3.1 rule 3, §5.11).
var keysCases = []rewriteCase{
	{
		name: "replaceKeyJoining",
		rule: "GRAMMAR.md §3.1 rule 3, DECISIONS 459: \"and\" first on a line takes a comma after the entry above",
		src:  keysTable,
		want: "package p\n\nlet c: table T = {\n  x { w: 1 }, // one\n  and { w: 2 }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Replace, Node: keyOf(t, f, "y"), Text: "and"}}
		},
	},
	{
		name: "replaceKeyFor",
		rule: "GRAMMAR.md §5.11: \"for\" after an entry of a brace literal is a clause, so a comma comes before it",
		src:  keysTable,
		want: "package p\n\nlet c: table T = {\n  x { w: 1 }, // one\n  for { w: 2 }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Replace, Node: keyOf(t, f, "y"), Text: "for"}}
		},
	},
	{
		name: "replaceKeyDocAbove",
		rule: "GRAMMAR.md §3.1 rule 3: a doc comment between the entries does not stop \"in\" continuing the line above",
		src:  "package p\n\nlet c: table T = {\n  x { w: 1 }\n\n  /// Why.\n  y { w: 2 }\n}\n",
		want: "package p\n\nlet c: table T = {\n  x { w: 1 },\n\n  /// Why.\n  in { w: 2 }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Replace, Node: keyOf(t, f, "y"), Text: "in"}}
		},
	},
	{
		name: "removeAboveReplaceJoin",
		rule: "FORMATTER.md §13, DECISIONS 459: Remove x and Replace y by \"and\" put the comma over the final order",
		src:  threeKeys,
		want: "package p\n\nlet c: table T = {\n  w { w: 0 },\n  and { w: 2 }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{
				{Kind: format.Remove, Node: find(t, f, entryNamed("x"))},
				{Kind: format.Replace, Node: keyOf(t, f, "y"), Text: "and"},
			}
		},
	},
	{
		name: "insertAboveReplaceFor",
		rule: "FORMATTER.md §13, DECISIONS 459: Insert z and Replace y by \"for\" put the comma over the final order",
		src:  threeKeys,
		want: "package p\n\nlet c: table T = {\n  w { w: 0 }\n  x { w: 1 }\n  z { w: 9 },\n  for { w: 2 }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{
				{Kind: format.Insert, List: named(t, f, syntax.KindBraceLit, "{\n").First(), At: 2, Text: "z { w: 9 }"},
				{Kind: format.Replace, Node: keyOf(t, f, "y"), Text: "for"},
			}
		},
	},
	{
		name: "replaceNamedItemFor",
		rule: "GRAMMAR.md §5.11, DECISIONS 459: a named item \"for: 2\" after an item takes a comma above it",
		src:  "package p\n\nlet r = {\n  a: 1\n  b: 2\n}\n",
		want: "package p\n\nlet r = {\n  a: 1,\n  for: 2\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Replace, Node: named(t, f, syntax.KindFieldItem, "b:"), Text: "for: 2"}}
		},
	},
	{
		name: "insertAnnotatedFor",
		rule: "GRAMMAR.md §5.11, DECISIONS 459: an entry \"for @ann { }\" takes a comma above it",
		src:  keysTable,
		want: "package p\n\nlet c: table T = {\n  x { w: 1 } // one\n  y { w: 2 },\n  for @deprecated(\"x\") { w: 3 }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, List: named(t, f, syntax.KindBraceLit, "{\n").First(), At: 2, Text: "for @deprecated(\"x\") { w: 3 }"}}
		},
	},
	{
		name: "replaceKeyForAfterSpread",
		rule: "GRAMMAR.md §5.11, DECISIONS 459: \"for\" after a spread takes a comma above it",
		src:  "package p\n\nlet c: table T = {\n  ...base\n  y { w: 2 }\n}\n",
		want: "package p\n\nlet c: table T = {\n  ...base,\n  for { w: 2 }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Replace, Node: keyOf(t, f, "y"), Text: "for"}}
		},
	},
	{
		name: "replaceKeyStopsJoining",
		rule: "FORMATTER.md §13, DECISIONS 459: a key that stops joining the line above drops the comma it needed",
		src:  "package p\n\nlet c: table T = {\n  x { w: 1 },\n  and { w: 2 }\n}\n",
		want: "package p\n\nlet c: table T = {\n  x { w: 1 }\n  y { w: 2 }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Replace, Node: keyOf(t, f, "and"), Text: "y"}}
		},
	},
	{
		name: "replaceFirstKeyFor",
		rule: "GRAMMAR.md §5.11: \"for\" opening a brace literal follows no item, so no comma",
		src:  keysTable,
		want: "package p\n\nlet c: table T = {\n  for { w: 1 } // one\n  y { w: 2 }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Replace, Node: keyOf(t, f, "x"), Text: "for"}}
		},
	},
	{
		name: "replaceKeyPlain",
		rule: "FORMATTER.md §6.1: a key that continues no line adds no comma",
		src:  keysTable,
		want: "package p\n\nlet c: table T = {\n  x { w: 1 } // one\n  if { w: 2 }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Replace, Node: keyOf(t, f, "y"), Text: "if"}}
		},
	},
	{
		name: "insertFor",
		rule: "GRAMMAR.md §5.11, DECISIONS 459: an inserted \"for\" entry follows a comma",
		src:  keysTable,
		want: "package p\n\nlet c: table T = {\n  x { w: 1 } // one\n  y { w: 2 },\n  for { w: 3 }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, List: named(t, f, syntax.KindBraceLit, "{\n").First(), At: 2, Text: "for { w: 3 }"}}
		},
	},
	{
		name: "insertForStatementNoComma",
		rule: "GRAMMAR.md §5.10: a \"for\" statement is no entry of a literal, so a block takes no comma before it",
		src:  "package p\n\nfn f() -> Int {\n  let a = 1\n  return a\n}\n",
		want: "package p\n\nfn f() -> Int {\n  let a = 1\n  for x in [1] { a }\n  return a\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, List: named(t, f, syntax.KindBlock, "{").First(), At: 1, Text: "for x in [1] { a }"}}
		},
	},
}

// FORMATTER.md §13, DECISIONS 459: a key or entry joining the line above keeps the file parsing.
func TestRewriteJoiningKeys(t *testing.T) {
	for _, c := range keysCases {
		t.Run(c.name, c.run)
	}
}
