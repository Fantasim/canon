package source_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/source"
)

// A file set normalizes "\r\n", numbers its files from 1 and resolves spans for display.
func Example() {
	var fs source.FileSet
	f, err := fs.Add("teamboard/taxonomy.canon", "/p/teamboard/taxonomy.canon", []byte("package teamboard\r\n\r\nconst VERSION = 7\r\n"))
	if err != nil {
		fmt.Println(err)
		return
	}
	s := source.Span{File: f.ID, Start: 25, End: 32}
	fmt.Printf("%d %q\n", f.ID, f.Content[s.Start:s.End])
	fmt.Printf("%+v\n", fs.Locate(s))
	// Output:
	// 1 "VERSION"
	// {Path:teamboard/taxonomy.canon Line:3 Col:7 EndLine:3 EndCol:14}
}
