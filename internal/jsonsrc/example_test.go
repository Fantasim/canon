package jsonsrc_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
)

// Parse locates every value by its span and RFC 6901 pointer; Format prints the tree in the
// canonical layout of JSON sources, numbers as written.
func Example() {
	var fs source.FileSet
	f, err := fs.Add("data/a.json", "/p/data/a.json", []byte(`{"a/b": [1.50, "\u00e9"], "m~n": {}}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	root, err := jsonsrc.Parse(f, diag.NewBag(&fs, "data"))
	if err != nil {
		fmt.Println(err)
		return
	}
	m := root.Members[1]
	fmt.Printf("%q at %+v, value %s\n", m.Key, fs.Locate(m.KeySpan), m.Value.Pointer())
	fmt.Printf("%s %q\n", root.Members[0].Value.Elems[1].Pointer(), jsonsrc.Format(root))
	// Output:
	// "m~n" at {Path:data/a.json Line:1 Col:27 EndLine:1 EndCol:32}, value /m~0n
	// /a~1b/1 "{\n  \"a/b\": [\n    1.50,\n    \"é\"\n  ],\n  \"m~n\": {}\n}\n"
}
