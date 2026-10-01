package format_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	projectText     = "project demo {\n  canon: \"0.1\"\n}\n"
	layerText       = "package a\nlayer louis\n\namend config {\n  a: 1\n}\n"
	translationText = "package a\ntranslation fr\n\nGlobal.price \"Prix\"\n"
)

// roleRun is a file's text judged in one role.
type roleRun struct {
	name string
	path string
	text string
	kind syntax.FileKind
}

// parsedAs is the text of c parsed as the project reads it: project.canon as a project, every
// other file as a source, whatever its text opens with.
func parsedAs(t *testing.T, c roleRun) *syntax.File {
	t.Helper()
	var fs source.FileSet
	src, err := fs.Add(c.path, "/"+c.path, []byte(c.text))
	if err != nil {
		t.Fatal(err)
	}
	kind := syntax.FileSource
	if c.path == projectFile {
		kind = syntax.FileProject
	}
	return syntax.Parse(src, kind, diag.NewBag(&fs, "p"))
}

// A file other than project.canon whose text opens with `project` is not a fixed point, and
// canon fmt agrees; a real project.canon still is one; a layer and a translation are judged as
// sources (API.md M9, DECISIONS 258).
func TestCanonicalInRole(t *testing.T) {
	for _, c := range []struct {
		roleRun
		fixed bool
		err   error
	}{
		{roleRun{"project.canon in its role", projectFile, projectText, syntax.FileProject}, true, nil},
		{roleRun{"a source opening with project", "a/a.canon", projectText, syntax.FileSource}, false, format.ErrSyntax},
		{roleRun{"a layer file", "a/l.canon", layerText, syntax.FileLayer}, true, nil},
		{roleRun{"a layer file as a source", "a/l.canon", layerText, syntax.FileSource}, true, nil},
		{roleRun{"a translation file", "a/t.canon", translationText, syntax.FileTranslation}, true, nil},
		{roleRun{"a translation file as a source", "a/t.canon", translationText, syntax.FileSource}, true, nil},
	} {
		fixed, err := format.Canonical(parsedAs(t, c.roleRun), c.kind)
		if fixed != c.fixed || !errors.Is(err, c.err) || (err == nil) != (c.err == nil) {
			t.Errorf("%s: Canonical %v, %v; want %v, %v", c.name, fixed, err, c.fixed, c.err)
		}
		agree(t, c.roleRun, c.err)
	}
}

// agree checks that canon fmt's entry point, Source, refuses exactly what Canonical refuses, with
// E1011 for a source opening with `project`.
func agree(t *testing.T, c roleRun, want error) {
	t.Helper()
	var fs source.FileSet
	src, err := fs.Add(c.path, "/"+c.path, []byte(c.text))
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(&fs, "p")
	out, err := format.Source(src, c.kind, bag)
	if !errors.Is(err, want) || (err == nil) != (want == nil) {
		t.Errorf("%s: Source gives %v, want %v", c.name, err, want)
	}
	reported := slices.ContainsFunc(bag.Findings(), func(fd diag.Finding) bool { return fd.Code == diag.E1011.Def().Code })
	if reported != (want != nil) {
		t.Errorf("%s: canon fmt reports the finding %v, want %v", c.name, reported, want != nil)
	}
	if want == nil && string(out) != c.text {
		t.Errorf("%s: Source prints %q", c.name, out)
	}
}

// The layout verdict of one tree is kept per role: asking in the other role first, or after,
// never carries a verdict across (API.md M9, DECISIONS 258).
func TestLayoutVerdictPerRole(t *testing.T) {
	for _, order := range [][]syntax.FileKind{
		{syntax.FileProject, syntax.FileSource},
		{syntax.FileSource, syntax.FileProject},
	} {
		f := parsedAs(t, roleRun{path: "a/a.canon", text: projectText})
		for range 2 { // the second round is answered from the memo
			askRoles(t, f, order)
		}
		for _, kind := range order {
			if !format.Judged(f, kind) {
				t.Errorf("kind %v: no verdict kept", kind)
			}
		}
	}
}

// askRoles asks Canonical of f in each role of order: a project is fixed, a source is refused.
func askRoles(t *testing.T, f *syntax.File, order []syntax.FileKind) {
	t.Helper()
	for _, kind := range order {
		fixed, err := format.Canonical(f, kind)
		want := kind == syntax.FileProject
		if fixed != want || (err == nil) != want {
			t.Errorf("order %v, kind %v: %v, %v", order, kind, fixed, err)
		}
	}
}

// Adopt in one role gives a tree no layout in the other (API.md M9, DECISIONS 258).
func TestAdoptPerRole(t *testing.T) {
	f := parsedAs(t, roleRun{path: "a/a.canon", text: projectText})
	format.Adopt(f, syntax.FileProject)
	if format.Judged(f, syntax.FileSource) {
		t.Fatal("a layout adopted as a project is held for a source")
	}
	if fixed, err := format.Canonical(f, syntax.FileSource); fixed || !errors.Is(err, format.ErrSyntax) {
		t.Errorf("as a source after Adopt as a project: %v, %v", fixed, err)
	}
}

// Rewrite refuses a source opening with `project`, as the fixed-point check does, and edits
// project.canon in its role (API.md M9, M5; DECISIONS 258).
func TestRewriteInRole(t *testing.T) {
	for _, c := range []struct {
		roleRun
		err error
	}{
		{roleRun{"a source opening with project", "a/a.canon", projectText, syntax.FileSource}, format.ErrSyntax},
		{roleRun{"project.canon", projectFile, projectText, syntax.FileProject}, nil},
	} {
		_, err := format.Rewrite(parsedAs(t, c.roleRun), c.kind, nil)
		if !errors.Is(err, c.err) || (err == nil) != (c.err == nil) {
			t.Errorf("%s: Rewrite gives %v, want %v", c.name, err, c.err)
		}
	}
}
