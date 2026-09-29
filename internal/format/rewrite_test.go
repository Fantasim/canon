package format_test

import (
	"bytes"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

// statuses is FORMATTER.md §9.2's table after formatting, the file of §13's examples.
const statuses = `package p

let statuses: stable table Status = {
  open { tone: warning, label: "Open", terminal: false, next: [taken, wont_do, duplicate] }
  taken {
    tone: info
    label: "Taken"
  }
  wont_do { tone: neutral, label: "Won't do", terminal: true, next: [open], requires: [reason] }
}
`

// keptDoc and keptAfter hold the two commas DECISIONS 216 keeps on their own line.
const (
	keptDoc   = "package p\n\nenum E {\n  a\n  /// d1\n  ,\n  b\n}\n"
	keptAfter = "package p\n\nenum E {\n  a\n  , /// x\n  b\n}\n"
)

// find is the first node of f, in source order, that pred accepts.
func find(t *testing.T, f *syntax.File, pred func(syntax.Node) bool) syntax.Node {
	t.Helper()
	var found syntax.Node
	syntax.Inspect(f, func(n syntax.Node) bool {
		if found == nil && n != nil && pred(n) {
			found = n
		}
		return found == nil
	})
	if found == nil {
		t.Fatal("no such node")
	}
	return found
}

// named is the first node of a kind whose text starts with prefix.
func named(t *testing.T, f *syntax.File, k syntax.NodeKind, prefix string) syntax.Node {
	t.Helper()
	return find(t, f, func(n syntax.Node) bool {
		s := f.Span(n)
		return n.Kind() == k && strings.HasPrefix(string(f.Src.Content[s.Start:s.End]), prefix)
	})
}

// rewriteCase is one Rewrite of src, the changes built on its tree.
type rewriteCase struct {
	name, rule, src, want string
	owner                 string
	changes               func(t *testing.T, f *syntax.File) []format.Change
}

func (c rewriteCase) run(t *testing.T) {
	t.Helper()
	in := parse(t, "a/a.canon", []byte(c.src))
	for _, fd := range in.bag.Findings() {
		if fd.Severity == diag.Error {
			t.Fatalf("%s: the input has the error %s", c.rule, fd.Code)
		}
	}
	got, err := format.Rewrite(in.file, c.changes(t, in.file))
	if err != nil {
		t.Fatalf("%s: %v", c.rule, err)
	}
	if string(got) != c.want {
		t.Fatalf("%s:\n%s", c.rule, lineDiff([]byte(c.want), got))
	}
	if again, err := formatText(t, "a/a.canon", got); err != nil || !bytes.Equal(again, got) {
		t.Fatalf("%s: not a fixed point (API.md M5, M6): %v\n%s", c.rule, err, lineDiff(got, again))
	}
}

// listOf is the opening bracket of the first list literal or brace literal starting with prefix.
func listOf(t *testing.T, f *syntax.File, k syntax.NodeKind, prefix string) syntax.Tok {
	t.Helper()
	return named(t, f, k, prefix).First()
}

var rewriteCases = []rewriteCase{
	{
		name: "setOneLine",
		rule: "FORMATTER.md §13 steps 1-2, API.md M6: a one-value Set changes one line",
		src:  statuses,
		want: strings.Replace(statuses, `label: "Won't do"`, `label: "Will not do"`, 1),
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Replace, Node: named(t, f, syntax.KindStringLit, `"Won't do"`), Text: `"Will not do"`}}
		},
	},
	{
		name: "addStaysOnItsLine",
		rule: "FORMATTER.md §13 step 4: \", item\" before the closing bracket of a single-line list that still fits",
		src:  statuses,
		want: strings.Replace(statuses, "[taken, wont_do, duplicate]", "[taken, wont_do, duplicate, fixed]", 1),
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, List: listOf(t, f, syntax.KindListLit, "[taken"), At: 3, Text: "fixed"}}
		},
	},
	{
		name: "addEscalates",
		rule: "FORMATTER.md §13 step 3: a line past 100 columns escalates to the item holding the single-line list",
		src:  statuses,
		want: strings.Replace(statuses,
			`  open { tone: warning, label: "Open", terminal: false, next: [taken, wont_do, duplicate] }`,
			"  open {\n    tone: warning\n    label: \"Open\"\n    terminal: false\n"+
				"    next: [taken, wont_do, duplicate, fixed, verified]\n  }", 1),
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			l := listOf(t, f, syntax.KindListLit, "[taken")
			return []format.Change{
				{Kind: format.Insert, List: l, At: 3, Text: "fixed"},
				{Kind: format.Insert, List: l, At: 3, Text: "verified"},
			}
		},
	},
	{
		name: "insertBrokenBrace",
		rule: "FORMATTER.md §13 step 4: a new line at the insertion point, at the item indentation, no blank line",
		src:  "package p\n\nlet c: C = {\n  a: 1\n\n  // c\n  c: 3\n}\n",
		want: "package p\n\nlet c: C = {\n  a: 1\n  b: 2\n\n  // c\n  c: 3\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, List: listOf(t, f, syntax.KindBraceLit, "{\n"), At: 1, Text: "b: 2"}}
		},
	},
	{
		name: "insertFirstAndEmpty",
		rule: "FORMATTER.md §13 step 4: \"{}\" becomes \"{ item }\", \"[]\" becomes \"[item]\"; the first item goes before the others",
		src:  "package p\n\nlet c: C = { a: {}, b: [], c: [2, 3] }\n",
		want: "package p\n\nlet c: C = { a: { x: 1 }, b: [1], c: [1, 2, 3] }\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{
				{Kind: format.Insert, List: listOf(t, f, syntax.KindBraceLit, "{}"), Text: "x: 1"},
				{Kind: format.Insert, List: listOf(t, f, syntax.KindListLit, "[]"), Text: "1"},
				{Kind: format.Insert, List: listOf(t, f, syntax.KindListLit, "[2"), Text: "1"},
			}
		},
	},
	{
		name: "insertBrokenParen",
		rule: "FORMATTER.md §13 step 4: into a broken ( )/[ ] list, a new line \"item,\"",
		src: "package p\n\nlet xs: [String] = [\n  \"a long string that makes the list break, and then some more text, and more, and more\",\n" +
			"  \"b\",\n]\n",
		want: "package p\n\nlet xs: [String] = [\n  \"a long string that makes the list break, and then some more text, and more, and more\",\n" +
			"  \"b\",\n  \"c\",\n]\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, List: listOf(t, f, syntax.KindListLit, "["), At: 2, Text: `"c"`}}
		},
	},
	{
		name: "insertKeyword",
		rule: "FORMATTER.md §13 step 4 and DECISIONS 211: \"in\" keeps a comma before it and after it",
		src:  "package p\n\nenum E {\n  a\n  b\n}\n",
		want: "package p\n\nenum E {\n  a,\n  in,\n  b\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, List: named(t, f, syntax.KindEnumDecl, "enum").(*syntax.EnumDecl).Braces.Open, At: 1, Text: "in"}}
		},
	},
	{
		name: "removeBroken",
		rule: "FORMATTER.md §13 step 5: an item's lines go with its comments; two blank lines in a row become one",
		src:  "package p\n\nlet c: C = {\n  a: 1\n\n  // about b\n  b: 2 // two\n\n  c: 3\n}\n",
		want: "package p\n\nlet c: C = {\n  a: 1\n\n  c: 3\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindFieldItem, "b")}}
		},
	},
	{
		name: "removeBrokenEdges",
		rule: "FORMATTER.md §13 step 5: no blank line is left after \"{\" or before \"}\"",
		src:  "package p\n\nlet c: C = {\n  a: 1\n\n  b: 2\n\n  c: 3\n}\n",
		want: "package p\n\nlet c: C = {\n  b: 2\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{
				{Kind: format.Remove, Node: named(t, f, syntax.KindFieldItem, "a")},
				{Kind: format.Remove, Node: named(t, f, syntax.KindFieldItem, "c")},
			}
		},
	},
	{
		name: "removeLastLeavesEmpty",
		rule: "FORMATTER.md §13 step 5: a list left empty becomes \"{}\" or \"[]\"",
		src:  "package p\n\nlet c: C = {\n  a: [1]\n  b: {\n    x: 1\n  }\n}\n",
		want: "package p\n\nlet c: C = {\n  a: []\n  b: {}\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{
				{Kind: format.Remove, Node: named(t, f, syntax.KindIntLit, "1")},
				{Kind: format.Remove, Node: named(t, f, syntax.KindFieldItem, "x")},
			}
		},
	},
	{
		name: "removeInline",
		rule: "FORMATTER.md §13 step 5: from a single-line list, the item and one adjacent \", \"",
		src:  "package p\n\nlet c: C = { a: [1, 2, 3], b: [4, 5] }\n",
		want: "package p\n\nlet c: C = { a: [1, 3], b: [4] }\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{
				{Kind: format.Remove, Node: named(t, f, syntax.KindIntLit, "2")},
				{Kind: format.Remove, Node: named(t, f, syntax.KindIntLit, "5")},
			}
		},
	},
}

// commentCases are the review's repros of comments in a list on one line, compared whole, each
// comment's owner, the token the formatter attaches it to, asserted in the result
// (log-2026-09-29 M4 U1r): owner is "comment=>token".
var commentCases = []rewriteCase{
	{
		name:  "insertAfterTrailingComment",
		rule:  "log-2026-09-29 M4 U1r, DECISIONS 167: the comment is 2's; the settle writes it after the comma, where it is still 2's",
		src:   "package p\n\nlet c = [1, 2 /* two */ ]\n",
		want:  "package p\n\nlet c = [1, 2, /* two */ 3]\n",
		owner: "/* two */=>2",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, List: listOf(t, f, syntax.KindListLit, "["), At: 2, Text: "3"}}
		},
	},
	{
		name:  "insertBeforeLeadComment",
		rule:  "log-2026-09-29 M4 U1r: a comment after the opening bracket is the bracket's and stays after it",
		src:   "package p\n\nlet c = [ /* lead */ 1, 2]\n",
		want:  "package p\n\nlet c = [ /* lead */ 0, 1, 2]\n",
		owner: "/* lead */=>[",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, List: listOf(t, f, syntax.KindListLit, "["), At: 0, Text: "0"}}
		},
	},
	{
		name:  "removeAfterCommentedItem",
		rule:  "log-2026-09-29 M4 U1r: a comment before a comma is the item's before it",
		src:   "package p\n\nlet c = [1 /* one */, 2, 3]\n",
		want:  "package p\n\nlet c = [1 /* one */, 3]\n",
		owner: "/* one */=>1",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindIntLit, "2")}}
		},
	},
	{
		name:  "removeLastArgument",
		rule:  "log-2026-09-29 M4 U1r: removing the last argument keeps the comment of the one before",
		src:   "package p\n\nlet c = f(1 /* one */, 2)\n",
		want:  "package p\n\nlet c = f(1 /* one */)\n",
		owner: "/* one */=>1",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindArg, "2")}}
		},
	},
	{
		name:  "insertBeforeCommentAfterComma",
		rule:  "log-2026-09-29 M4 U1r, DECISIONS 168: a comment after a dropped comma is the item's before it and stays with it",
		src:   "package p\n\nlet c: C = { a: 1, /* bee */ b: 2, c: 3 }\n",
		want:  "package p\n\nlet c: C = { a: 1 /* bee */, x: 9, b: 2, c: 3 }\n",
		owner: "/* bee */=>1",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, List: listOf(t, f, syntax.KindBraceLit, "{"), At: 1, Text: "x: 9"}}
		},
	},
	{
		name:  "removeCommentAfterComma",
		rule:  "log-2026-09-29 M4 U1r, DECISIONS 168: removing b keeps the comment after the comma before it, a's",
		src:   "package p\n\nlet c: C = { a: 1, /* bee */ b: 2, c: 3 }\n",
		want:  "package p\n\nlet c: C = { a: 1 /* bee */, c: 3 }\n",
		owner: "/* bee */=>1",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindFieldItem, "b")}}
		},
	},
	{
		name:  "removeAfterLineComment",
		rule:  "log-2026-09-29 M4 U1r: removing the item after a line comment keeps the comment, a's, and its line break",
		src:   "package p\n\nlet c = f(a // c\n  , b)\n",
		want:  "package p\n\nlet c = f(a // c\n  )\n",
		owner: "// c=>a",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindArg, "b")}}
		},
	},
	{
		name:  "removeBeforeOwnLineDoc",
		rule:  "log-2026-09-29 M4 U1r, DECISIONS 216: the first item goes with its kept comma's doc; the next item's doc keeps its line",
		src:   "package p\n\nlet c = [a\n  /// d\n  ,\n  /// e\n  b]\n",
		want:  "package p\n\nlet c = [\n  /// e\n  b]\n",
		owner: "/// e=>b",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindIdentExpr, "a")}}
		},
	},
}

// owner reports the text of the token the comment starting with prefix is attached to in f.
func owner(t *testing.T, f *syntax.File, prefix string) string {
	t.Helper()
	hosts := format.CommentHosts(f)
	for _, at := range slices.Sorted(maps.Keys(hosts)) {
		if tok := hosts[at]; strings.HasPrefix(string(f.Src.Content[at:]), prefix) {
			tk := f.Tokens[tok]
			return string(f.Src.Content[tk.Start:tk.End])
		}
	}
	t.Fatalf("no comment %q", prefix)
	return ""
}

// groupedCases are the changes planned together over a list (log-2026-09-29 M4 U1r).
var groupedCases = []rewriteCase{
	{
		name: "removeBothInline",
		rule: "log-2026-09-29 M4 U1r: the removals of one list are planned together; both items of { a: 1, b: 2 } leave {}",
		src:  "package p\n\nlet c: C = { a: 1, b: 2 }\n",
		want: "package p\n\nlet c: C = {}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{
				{Kind: format.Remove, Node: named(t, f, syntax.KindFieldItem, "a")},
				{Kind: format.Remove, Node: named(t, f, syntax.KindFieldItem, "b")},
			}
		},
	},
	{
		name: "removeTwoOfThree",
		rule: "log-2026-09-29 M4 U1r: removing 2 and 3 of [1, 2, 3]",
		src:  "package p\n\nlet c: [Int] = [1, 2, 3]\n",
		want: "package p\n\nlet c: [Int] = [1]\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{
				{Kind: format.Remove, Node: named(t, f, syntax.KindIntLit, "2")},
				{Kind: format.Remove, Node: named(t, f, syntax.KindIntLit, "3")},
			}
		},
	},
	{
		name: "removeAndInsertKeyword",
		rule: "log-2026-09-29 M4 U1r, DECISIONS 211: Remove b and Insert in at 2 of a broken enum, commas over the final order",
		src:  "package p\n\nenum E {\n  a\n  b\n  c\n}\n",
		want: "package p\n\nenum E {\n  a,\n  in,\n  c\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			l := named(t, f, syntax.KindEnumDecl, "enum").(*syntax.EnumDecl).Braces.Open
			return []format.Change{
				{Kind: format.Remove, Node: named(t, f, syntax.KindEnumMember, "b")},
				{Kind: format.Insert, List: l, At: 2, Text: "in"},
			}
		},
	},
	{
		name: "insertTwiceSamePlace",
		rule: "log-2026-09-29 M4 U1r, DECISIONS 211: two Inserts at one position, in change order, each with its commas",
		src:  "package p\n\nenum E {\n  a\n  b\n}\n",
		want: "package p\n\nenum E {\n  a,\n  in,\n  or,\n  x,\n  in,\n  b\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			l := named(t, f, syntax.KindEnumDecl, "enum").(*syntax.EnumDecl).Braces.Open
			return []format.Change{
				{Kind: format.Insert, List: l, At: 1, Text: "in"},
				{Kind: format.Insert, List: l, At: 1, Text: "or"},
				{Kind: format.Insert, List: l, At: 1, Text: "x"},
				{Kind: format.Insert, List: l, At: 1, Text: "in"},
			}
		},
	},
	{
		name: "mapSetEmpty",
		rule: "log-2026-09-29 M4 U1r, API.md M1: a map set to {} is one Remove per entry",
		src:  "package p\n\nlet m: {String: Int} = {\n  \"a\": 1\n\n  // b\n  \"b\": 2\n  \"c\": 3\n}\n",
		want: "package p\n\nlet m: {String: Int} = {}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			lit := named(t, f, syntax.KindBraceLit, "{\n").(*syntax.BraceLit)
			var cs []format.Change
			for _, it := range lit.Items {
				cs = append(cs, format.Change{Kind: format.Remove, Node: it})
			}
			return cs
		},
	},
	{
		name: "setAndInsertInOneList",
		rule: "log-2026-09-29 M4 U1r, API.md M1: a Set inside a kept item and an Insert in its list combine",
		src:  "package p\n\nlet c: C = {\n  a: 1\n  b: 2\n}\n",
		want: "package p\n\nlet c: C = {\n  a: 5\n  x: 9\n  b: 2\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{
				{Kind: format.Replace, Node: named(t, f, syntax.KindIntLit, "1"), Text: "5"},
				{Kind: format.Insert, List: named(t, f, syntax.KindBraceLit, "{\n").First(), At: 1, Text: "x: 9"},
			}
		},
	},
	{
		name: "newComprehension",
		rule: "log-2026-09-29 M4 U1r, FORMATTER.md §6.3: a list in a new brace comprehension's clauses breaks it",
		src:  "package p\n\nlet c = {\n  a: 1\n}\n",
		want: "package p\n\nlet c = {\n  a: {\n    k: k\n    for k in [1]\n  }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Replace, Node: named(t, f, syntax.KindIntLit, "1"), Text: "{ k: k for k in [1] }"}}
		},
	},
	{
		name: "appendDeclaration",
		rule: "log-2026-09-29 M4 G1, FORMATTER.md §6.3: a declaration goes after the last one and its trailing comment, one blank line apart",
		src:  "package p\n\nlet a = 1 // one\n\n// end of file\n",
		want: "package p\n\nlet a = 1 // one\n\nlet b = {\n  x: [1]\n}\n\n// end of file\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, At: 1, Text: "let b = { x: [1] }"}}
		},
	},
	{
		name: "appendAmend",
		rule: "log-2026-09-29 M4 G1, API.md W11: a layer file without declarations gets one after its header",
		src:  "package p\nlayer dev\n",
		want: "package p\nlayer dev\n\namend config { port: 9000 }\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, Text: "amend config {port: 9000}"}}
		},
	},
	{
		name: "appendAfterImports",
		rule: "log-2026-09-29 M4 G1: after the imports when the file has no declaration, the M5 settle applying",
		src:  "package p\n\nimport a.b\n",
		want: "package p\n\nimport a.b\n\nlet longName: SomeType =\n  load(\"@resource/Server/Quest/adventure_quest_config_that_is_long_enough.json\")\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Insert, Text: `let longName: SomeType = load("@resource/Server/Quest/adventure_quest_config_that_is_long_enough.json")`}}
		},
	},
	{
		name: "removeBeforeKeptComma",
		rule: "log-2026-09-29 M4 G2, DECISIONS 216: the removed item takes the comma kept on its own line and the doc before it",
		src:  keptDoc,
		want: "package p\n\nenum E {\n  b\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindEnumMember, "a")}}
		},
	},
	{
		name: "removeBeforeKeptCommaComment",
		rule: "log-2026-09-29 M4 G2, DECISIONS 216: the removed item takes a \", /// x\" comma line",
		src:  keptAfter,
		want: "package p\n\nenum E {\n  b\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindEnumMember, "a")}}
		},
	},
	{
		name: "removeAfterKeptComma",
		rule: "log-2026-09-29 M4 G2, API.md M5, DECISIONS 216: removing b leaves the comma line, which the settle then drops: no item follows it",
		src:  keptDoc,
		want: "package p\n\nenum E {\n  a\n  /// d1\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindEnumMember, "b")}}
		},
	},
	{
		name: "removeAfterKeptCommaComment",
		rule: "log-2026-09-29 M4 G2: removing the item after a \", /// x\" comma line leaves that line alone",
		src:  keptAfter,
		want: "package p\n\nenum E {\n  a\n  , /// x\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindEnumMember, "b")}}
		},
	},
	{
		name: "removeKeywordNeighbour",
		rule: "FORMATTER.md §13 step 5 and DECISIONS 211: the comma kept for the removed item goes with it",
		src:  "package p\n\nenum E {\n  a,\n  in,\n  b\n}\n",
		want: "package p\n\nenum E {\n  a\n  b\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindEnumMember, "in")}}
		},
	},
	{
		name: "removeDeclaration",
		rule: "FORMATTER.md §13 step 5, API.md N6: a top-level declaration goes with its doc; no blank line is left at the end",
		src:  "package p\n\nentry t.a { x: 1 }\n\n/// b\nentry t.b { x: 2 }\n",
		want: "package p\n\nentry t.a { x: 1 }\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Remove, Node: named(t, f, syntax.KindEntryDecl, "entry t.b")}}
		},
	},
	{
		name: "retire",
		rule: "FORMATTER.md §13 step 6, API.md N7: \"retired \" before the entry keyword, the key or the member",
		src: "package p\n\nenum E { a = 1, b = 2 }\n\nlet t: stable table T = {\n  x { n: 1 }\n}\n\n" +
			"@deprecated(\"old\")\nentry t.y { n: 2 }\n",
		want: "package p\n\nenum E { a = 1, retired b = 2 }\n\nlet t: stable table T = {\n  retired x { n: 1 }\n}\n\n" +
			"@deprecated(\"old\")\nretired entry t.y { n: 2 }\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{
				{Kind: format.Retire, Node: named(t, f, syntax.KindEnumMember, "b")},
				{Kind: format.Retire, Node: named(t, f, syntax.KindEntryItem, "x")},
				{Kind: format.Retire, Node: named(t, f, syntax.KindEntryDecl, "@deprecated")},
			}
		},
	},
	{
		name: "newLists",
		rule: "FORMATTER.md §6.3: a new brace list is single-line if it fits and no item holds a list, else broken",
		src:  "package p\n\nlet c: C = {\n  a: 1\n  b: 2\n}\n",
		want: "package p\n\nlet c: C = {\n  a: { x: 1, y: 2 }\n  b: {\n    x: [1]\n  }\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{
				{Kind: format.Replace, Node: named(t, f, syntax.KindIntLit, "1"), Text: "{ x: 1, y: 2 }"},
				{Kind: format.Replace, Node: named(t, f, syntax.KindIntLit, "2"), Text: "{\n x: [1] }"},
			}
		},
	},
	{
		name: "settle",
		rule: "API.md M5: an item that is no longer a fixed point is re-printed, up to its declaration",
		src: "package p\n\nlet adventureQuests: AdventureQuestConfig =\n" +
			"  load(\"@resource/Server/Quest/adventure_quest_config.json\")\n",
		want: "package p\n\nlet adventureQuests: AdventureQuestConfig = load(\"a.json\")\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Replace, Node: named(t, f, syntax.KindStringLit, `"@`), Text: `"a.json"`}}
		},
	},
}

// FORMATTER.md §13: bytes outside the re-printed units never change, even out of layout.
func TestRewriteKeepsOtherBytes(t *testing.T) {
	src := "package p\n\nlet a   =   { x:1,y:2 }\nlet b = {\n  x:    1\n  y: 2\n}\n"
	want := "package p\n\nlet a   =   { x:1,y:2 }\nlet b = {\n  x:    1\n  y: 3\n}\n"
	in := parse(t, "a/a.canon", []byte(src))
	y := find(t, in.file, func(n syntax.Node) bool {
		fi, ok := n.(*syntax.FieldItem)
		return ok && fi.Name.Name == "y" && in.file.Tokens[fi.First()].Start > 30
	})
	got, err := format.Rewrite(in.file, []format.Change{{Kind: format.Replace, Node: y.(*syntax.FieldItem).Value, Text: "3"}})
	if err != nil || string(got) != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

// log-2026-09-29 M4 G2: removing the item after a kept comma leaves the comma line; in a file
// that is not a fixed point, no settle drops it afterwards.
func TestRemoveAfterKeptCommaKeepsIt(t *testing.T) {
	src := keptDoc + "\nconst   Z = 1\n"
	in := parse(t, "a/a.canon", []byte(src))
	got, err := format.Rewrite(in.file, []format.Change{{Kind: format.Remove, Node: named(t, in.file, syntax.KindEnumMember, "b")}})
	want := "package p\n\nenum E {\n  a\n  /// d1\n  ,\n}\n\nconst   Z = 1\n"
	if err != nil || string(got) != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

// refusal is one Rewrite of src that must fail with want.
type refusal struct {
	name, src string
	changes   func(t *testing.T, f *syntax.File) []format.Change
	want      error
}

// removeNamed removes the first node of kind k whose text starts with prefix.
func removeNamed(k syntax.NodeKind, prefix string) func(t *testing.T, f *syntax.File) []format.Change {
	return func(t *testing.T, f *syntax.File) []format.Change {
		return []format.Change{{Kind: format.Remove, Node: named(t, f, k, prefix)}}
	}
}

// insertText inserts text at position at of the first brace or list literal starting with prefix.
func insertText(k syntax.NodeKind, prefix string, at int, text string) func(t *testing.T, f *syntax.File) []format.Change {
	return func(t *testing.T, f *syntax.File) []format.Change {
		return []format.Change{{Kind: format.Insert, List: named(t, f, k, prefix).First(), At: at, Text: text}}
	}
}

// replaceNamed writes text over the first node of kind k whose text starts with prefix.
func replaceNamed(k syntax.NodeKind, prefix, text string) func(t *testing.T, f *syntax.File) []format.Change {
	return func(t *testing.T, f *syntax.File) []format.Change {
		return []format.Change{{Kind: format.Replace, Node: named(t, f, k, prefix), Text: text}}
	}
}

const (
	packed  = "package p\n\nlet c: C = {\n  a: 1, b: 2\n  c: 3\n}\n"
	header  = "package p\n\nimport a.b { X, Y }\n\nlet c: C = { a: 1 }\n"
	broken  = "package p\n\nlet c: C = {\n  a: 1\n}\n"
	inline  = "package p\n\nlet c: C = { a: 1 }\n"
	methods = "package p\n\nrecord R {\n  x: Int\n\n  fn f(self, a: Int) -> Int { return 1 }\n}\n"
)

var refusals = []refusal{
	{"packed remove", packed, removeNamed(syntax.KindFieldItem, "b"), format.ErrChange},
	{"packed insert", packed, insertText(syntax.KindBraceLit, "{\n", 1, "x: 9"), format.ErrChange},
	{"two items inline", inline, insertText(syntax.KindBraceLit, "{", 1, "a: 1, z: 5"), format.ErrText},
	{"two items broken", broken, insertText(syntax.KindBraceLit, "{", 1, "a: 1, z: 5"), format.ErrText},
	{"two items replace", broken, replaceNamed(syntax.KindFieldItem, "a", "a: 1, z: 5"), format.ErrText},
	{"line comment inline", inline, insertText(syntax.KindBraceLit, "{", 0, "z: 5 // c"), format.ErrText},
	{"line comment broken", broken, insertText(syntax.KindBraceLit, "{", 1, "z: 5 // c"), format.ErrText},
	{"not the kind", broken, replaceNamed(syntax.KindFieldItem, "a", "5"), format.ErrText},
	{"package name", header, replaceNamed(syntax.KindIdent, "p", "q"), format.ErrChange},
	{"import path", header, replaceNamed(syntax.KindQualifiedName, "a.b", "a.c"), format.ErrChange},
	{"import name", header, removeNamed(syntax.KindIdent, "X"), format.ErrChange},
	{"file", header, func(t *testing.T, f *syntax.File) []format.Change {
		return []format.Change{{Kind: format.Replace, Node: f, Text: "x"}}
	}, format.ErrChange},
	{"self", methods, removeNamed(syntax.KindParam, "a"), format.ErrChange},
	{"tab", "package p\n\nlet c: C = {\n\ta: 1\n}\n", removeNamed(syntax.KindFieldItem, "a"), format.ErrLayout},
	{"carriage return", "package p\n\n// a\r comment\nlet c: C = { a: 1 }\n", removeNamed(syntax.KindFieldItem, "a"), format.ErrLayout},
	{"lexer error", "package p\n\nlet c: C = { a: \"\\q\" }\n", removeNamed(syntax.KindFieldItem, "a"), format.ErrSyntax},
}

// log-2026-09-29 M4 U1r: Rewrite edits items only, never lets new text parse as something else,
// and refuses a list whose items share a line, a text it would fold, a tree with a lexer error.
func TestRewriteRefusesU1r(t *testing.T) {
	for _, c := range refusals {
		f := parse(t, "a/a.canon", []byte(c.src)).file
		if out, err := format.Rewrite(f, c.changes(t, f)); !errors.Is(err, c.want) || out != nil {
			t.Errorf("%s: got %q, %v; want %v", c.name, out, err, c.want)
		}
	}
	lexer := refusals[slices.IndexFunc(refusals, func(r refusal) bool { return r.want == format.ErrSyntax })]
	lexed := parse(t, "a/a.canon", []byte(lexer.src)).file
	if _, err := format.File(lexed); err != nil {
		t.Errorf("the lexer-error case must leave a whole tree (DECISIONS 166): %v", err)
	}
	sentinels := []error{format.ErrSyntax, format.ErrLayout, format.ErrChange, format.ErrText, format.ErrUnsettled}
	for i, e := range sentinels {
		for _, other := range sentinels[i+1:] {
			if errors.Is(e, other) {
				t.Errorf("%v wraps %v: callers could not tell them apart", e, other)
			}
		}
	}
}

// log-2026-09-29 M4 U1r: a block comment spanning lines after an item's last token is attached
// to what follows (DECISIONS 168), yet shares the item's line: no line can be written or removed
// there without cutting it, so an insert, a removal or a new declaration there is refused.
func TestRewriteAfterBlockComment(t *testing.T) {
	item := "package p\n\nlet c: C = {\n  a: 1 /* one\n    more */\n  b: 2\n}\n"
	decl := "package p\n\nlet a = 1 /* one\n  more */\n"
	cases := []struct {
		src    string
		change func(f *syntax.File) format.Change
	}{
		{item, func(f *syntax.File) format.Change {
			return format.Change{Kind: format.Insert, List: named(t, f, syntax.KindBraceLit, "{\n").First(), At: 1, Text: "x: 9"}
		}},
		{item, func(f *syntax.File) format.Change {
			return format.Change{Kind: format.Remove, Node: named(t, f, syntax.KindFieldItem, "a")}
		}},
		{decl, func(*syntax.File) format.Change { return format.Change{Kind: format.Insert, At: 1, Text: "let b = 2"} }},
	}
	for _, c := range cases {
		f := parse(t, "a/a.canon", []byte(c.src)).file
		if got, err := format.Rewrite(f, []format.Change{c.change(f)}); !errors.Is(err, format.ErrChange) {
			t.Errorf("got %q, %v; want ErrChange", got, err)
		}
	}
}

// FORMATTER.md §13: minimal re-printing of the units an edit touches.
func TestRewrite(t *testing.T) {
	for _, c := range append(rewriteCases, groupedCases...) {
		t.Run(c.name, c.run)
	}
}

// log-2026-09-29 M4 U1r: in a list on one line a comment goes only with the token the
// formatter attaches it to, and stays attached to it.
func TestRewriteKeepsInlineComments(t *testing.T) {
	for _, c := range commentCases {
		f := parse(t, "a/a.canon", []byte(c.src)).file
		comment, host, _ := strings.Cut(c.owner, "=>")
		if got := owner(t, f, comment); got != host {
			t.Errorf("%s: before, attached to %q, want %q", c.name, got, host)
		}
		got, err := format.Rewrite(f, c.changes(t, f))
		if err != nil || string(got) != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.rule, got, err, c.want)
			continue
		}
		if after := owner(t, parse(t, "a/a.canon", got).file, comment); after != host {
			t.Errorf("%s: after, attached to %q, want %q", c.name, after, host)
		}
	}
}

// FORMATTER.md §13, API.md §9: changes that do not apply are refused, the tree untouched.
func TestRewriteRefuses(t *testing.T) {
	in := parse(t, "a/a.canon", []byte(statuses))
	f := in.file
	lit := named(t, f, syntax.KindStringLit, `"Open"`)
	cases := []struct {
		name    string
		changes []format.Change
		want    error
	}{
		{"kind", []format.Change{{Kind: format.Retire + 1, Node: lit}}, format.ErrChange},
		{"no list", []format.Change{{Kind: format.Insert, List: lit.First(), Text: "x"}}, format.ErrChange},
		{"bad position", []format.Change{{Kind: format.Insert, List: listOf(t, f, syntax.KindListLit, "[open]"), At: 2, Text: "x"}}, format.ErrChange},
		{"not an item", []format.Change{{Kind: format.Remove, Node: lit}}, format.ErrChange},
		{"not retirable", []format.Change{{Kind: format.Retire, Node: lit}}, format.ErrChange},
		{"empty text", []format.Change{{Kind: format.Replace, Node: lit, Text: " "}}, format.ErrChange},
		{"overlap", []format.Change{{Kind: format.Replace, Node: lit, Text: "1"}, {Kind: format.Replace, Node: lit, Text: "2"}}, format.ErrChange},
		{"text", []format.Change{{Kind: format.Replace, Node: lit, Text: "1 +"}}, format.ErrText},
		{"nil node", []format.Change{{Kind: format.Retire, Node: (*syntax.EntryItem)(nil)}}, format.ErrChange},
	}
	for _, c := range cases {
		if out, err := format.Rewrite(f, c.changes); !errors.Is(err, c.want) || out != nil {
			t.Errorf("%s: got %q, %v; want %v", c.name, out, err, c.want)
		}
	}
	retired := parse(t, "a/a.canon", []byte("package p\n\nenum E { retired a }\n"))
	if _, err := format.Rewrite(retired.file, []format.Change{{Kind: format.Retire, Node: named(t, retired.file, syntax.KindEnumMember, "retired")}}); !errors.Is(err, format.ErrChange) {
		t.Errorf("retiring twice: %v, want ErrChange", err)
	}
	top := parse(t, "a/a.canon", []byte("package p\n\nlet a = 1\n")).file
	project := parse(t, "project.canon", []byte("project p {\n  version: \"0.1\"\n}\n")).file
	for _, c := range []struct {
		f  *syntax.File
		at int
	}{{top, 0}, {top, 2}, {project, 1}} {
		if _, err := format.Rewrite(c.f, []format.Change{{Kind: format.Insert, At: c.at, Text: "let b = 2"}}); !errors.Is(err, format.ErrChange) {
			t.Errorf("log-2026-09-29 M4 G1: a declaration at %d of %s: %v, want ErrChange", c.at, c.f.Src.Path, err)
		}
	}
	bad := parse(t, "a/a.canon", []byte("package p\n\nconst = 1\n"))
	if _, err := format.Rewrite(bad.file, nil); !errors.Is(err, format.ErrSyntax) {
		t.Errorf("a tree with a syntax error: %v, want ErrSyntax", err)
	}
}
