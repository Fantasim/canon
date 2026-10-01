package format_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

// importCommentCases are imports whose comments must stay on their token while the names and the
// imports are sorted; the first is FuzzFormat's 5f18ea43a28949f9.
var importCommentCases = []struct{ name, in, want string }{
	{"trailsNameSortedLast", "package A\nimport A{b//\nA}", "package A\n\nimport A {\n  A\n  b //\n}\n"},
	{"trailsOpenBrace", "package A\nimport A{//\nb\nA}", "package A\n\nimport A { //\n  A\n  b\n}\n"},
	{"trailsDroppedComma", "package A\nimport A{b,//\nA}", "package A\n\nimport A {\n  A\n  b //\n}\n"},
	{"leadsAfterCommaLine", "package A\nimport A{b\n,/**/A}", "package A\n\nimport A {\n  /**/\n  A\n  b\n}\n"},
	{"trailsImportSortedLast", "package A\nimport B{x}//\nimport A{y}\nconst c = 1", "package A\n\nimport A { y }\nimport B { x } //\n\nconst c = 1\n"},
	{"leadsImportSortedLast", "package A\n// c\nimport B{x}\nimport A{y}", "package A\n\nimport A { y }\n// c\nimport B { x }\n"},
}

// FORMATTER.md §8.1, §9.1: an import-block comment moves with the token it is attached to.
func TestImportCommentsStayOnTheirToken(t *testing.T) {
	for _, c := range importCommentCases {
		in := parse(t, "a/a.canon", []byte(c.in))
		out := mustFile(t, in.file)
		if string(out) != c.want {
			t.Fatalf("%s:\n%s", c.name, lineDiff([]byte(c.want), out))
		}
		checkFormatted(t, "a/a.canon", in, out)
	}
}

// importCommentMoves are two layouts of one import the comment check must tell apart.
var importCommentMoves = []struct{ name, before, after string }{
	{"toAnotherName", "package A\n\nimport A {\n  A // c\n  b\n}\n", "package A\n\nimport A {\n  A\n  b // c\n}\n"},
	{"swappedOnOneName", "package A\n\nimport A {\n  // x\n  // y\n  A\n}\n", "package A\n\nimport A {\n  // y\n  // x\n  A\n}\n"},
}

// FORMATTER.md §8.1: the comment check sees a comment moved to another name or reordered.
func TestImportCommentCheckSeesAMove(t *testing.T) {
	for _, c := range importCommentMoves {
		_, before := comments(parse(t, "a/a.canon", []byte(c.before)).file)
		_, after := comments(parse(t, "a/a.canon", []byte(c.after)).file)
		if slices.Equal(before, after) {
			t.Errorf("%s: unseen: %q", c.name, before)
		}
	}
}

// removeTopCases remove a top-level declaration holding and followed by trailing comments;
// the first is FuzzRewrite's 10088c38267378e7.
var removeTopCases = []struct{ name, in, want string }{
	{
		"noBlankLines",
		"package p\n\nenum E { a }\n// lead\nrecord R { // open\n  a: Int // field\n} // close\nconst Z = 1\n",
		"package p\n\nenum E { a }\nconst Z = 1\n",
	},
	{
		"blankLines",
		"package p\n\nenum E { a }\n\n// lead\nrecord R {\n  a: Int\n} // close\n\nconst Z = 1\n",
		"package p\n\nenum E { a }\n\nconst Z = 1\n",
	},
	{
		"lastDeclaration",
		"package p\n\nenum E { a }\n\nrecord R { a: Int } // close\n",
		"package p\n\nenum E { a }\n",
	},
	{
		"nextKeepsItsComment",
		"package p\n\nenum E { a }\nrecord R { a: Int } // close\nconst Z = 1 // z\n",
		"package p\n\nenum E { a }\nconst Z = 1 // z\n",
	},
	{
		"nextKeepsItsCommentBlankLines",
		"package p\n\nenum E { a }\n\nrecord R { a: Int } // close\n\nconst Z = 1 // z\n",
		"package p\n\nenum E { a }\n\nconst Z = 1 // z\n",
	},
}

// FORMATTER.md §13 step 5, API.md M4, M6: a removed declaration takes its trailing comment.
func TestRemoveDeclarationTakesItsTrailingComment(t *testing.T) {
	for _, c := range removeTopCases {
		ex := example{path: "a/a.canon", data: []byte(c.in)}
		f := parse(t, ex.path, ex.data).file
		if out := mustFile(t, f); string(out) != c.in {
			t.Fatalf("%s: the input is not canonical:\n%s", c.name, lineDiff(ex.data, out))
		}
		n := find(t, f, func(n syntax.Node) bool { return n.Kind() == syntax.KindRecordDecl })
		change := format.Change{Kind: format.Remove, Node: n}
		if err := checkRewrite(t, ex, f, n, change); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got, err := format.Rewrite(f, f.FileKind, []format.Change{change})
		if err != nil || string(got) != c.want {
			t.Fatalf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}
