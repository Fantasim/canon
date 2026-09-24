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
