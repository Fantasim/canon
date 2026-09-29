package i18n_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// I18N.md K "v", F6: MenuLabel finds a let's `@menu(label: "…")` node; SourceText renders it in
// the one translation-file form. Exported so `views` reads the same node instead of its own
// walk of the annotation (VIEWMODEL.md N3).
func TestMenuLabelAndSourceText(t *testing.T) {
	fs := &source.FileSet{}
	src, err := fs.Add("m/m.canon", "/m/m.canon", []byte(`package m

@menu(label: "Potions {2}")
let potions: [Int] = []
`))
	if err != nil {
		t.Fatal(err)
	}
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, "m"))
	d := firstLet(t, f)

	s := i18n.MenuLabel(d)
	if s == nil {
		t.Fatal("MenuLabel: want a label, got none")
	}
	if got, want := i18n.SourceText(f, s), "Potions {2}"; got != want {
		t.Errorf("SourceText(MenuLabel(d)) = %q, want %q", got, want)
	}
}

// I18N.md K "v": a let with no `@menu(label:)` has no MenuLabel.
func TestMenuLabelNone(t *testing.T) {
	fs := &source.FileSet{}
	src, err := fs.Add("m/m.canon", "/m/m.canon", []byte(`package m

let potions: [Int] = []
`))
	if err != nil {
		t.Fatal(err)
	}
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, "m"))
	if s := i18n.MenuLabel(firstLet(t, f)); s != nil {
		t.Errorf("MenuLabel: want none, got %v", s)
	}
}

func firstLet(t *testing.T, f *syntax.File) *syntax.LetDecl {
	t.Helper()
	for _, decl := range f.Decls {
		if d, ok := decl.(*syntax.LetDecl); ok {
			return d
		}
	}
	t.Fatal("no let decl parsed")
	return nil
}
