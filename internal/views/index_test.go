package views_test

import (
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
)

// VIEWMODEL.md J4, G5: a second view of one record (E1607) is skipped, and gives no property.
func TestDuplicateViewGivesNothing(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": &fstest.MapFile{Data: []byte("project demo {\n  canon: \"0.1\"\n}\n")},
		"law/a/a.canon": &fstest.MapFile{Data: []byte(`package a

record Item {
  first: String
  second: String
}

view Item {
  first { control: textarea }
}

view Item {
  second { control: code }
}
`)},
	}
	x := analyze(t, fsys, lawDir, "", build.Options{})
	item := x.named(t, demoPkg, "Item")
	r := x.resolver()
	if got := r.Field(item, field(t, item, "first")).Kind; got != "textarea" {
		t.Errorf("first = %s, want the first view's textarea", got)
	}
	if got := r.Field(item, field(t, item, "second")).Kind; got != "input" {
		t.Errorf("second = %s, want input: the duplicate view gives nothing", got)
	}
}

// I18N.md K7, VIEWMODEL.md J9: a parent view in package a gives b's case field a `help` text,
// but only b's own texts are keys of b's catalogue, so b's model writes no help for it.
func TestCrossPackageHelpIsNotAKey(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": &fstest.MapFile{Data: []byte("project demo {\n  canon: \"0.1\"\n}\n")},
		"law/b/b.canon": &fstest.MapFile{Data: []byte(`package b

variant Shape {
  circle { r: Int }
  square { side: Int }
}
`)},
		"law/a/a.canon": &fstest.MapFile{Data: []byte(`package a

import b { Shape }

record Holder {
  shape: Shape @json(inline)
}

view Holder {
  r "Radius" { help: "How round" }
}
`)},
	}
	x := analyze(t, fsys, lawDir, "", build.Options{})
	//canon:unordered any error fails the test, whichever package holds it
	for _, b := range x.bags {
		if b.Summary().Errors > 0 {
			t.Fatalf("errors: %v", b.Findings())
		}
	}
	shape := x.model(t, "b").Types["b.Shape"]
	if h := shape.Cases[0].Fields[0].Help; h.Key != "" || h.Text != nil {
		t.Errorf("b.Shape.circle.r help = %s, want none: a's text is no key of b", text(h))
	}
}

// VIEWMODEL.md G4, J4: a view in package b of a record of a (E1603) is skipped, and gives a's
// fields nothing.
func TestForeignViewGivesNothing(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": &fstest.MapFile{Data: []byte("project demo {\n  canon: \"0.1\"\n}\n")},
		"law/a/a.canon":     &fstest.MapFile{Data: []byte("package a\n\nrecord Item {\n  name: String\n}\n")},
		"law/b/b.canon": &fstest.MapFile{Data: []byte(
			"package b\n\nimport a { Item }\n\nview Item {\n  name { control: textarea }\n}\n")},
	}
	x := analyze(t, fsys, lawDir, "", build.Options{})
	item := x.named(t, demoPkg, "Item")
	if got := x.resolver().Field(item, field(t, item, "name")).Kind; got != "input" {
		t.Errorf("name = %s, want input: b's view of a.Item gives nothing", got)
	}
}
