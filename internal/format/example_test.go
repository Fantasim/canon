package format_test

import (
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Source prints a file in its canonical layout, and refuses one that does not parse.
func ExampleSource() {
	var fs source.FileSet
	good, _ := fs.Add("a/a.canon", "/p/a/a.canon", []byte("package a\nconst   Wait=90s    // a minute and a half\n"))
	out, err := format.Source(good, syntax.FileSource, diag.NewBag(&fs, "a"))
	bad, _ := fs.Add("a/b.canon", "/p/a/b.canon", []byte("package a\nconst = 1\n"))
	_, refused := format.Source(bad, syntax.FileSource, diag.NewBag(&fs, "a"))
	fmt.Printf("%q %v %v\n", out, err, errors.Is(refused, format.ErrSyntax))
	// Output: "package a\n\nconst Wait = 1m30s // a minute and a half\n" <nil> true
}

// File prints a tree already parsed without error.
func ExampleFile() {
	var fs source.FileSet
	src, _ := fs.Add("a/a.canon", "/p/a/a.canon", []byte("package a\nenum E { A,B, }\n"))
	out, err := format.File(syntax.Parse(src, syntax.FileSource, diag.NewBag(&fs, "a")))
	fmt.Printf("%q %v\n", out, err)
	// Output: "package a\n\nenum E { A, B }\n" <nil>
}

// Rewrite applies an edit's changes and re-prints only the items they touch.
func ExampleRewrite() {
	var fs source.FileSet
	src, _ := fs.Add("a/a.canon", "/p/a/a.canon", []byte("package a\n\nlet c: C = {\n  a: 1 // one\n  b: [1]\n}\n"))
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(&fs, "a"))
	c := f.Decls[0].(*syntax.LetDecl).Value.(*syntax.BraceLit)
	out, err := format.Rewrite(f, []format.Change{
		{Kind: format.Replace, Node: c.Items[0].(*syntax.FieldItem).Value, Text: "2"},
		{Kind: format.Insert, List: c.Items[1].(*syntax.FieldItem).Value.First(), At: 1, Text: "2"},
		{Kind: format.Insert, List: c.First(), At: 2, Text: "d: { x: 1 }"},
	})
	fmt.Printf("%q %v\n", out, err)
	// Output: "package a\n\nlet c: C = {\n  a: 2 // one\n  b: [1, 2]\n  d: { x: 1 }\n}\n" <nil>
}

// A Move carries an item with its comments to where an Insert at At would put a text.
func ExampleRewrite_move() {
	var fs source.FileSet
	src, _ := fs.Add("a/a.canon", "/p/a/a.canon", []byte("package a\n\nlet c: C = {\n  /// A.\n  a: 1 // one\n  b: 2\n}\n"))
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(&fs, "a"))
	c := f.Decls[0].(*syntax.LetDecl).Value.(*syntax.BraceLit)
	out, err := format.Rewrite(f, []format.Change{{Kind: format.Move, Node: c.Items[0], List: c.First(), At: 2}})
	fmt.Printf("%q %v\n", out, err)
	// Output: "package a\n\nlet c: C = {\n  b: 2\n  /// A.\n  a: 1 // one\n}\n" <nil>
}

// Node prints one node from a column; Flat prints it on one line.
func ExampleNode() {
	var fs source.FileSet
	src, _ := fs.Add("a/a.canon", "/p/a/a.canon", []byte("package a\n\nlet c = {\n  a: 1\n}\n"))
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(&fs, "a"))
	v := f.Decls[0].(*syntax.LetDecl).Value
	broken, _ := format.Node(f, v, format.Place{Indent: 2, Column: 5})
	flat, _ := format.Flat(f, v)
	fmt.Printf("%q %q\n", broken, flat)
	// Output: "{\n    a: 1\n  }" "{ a: 1 }"
}

// Fresh prints a file the edit API creates.
func ExampleFresh() {
	var fs source.FileSet
	src, _ := fs.Add("a/a.canon", "/p/a/a.canon", []byte("package a\nentry t.x { tags: [a], at: { x: 1 } }\n"))
	out, err := format.Fresh(syntax.Parse(src, syntax.FileSource, diag.NewBag(&fs, "a")))
	fmt.Printf("%q %v\n", out, err)
	// Output: "package a\n\nentry t.x {\n  tags: [a]\n  at: { x: 1 }\n}\n" <nil>
}
