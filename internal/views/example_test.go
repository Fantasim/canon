package views_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/views"
)

// Build writes a package's view model: here its `types` (VIEWMODEL.md 12.3), a public record
// with a documented field.
func Example() {
	fsys := mapFS{
		"law/project.canon": &fstest.MapFile{Data: []byte("project demo {\n  canon: \"0.1\"\n}\n")},
		"law/a/a.canon": &fstest.MapFile{Data: []byte(
			"package a\n\nrecord Pt {\n  /// Across.\n  x: Int(0..=9) = 1\n}\n")},
	}
	ctx := context.Background()
	p, err := build.Open(fsys, lawDir, build.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	a, err := p.Analyze(ctx, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	m, err := views.Build(ctx, views.Input{Program: a.Program(), Package: "a", Language: "0.1"})
	if err != nil {
		fmt.Println(err)
		return
	}
	out, err := json.Marshal(m.Types)
	fmt.Println(m.Schema, string(out), err)
	// Output:
	// canon-vm/1 {"a.Pt":{"kind":"record","name":"Pt","fields":[{"name":"x","type":{"kind":"int","bits":64,"signed":true,"min":0,"max":9},"help":"a:Pt.x.help","wire":{"name":"x"}}]}} <nil>
}
