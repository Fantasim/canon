package views_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/views"
)

// Build writes a package's view model: here the view of a public record with a documented field
// (VIEWMODEL.md 12.4, C12), its texts read from the package's catalogue (i18n.Check, J9).
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
	bags := check.Bags{"a": a.Bag("a")}
	texts := i18n.Check(a.Program(), project.New("demo", project.SupportedVersions()[0]), bags, nil)
	m, err := views.Build(ctx, views.Input{Program: a.Program(), Package: "a", Language: "0.1", I18N: texts})
	if err != nil {
		fmt.Println(err)
		return
	}
	out, err := json.Marshal(m.Views["a.Pt"].Fields["x"])
	fmt.Println(m.Schema, string(out), err)
	// Output:
	// canon-vm/1 {"label":"a:Pt.x","help":"a:Pt.x.help","control":{"kind":"number","min":0,"max":9,"stepper":true}} <nil>
}
