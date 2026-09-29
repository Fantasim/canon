package edit_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/edit"
)

// A commit writes an edit's files all or none, with '\n' line ends (API.md N10, N12).
func ExampleCommit() {
	d := newDiskFS(map[string]string{"/p/a.canon": "let a = 1\n"})
	site := edit.Site{FS: d, Layout: testLayout, Self: edit.Process{Host: "example", PID: 1}}
	changes := []edit.Change{{Kind: edit.ChangeModified, Path: "a.canon", Before: []byte("let a = 1\n"), After: []byte("let a = 2\r\n")}}
	err := edit.Commit(context.Background(), site, testRev, changes)
	fmt.Printf("%v %q %d\n", err, d.files["/p/a.canon"], len(journalsIn(d)))
	// Output: <nil> "let a = 2\n" 0
}

// Recover rolls back an edit its process left half done (API.md O5).
func ExampleRecover() {
	d := newDiskFS(map[string]string{"/p/a.canon": "let a = 1\n", "/p/b.canon": "let b = 1\n"})
	changes := []edit.Change{
		{Kind: edit.ChangeModified, Path: "a.canon", Before: []byte("let a = 1\n"), After: []byte("let a = 2\n")},
		{Kind: edit.ChangeModified, Path: "b.canon", Before: []byte("let b = 1\n"), After: []byte("let b = 2\n")},
	}
	dies := &faultFS{diskFS: d, dieAfterRename: 1}
	_ = edit.Commit(context.Background(), siteOf(dies), testRev, changes)
	fmt.Printf("%q %q\n", d.files["/p/a.canon"], d.files["/p/b.canon"])
	err := edit.Recover(siteOf(d), nil)
	fmt.Printf("%v %q %q\n", err, d.files["/p/a.canon"], d.files["/p/b.canon"])
	// Output: "let a = 2\n" "let b = 1\n"
	// <nil> "let a = 1\n" "let b = 1\n"
}
