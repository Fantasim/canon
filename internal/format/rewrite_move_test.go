package format_test

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

// commented is a broken brace list whose item b carries a leading, a doc and a trailing comment.
const commented = "package p\n\nrecord R {\n  a: Int\n\n  // about b\n  /// B.\n  b: Int // bee\n  c: Int\n}\n"

// brokenLit and oneLineLit are brace literals whose item b moves (log-2026-09-29 M4 U1b-r).
const (
	brokenLit  = "package p\n\nlet c: C = {\n  a: 1\n  b: 2 // bee\n  c: 3\n}\n"
	oneLineLit = "package p\n\nlet c: C = { a: 1, b: 2, c: 3 }\n"
)

// keywords is a broken enum holding DECISIONS 211's comma around "in".
const keywords = "package p\n\nenum E {\n  a,\n  in,\n  b\n}\n"

// moveOf moves the first node of kind k starting with prefix to position at of its list, which
// the first node of kind lk starting with list opens.
func moveOf(k syntax.NodeKind, prefix string, lk syntax.NodeKind, list string, at int) func(t *testing.T, f *syntax.File) []format.Change {
	return func(t *testing.T, f *syntax.File) []format.Change {
		return []format.Change{{Kind: format.Move, Node: named(t, f, k, prefix), List: listOf(t, f, lk, list), At: at}}
	}
}

// memberMove moves member prefix of the file's first enum to position at.
func memberMove(prefix string, at int) func(t *testing.T, f *syntax.File) []format.Change {
	return func(t *testing.T, f *syntax.File) []format.Change {
		open := named(t, f, syntax.KindEnumDecl, "enum").(*syntax.EnumDecl).Braces.Open
		return []format.Change{{Kind: format.Move, Node: named(t, f, syntax.KindEnumMember, prefix), List: open, At: at}}
	}
}

// fieldMove moves field prefix of the file's first record to position at.
func fieldMove(prefix string, at int) func(t *testing.T, f *syntax.File) []format.Change {
	return func(t *testing.T, f *syntax.File) []format.Change {
		open := named(t, f, syntax.KindRecordDecl, "record").(*syntax.RecordDecl).Body.First()
		return []format.Change{{Kind: format.Move, Node: named(t, f, syntax.KindFieldDecl, prefix), List: open, At: at}}
	}
}

// argMove moves argument prefix of the file's first call to position at.
func argMove(prefix string, at int) func(t *testing.T, f *syntax.File) []format.Change {
	return func(t *testing.T, f *syntax.File) []format.Change {
		open := named(t, f, syntax.KindCallExpr, "f").(*syntax.CallExpr).Parens.Open
		return []format.Change{{Kind: format.Move, Node: named(t, f, syntax.KindArg, prefix), List: open, At: at}}
	}
}

// moveCases are FORMATTER.md §13's Moves, steps 5 then 4, comments carried (log-2026-09-29 U1b).
var moveCases = []rewriteCase{
	{
		name:    "moveBrokenToEnd",
		rule:    "FORMATTER.md §8.1, §13 steps 4-5: b's lines leave with its //, /// and trailing comments and arrive last, the blank line before it kept once",
		src:     commented,
		want:    "package p\n\nrecord R {\n  a: Int\n\n  c: Int\n  // about b\n  /// B.\n  b: Int // bee\n}\n",
		changes: fieldMove("b", 3),
	},
	{
		name:    "moveBrokenToStart",
		rule:    "FORMATTER.md §8.1, §13 step 4: b arrives first, after the opening bracket, without a blank line",
		src:     commented,
		want:    "package p\n\nrecord R {\n  // about b\n  /// B.\n  b: Int // bee\n  a: Int\n\n  c: Int\n}\n",
		changes: fieldMove("b", 0),
	},
	{
		name:    "moveBrokenBack",
		rule:    "FORMATTER.md §13 step 4: c arrives between a and b, at the insertion point after a's lines",
		src:     commented,
		want:    "package p\n\nrecord R {\n  a: Int\n  c: Int\n\n  // about b\n  /// B.\n  b: Int // bee\n}\n",
		changes: fieldMove("c", 1),
	},
	{
		name:    "moveKeywordLast",
		rule:    "FORMATTER.md §13, DECISIONS 211: commas over the final order; \"in\" moved last loses its comma, b before it gains one",
		src:     keywords,
		want:    "package p\n\nenum E {\n  a\n  b,\n  in\n}\n",
		changes: memberMove("in", 3),
	},
	{
		name:    "moveBeforeKeyword",
		rule:    "FORMATTER.md §13, DECISIONS 211: a moved after \"in\" loses the comma it had before \"in\"",
		src:     keywords,
		want:    "package p\n\nenum E {\n  in,\n  b\n  a\n}\n",
		changes: memberMove("a", 3),
	},
	{
		name:    "moveBeforeKeptComma",
		rule:    "DECISIONS 216: c arrives after a's kept comma line, which still keeps /// d1 from documenting c",
		src:     "package p\n\nenum E {\n  a\n  /// d1\n  ,\n  b\n  c\n}\n",
		want:    "package p\n\nenum E {\n  a\n  /// d1\n  ,\n  c\n  b\n}\n",
		changes: memberMove("c", 1),
	},
	{
		name:    "moveKeptCommaItem",
		rule:    "DECISIONS 216, API.md M5: a leaves with its kept comma line; last, the comma guards nothing and the settle drops it",
		src:     keptDoc,
		want:    "package p\n\nenum E {\n  b\n  a\n  /// d1\n}\n",
		changes: memberMove("a", 2),
	},
	{
		name:    "moveKeptCommaComment",
		rule:    "DECISIONS 216: b goes first; a keeps its \", /// x\" line, a comma kept whatever follows it",
		src:     keptAfter,
		want:    "package p\n\nenum E {\n  b\n  a\n  , /// x\n}\n",
		changes: memberMove("b", 0),
	},
	{
		name: "moveBrokenBracket",
		rule: "FORMATTER.md §6.2, §13 step 4: in a broken [ ] list the moved element keeps its comma and its comments",
		src: "package p\n\nlet xs: [String] = [\n  // first\n  \"a long string that makes the list break, and then some more text, and more, and more\",\n" +
			"  \"b\", // bee\n  \"c\",\n]\n",
		want: "package p\n\nlet xs: [String] = [\n  \"b\", // bee\n  // first\n  \"a long string that makes the list break, and then some more text, and more, and more\",\n" +
			"  \"c\",\n]\n",
		changes: moveOf(syntax.KindStringLit, `"b"`, syntax.KindListLit, "[", 0),
	},
	{
		name:    "moveInlineBrace",
		rule:    "FORMATTER.md §6.1, §13 step 4: in a one-line brace list, which holds no comment, b goes last",
		src:     "package p\n\nlet c: C = { a: 1, b: 2, c: 3 }\n",
		want:    "package p\n\nlet c: C = { a: 1, c: 3, b: 2 }\n",
		changes: moveOf(syntax.KindFieldItem, "b", syntax.KindBraceLit, "{", 3),
	},
	{
		name:    "moveInlineBracketFirst",
		rule:    "FORMATTER.md §8.1, DECISIONS 167, §13 step 4: 2 goes first with its trailing comment, written after its comma again",
		src:     "package p\n\nlet c = [1, 2, /* two */ 3, 4]\n",
		want:    "package p\n\nlet c = [2, /* two */ 1, 3, 4]\n",
		changes: moveOf(syntax.KindIntLit, "2", syntax.KindListLit, "[", 0),
	},
	{
		name:    "moveInlineBracketLast",
		rule:    "FORMATTER.md §8.1, §13 step 4: 2 goes last with its trailing comment",
		src:     "package p\n\nlet c = [1, 2, /* two */ 3, 4]\n",
		want:    "package p\n\nlet c = [1, 3, 4, 2 /* two */ ]\n",
		changes: moveOf(syntax.KindIntLit, "2", syntax.KindListLit, "[", 4),
	},
	{
		name:    "moveArgument",
		rule:    "FORMATTER.md §13 step 4: an argument of a ( ) list moves last",
		src:     "package p\n\nlet c = f(1, 2, 3)\n",
		want:    "package p\n\nlet c = f(2, 3, 1)\n",
		changes: argMove("1", 3),
	},
	{
		name: "moveEscalates",
		rule: "FORMATTER.md §13 step 3: a one-line list a move and an insert leave past 100 columns re-prints the item holding it, broken",
		src:  "package p\n\nlet c: C = { a: [1, 2], b: \"a string long enough to reach the width\", c: \"and a little more text\" }\n",
		want: "package p\n\nlet c: C = {\n  b: \"a string long enough to reach the width\"\n  c: \"and a little more text\"\n  a: [1, 2]\n  d: 1234567890\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			l := listOf(t, f, syntax.KindBraceLit, "{")
			return []format.Change{
				{Kind: format.Move, Node: named(t, f, syntax.KindFieldItem, "a"), List: l, At: 3},
				{Kind: format.Insert, List: l, At: 3, Text: "d: 1234567890"},
			}
		},
	},
	{
		name: "moveWithInsertAndRemove",
		rule: "log-2026-09-29 M4 U1r, U1b: a Move, an Insert and a Remove of one list planned once over its final items",
		src:  "package p\n\nrecord R {\n  a: Int\n  b: Int // bee\n  c: Int\n  d: Int\n}\n",
		want: "package p\n\nrecord R {\n  c: Int\n  d: Int\n  b: Int // bee\n  x: Int\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			open := named(t, f, syntax.KindRecordDecl, "record").(*syntax.RecordDecl).Body.First()
			return []format.Change{
				{Kind: format.Remove, Node: named(t, f, syntax.KindFieldDecl, "a")},
				{Kind: format.Move, Node: named(t, f, syntax.KindFieldDecl, "b"), List: open, At: 4},
				{Kind: format.Insert, List: open, At: 4, Text: "x: Int"},
			}
		},
	},
	{
		name: "moveBesideReplaces",
		rule: "log-2026-09-29 M4 U1b-r: Replaces on the kept siblings before and after a moved item apply with it",
		src:  brokenLit,
		want: "package p\n\nlet c: C = {\n  a: 9\n  c: 7\n  b: 2 // bee\n}\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			return append(moveOf(syntax.KindFieldItem, "b", syntax.KindBraceLit, "{", 3)(t, f),
				format.Change{Kind: format.Replace, Node: named(t, f, syntax.KindIntLit, "1"), Text: "9"},
				format.Change{Kind: format.Replace, Node: named(t, f, syntax.KindIntLit, "3"), Text: "7"})
		},
	},
	{
		name: "moveWithInsertAndRemoveInline",
		rule: "log-2026-09-29 M4 U1r, U1b: in a one-line list, remove 1, move 4 before 2, insert 9 before 3",
		src:  "package p\n\nlet c = [1, 2, 3, 4]\n",
		want: "package p\n\nlet c = [4, 2, 9, 3]\n",
		changes: func(t *testing.T, f *syntax.File) []format.Change {
			l := listOf(t, f, syntax.KindListLit, "[")
			return []format.Change{
				{Kind: format.Remove, Node: named(t, f, syntax.KindIntLit, "1")},
				{Kind: format.Move, Node: named(t, f, syntax.KindIntLit, "4"), List: l, At: 1},
				{Kind: format.Insert, List: l, At: 2, Text: "9"},
			}
		},
	},
}

// FORMATTER.md §8.1, §13 (log-2026-09-29 M4 U1b): every comment keeps its token across a Move.
func TestRewriteMove(t *testing.T) {
	for _, c := range moveCases {
		t.Run(c.name, func(t *testing.T) {
			c.run(t)
			before := parse(t, "a/a.canon", []byte(c.src)).file
			after := parse(t, "a/a.canon", []byte(c.want)).file
			if lost := lostComments(before, after, nil); len(lost) > 0 {
				t.Errorf("%s: comments lost or attached elsewhere: %q", c.rule, lost)
			}
		})
	}
}

// FORMATTER.md §13 step 3, API.md M5: moveEscalates escalates in a file the settle skips.
func TestRewriteMoveEscalatesUnsettled(t *testing.T) {
	c := moveCases[slices.IndexFunc(moveCases, func(c rewriteCase) bool { return c.name == "moveEscalates" })]
	const outOfLayout = "\nconst   Z = 1\n"
	f := parse(t, "a/a.canon", []byte(c.src+outOfLayout)).file
	got, err := format.Rewrite(f, f.FileKind, c.changes(t, f))
	if err != nil || string(got) != c.want+outOfLayout {
		t.Errorf("%s: got %q, %v; want %q", c.rule, got, err, c.want+outOfLayout)
	}
}

// log-2026-09-29 M4 U1b: a Move to its own place, before or after itself, changes nothing.
func TestRewriteMoveInPlace(t *testing.T) {
	for _, at := range []int{1, 2} {
		f := parse(t, "a/a.canon", []byte(commented)).file
		got, err := format.Rewrite(f, f.FileKind, fieldMove("b", at)(t, f))
		if err != nil || !bytes.Equal(got, []byte(commented)) {
			t.Errorf("a move of b to %d: got %q, %v; want the input", at, got, err)
		}
	}
}

// log-2026-09-29 M4 U1b: a Move stays in its list, at a position of it, once per item; an item
// whose comment ends its line cannot join another on one line.
func TestRewriteMoveRefuses(t *testing.T) {
	both := func(second format.ChangeKind) func(t *testing.T, f *syntax.File) []format.Change {
		return func(t *testing.T, f *syntax.File) []format.Change {
			cs := fieldMove("b", 0)(t, f)
			return append(cs, format.Change{Kind: second, Node: cs[0].Node, List: cs[0].List, At: 3})
		}
	}
	inside := func(list string) func(t *testing.T, f *syntax.File) []format.Change {
		return func(t *testing.T, f *syntax.File) []format.Change {
			return append(moveOf(syntax.KindFieldItem, "b", syntax.KindBraceLit, list, 3)(t, f),
				format.Change{Kind: format.Replace, Node: named(t, f, syntax.KindIntLit, "2"), Text: "5"})
		}
	}
	for _, c := range []refusal{
		{"replace inside a moved item, broken", brokenLit, inside("{\n"), format.ErrChange},
		{"replace inside a moved item, one line", oneLineLit, inside("{"), format.ErrChange},
		{"past the end", commented, fieldMove("b", 4), format.ErrChange},
		{"before the start", commented, fieldMove("b", -1), format.ErrChange},
		{"moved twice", commented, both(format.Move), format.ErrChange},
		{"moved and removed", commented, both(format.Remove), format.ErrChange},
		{"another list", "package p\n\nlet c = { a: [1], b: [2, 3] }\n",
			moveOf(syntax.KindIntLit, "1", syntax.KindListLit, "[2", 0), format.ErrChange},
		{"not an item", commented, func(t *testing.T, f *syntax.File) []format.Change {
			cs := fieldMove("b", 0)(t, f)
			cs[0].Node = named(t, f, syntax.KindIdent, "b")
			return cs
		}, format.ErrChange},
		{"declaration", "package p\n\nlet a = 1\n\nlet b = 2\n", func(t *testing.T, f *syntax.File) []format.Change {
			return []format.Change{{Kind: format.Move, Node: named(t, f, syntax.KindLetDecl, "let b"), At: 0}}
		}, format.ErrChange},
		{"line comment inline", "package p\n\nlet c = f(a // c\n  , b)\n",
			argMove("a", 2), format.ErrChange},
	} {
		f := parse(t, "a/a.canon", []byte(c.src)).file
		if out, err := format.Rewrite(f, f.FileKind, c.changes(t, f)); !errors.Is(err, c.want) || out != nil {
			t.Errorf("%s: got %q, %v; want %v", c.name, out, err, c.want)
		}
	}
}
