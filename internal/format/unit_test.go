package format_test

import (
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

// FORMATTER.md §13 step 1: a node printed alone from a column, the rest of its line counted.
func TestNode(t *testing.T) {
	src := "package p\n\nlet c: C = {\n  // lead\n  a: { x: 1, y: [1, 2] } // trail\n  b: {\n    x: 1\n  }\n}\n"
	f := parse(t, "a/a.canon", []byte(src)).file
	a, b := named(t, f, syntax.KindFieldItem, "a"), named(t, f, syntax.KindFieldItem, "b")
	cases := []struct {
		rule string
		n    syntax.Node
		at   format.Place
		want string
	}{
		{"§13 step 1: fits on its line", a, format.Place{Indent: 2, Column: 2}, "a: { x: 1, y: [1, 2] }"},
		{"§13 step 1, §7.3: the rest counts", a, format.Place{Indent: 2, Column: 2, Rest: 80},
			"a: {\n    x: 1\n    y: [1, 2]\n  }"},
		{"§13 step 1, §6.1: a broken list stays broken", b, format.Place{Indent: 4, Column: 10}, "b: {\n      x: 1\n    }"},
	}
	for _, c := range cases {
		got, err := format.Node(f, c.n, c.at)
		if err != nil || string(got) != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.rule, got, err, c.want)
		}
	}
	for _, at := range []format.Place{{Rest: -1}, {Column: -1}, {Indent: -1}} {
		if _, err := format.Node(f, a, at); !errors.Is(err, format.ErrPlace) {
			t.Errorf("log-2026-09-29 M4 U1r: %+v: %v, want ErrPlace", at, err)
		}
	}
}

// API.md E26: the canonical single-line text of a value, whatever its width; a node a line
// cannot hold is refused.
func TestFlat(t *testing.T) {
	src := "package p\n\nlet c: C = {\n  a: [\n    1,\n    2,\n  ]\n  b: {\n    x: 90s\n  }\n  // own line\n  d: 1 // trailing\n}\n\n" +
		"let s = \"\"\"\n  text\n  \"\"\"\n"
	f := parse(t, "a/a.canon", []byte(src)).file
	got, err := format.Flat(f, named(t, f, syntax.KindFieldItem, "b"))
	if err != nil || string(got) != "b: { x: 1m30s }" {
		t.Errorf("a broken brace list: got %q, %v", got, err)
	}
	got, err = format.Flat(f, named(t, f, syntax.KindListLit, "["))
	if err != nil || string(got) != "[1, 2]" {
		t.Errorf("a broken list: got %q, %v", got, err)
	}
	for _, n := range []syntax.Node{named(t, f, syntax.KindBraceLit, "{"), named(t, f, syntax.KindStringLit, `"""`)} {
		if out, err := format.Flat(f, n); !errors.Is(err, format.ErrLines) {
			t.Errorf("%s: got %q, %v; want ErrLines", n.Kind(), out, err)
		}
	}
	fn := parse(t, "a/a.canon", []byte("package p\n\nfn g(x: Int) -> Int {\n  let y = x\n  return y\n}\n")).file
	if _, err := format.Flat(fn, named(t, fn, syntax.KindFnDecl, "fn")); !errors.Is(err, format.ErrLines) {
		t.Errorf("a block of two statements (§6.1): %v, want ErrLines", err)
	}
	doc := parse(t, "a/a.canon", []byte("package p\n\n/// d\nconst X = 1\n")).file
	if _, err := format.Flat(doc, doc.Decls[0].(*syntax.ConstDecl).Doc); !errors.Is(err, format.ErrNode) {
		t.Errorf("a doc comment: %v, want ErrNode", err)
	}
	bad := parse(t, "a/a.canon", []byte("package p\n\nconst X = (1 +)\n")).file
	if _, err := format.Flat(bad, named(t, bad, syntax.KindConstDecl, "const")); !errors.Is(err, format.ErrSyntax) {
		t.Errorf("a node that did not parse: %v, want ErrSyntax", err)
	}
	if _, err := format.Flat(bad, (*syntax.ConstDecl)(nil)); !errors.Is(err, format.ErrSyntax) {
		t.Errorf("a nil node: %v, want ErrSyntax", err)
	}
	if _, err := format.Node(bad, named(t, bad, syntax.KindConstDecl, "const"), format.Place{}); !errors.Is(err, format.ErrSyntax) {
		t.Errorf("Node of a node that did not parse: %v, want ErrSyntax", err)
	}
}

// FORMATTER.md §13 step 7 and §6.3: a file the edit API creates has no layout to keep.
func TestFresh(t *testing.T) {
	src := "package p\n\nentry t.a { name: \"A\", tags: [x], size: { w: 1, h: 2 } }\n\nentry t.b {\n  name: \"B\"\n}\n"
	want := "package p\n\nentry t.a {\n  name: \"A\"\n  tags: [x]\n  size: { w: 1, h: 2 }\n}\n\nentry t.b { name: \"B\" }\n"
	got, err := format.Fresh(parse(t, "a/a.canon", []byte(src)).file)
	if err != nil || string(got) != want {
		t.Fatalf("got %v\n%s", err, lineDiff([]byte(want), got))
	}
	if _, err := format.Fresh(parse(t, "a/a.canon", []byte("const X = 1\n")).file); !errors.Is(err, format.ErrSyntax) {
		t.Errorf("a file without package: %v, want ErrSyntax", err)
	}
}
