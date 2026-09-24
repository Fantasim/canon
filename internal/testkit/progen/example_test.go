package progen_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// A project in memory is checked; a mutation site's finding starts in its focus edit.
func Example() {
	p := progen.NewProject()
	p.Set("project.canon", []byte("project p {\n  canon: \"0.1\"\n}\n"))
	p.Set("a/a.canon", []byte("package a\n\nlocal let x: Int = 1\n"))
	out := progen.Run(context.Background(), p, progen.RunOptions{Packages: []string{"a"}})
	fmt.Println(len(out.Findings), out.Err, out.Panic == "")

	site := progen.Site{Path: "a/a.canon", Edits: []progen.Edit{{Start: 30, End: 31, Text: `"one"`}}}
	m, err := site.Mutate(p)
	if err != nil {
		fmt.Println(err)
		return
	}
	out = progen.Run(context.Background(), m.Project, progen.RunOptions{Packages: []string{"a"}})
	fmt.Println(out.Findings[0], m.At.Start <= out.Findings[0].Start && out.Findings[0].Start <= m.At.End)
	// Output:
	// 0 <nil> true
	// E3002 a/a.canon:3:20 true
}
