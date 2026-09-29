package check_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// checkKeepingBags is checkFile, but it also returns the package bag (pre-created and parsed
// into, as a build does, so a parse-time finding reaches check.Check's own syntaxHeld/badLits,
// DECISIONS 214), so a test can Truncate it.
func checkKeepingBags(t *testing.T, src string) (*check.Program, *syntax.File, *diag.Bag) {
	t.Helper()
	fs := &source.FileSet{}
	bag := diag.NewBag(fs, "a")
	f, err := fs.Add("a/a.canon", "/a/a.canon", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	file := syntax.Parse(f, syntax.FileSource, bag)
	bags := check.Bags{"a": bag}
	prog := check.Check(context.Background(), exampleProject(), []*syntax.File{file}, bags, literalFolder{})
	return prog, file, bags["a"]
}

// VIEWMODEL.md J4: a broken view (two unresolved names) stays broken under Truncate(1) too.
func TestViewBrokenIndependentOfTruncation(t *testing.T) {
	prog, file, bag := checkKeepingBags(t, `package a

record Thing {
  name: String
}

view Thing {
  title "{missing1} {missing2}"
}

record Other {
  name: String
}

view Other {
  title "{name}"
}
`)
	v := checkedViews{prog: prog, files: []*syntax.File{file}}
	broken, intact := v.view("Thing"), v.view("Other")
	if broken == nil || intact == nil {
		t.Fatalf("views not found: Thing=%v Other=%v", broken, intact)
	}
	if len(bag.Findings()) < 2 {
		t.Fatalf("want at least 2 findings before truncation, got %d", len(bag.Findings()))
	}
	if !check.ViewBroken(prog.Info, broken) {
		t.Error("view Thing: want broken (two unresolved names in its title)")
	}
	if check.ViewBroken(prog.Info, intact) {
		t.Error("view Other: want not broken")
	}
	bag.Truncate(1)
	if len(bag.Findings()) != 1 {
		t.Fatalf("Truncate(1): want 1 finding, got %d", len(bag.Findings()))
	}
	if !check.ViewBroken(prog.Info, broken) {
		t.Error("view Thing: want still broken after Truncate(1)")
	}
	if check.ViewBroken(prog.Info, intact) {
		t.Error("view Other: want still not broken after Truncate(1)")
	}
}

// VIEWMODEL.md J4: an unresolved view target (E2102 before any item is checked) also marks it.
func TestViewBrokenUnresolvedTarget(t *testing.T) {
	prog, file, _ := checkKeepingBags(t, "package a\n\nview Nope {\n  title \"x\"\n}\n")
	v := checkedViews{prog: prog, files: []*syntax.File{file}}
	d := v.view("Nope")
	if d == nil {
		t.Fatal("view Nope not found")
	}
	if !check.ViewBroken(prog.Info, d) {
		t.Error("view Nope: want broken (its target does not resolve)")
	}
}

// checkFiles checks files (path -> source) across one or more packages, sharing one bag per
// package's directory, and returns the program and each parsed file by path.
func checkFiles(t *testing.T, files map[string]string) (*check.Program, map[string]*syntax.File) {
	t.Helper()
	fs := &source.FileSet{}
	bags := check.Bags{}
	parsed := map[string]*syntax.File{}
	var all []*syntax.File
	for path, src := range files { //canon:unordered every file parsed once, order does not matter
		f, err := fs.Add(path, "/"+path, []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		pkg := path[:strings.LastIndex(path, "/")]
		if bags[pkg] == nil {
			bags[pkg] = diag.NewBag(fs, pkg)
		}
		file := syntax.Parse(f, fileKindOf(path), bags[pkg])
		parsed[path], all = file, append(all, file)
	}
	prog := check.Check(context.Background(), exampleProject(), all, bags, literalFolder{})
	return prog, parsed
}

// fileKindOf is FileTranslation for a "<dir>/<name>.<lang>.canon" path (two dots in its base
// name), else FileSource.
func fileKindOf(path string) syntax.FileKind {
	base := path[strings.LastIndex(path, "/")+1:]
	if strings.Count(base, ".") > 1 {
		return syntax.FileTranslation
	}
	return syntax.FileSource
}

// VIEWMODEL.md J4: a bare reference to a broken (but resolved) declaration marks the view.
func TestViewBrokenBareReferenceToBrokenDeclaration(t *testing.T) {
	prog, files := checkFiles(t, map[string]string{"a/a.canon": `package a

let bad: String = 1 + "x"

record Thing {
  name: String
}

view Thing {
  title "T {bad}"
}
`})
	v := checkedViews{prog: prog, files: []*syntax.File{files["a/a.canon"]}}
	d := v.view("Thing")
	if d == nil {
		t.Fatal("view Thing not found")
	}
	if !check.ViewBroken(prog.Info, d) {
		t.Error("view Thing: want broken (a bare reference to broken bad)")
	}
}

// VIEWMODEL.md J4: a qualified reference (`pkg.name`) to a broken declaration marks the view too.
func TestViewBrokenQualifiedReferenceToBrokenDeclaration(t *testing.T) {
	prog, files := checkFiles(t, map[string]string{
		"o/o.canon": `package o

let bad: String = 1 + "x"
`,
		"a/a.canon": `package a

import o

record Thing {
  name: String
}

view Thing {
  title "T {o.bad}"
}
`,
	})
	v := checkedViews{prog: prog, files: []*syntax.File{files["a/a.canon"]}}
	d := v.view("Thing")
	if d == nil {
		t.Fatal("view Thing not found")
	}
	if !check.ViewBroken(prog.Info, d) {
		t.Error("view Thing: want broken (o.bad is a broken declaration, qualified)")
	}
}

// A call whose callee is a broken method marks the view too (its declared return type can stay
// well-formed, so only checking Calls[x].Obj catches it).
func TestViewBrokenCallToBrokenMethod(t *testing.T) {
	prog, files := checkFiles(t, map[string]string{"a/a.canon": `package a

record Thing {
  name: String

  fn bad(self) -> Int { return 1 + "x" }
}

view Thing {
  title "T {self.bad()}"
}
`})
	v := checkedViews{prog: prog, files: []*syntax.File{files["a/a.canon"]}}
	d := v.view("Thing")
	if d == nil {
		t.Fatal("view Thing not found")
	}
	if !check.ViewBroken(prog.Info, d) {
		t.Error("view Thing: want broken (self.bad() calls a broken method)")
	}
}

// ADR-0009: a lexical error inside a view (truncation-proof) marks it too.
func TestViewBrokenLexicalError(t *testing.T) {
	prog, file, _ := checkKeepingBags(t, `package a

record Thing {
  name: String
}

view Thing {
  title "T \q {name}"
}
`)
	v := checkedViews{prog: prog, files: []*syntax.File{file}}
	d := v.view("Thing")
	if d == nil {
		t.Fatal("view Thing not found")
	}
	if !check.ViewBroken(prog.Info, d) {
		t.Error("view Thing: want broken (\\q is an invalid escape sequence in its title)")
	}
}

// I18N.md T2: check marks BrokenTranslations by the report itself, not the expression's type: a
// format spec on a non-numeric (but well-typed, String?) interpolation still marks the entry.
func TestBrokenTranslationMarkedEvenWhenWellTyped(t *testing.T) {
	prog, files := checkFiles(t, map[string]string{
		"a/a.canon": `package a

record Three {
  note: String?
}

view Three {
  title "Three {note}"
}
`,
		"a/a.fr.canon": `package a
translation fr

Three.title "Trois {note:+}"
`,
	})
	entries := files["a/a.fr.canon"].Entries
	if len(entries) != 1 {
		t.Fatalf("want 1 translation entry, got %d", len(entries))
	}
	entry := entries[0]
	for _, part := range entry.Text.(*syntax.StringLit).Parts {
		if part.Interp == nil {
			continue
		}
		if t2 := prog.Info.Types[part.Interp.X]; t2 == nil || t2.Kind() == types.Error {
			t.Fatalf("note: want a well-typed expression (String?), got %v", t2)
		}
	}
	if !prog.Info.BrokenTranslations[entry] {
		t.Error("Three.title's fr entry: want BrokenTranslations marked (its format spec on note is invalid)")
	}
}

// ADR-0009: a lexical error inside a translation entry (a bad escape) marks it too.
func TestBrokenTranslationLexicalError(t *testing.T) {
	prog, files := checkFiles(t, map[string]string{
		"a/a.canon": `package a

record Thing {
  name: String
}

view Thing {
  title "T {name}"
}
`,
		"a/a.fr.canon": `package a
translation fr

Thing.title "T \q {name}"
`,
	})
	entries := files["a/a.fr.canon"].Entries
	if len(entries) != 1 {
		t.Fatalf("want 1 translation entry, got %d", len(entries))
	}
	if !prog.Info.BrokenTranslations[entries[0]] {
		t.Error("Thing.title's fr entry: want BrokenTranslations marked (\\q is an invalid escape)")
	}
}

// A translation's own error does not mark the source view (ADR-0009: translations are separate).
func TestViewBrokenTranslationErrorDoesNotMarkSourceView(t *testing.T) {
	fs := &source.FileSet{}
	src, err := fs.Add("a/a.canon", "/a/a.canon", []byte("package a\n\nrecord Thing {\n  name: String\n}\n\nview Thing {\n  title \"T {name}\"\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	trans, err := fs.Add("a/a.fr.canon", "/a/a.fr.canon", []byte("package a\ntranslation fr\n\nThing.title \"T {missing}\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(fs, "a")
	file := syntax.Parse(src, syntax.FileSource, bag)
	transFile := syntax.Parse(trans, syntax.FileTranslation, bag)
	bags := check.Bags{"a": bag}
	prog := check.Check(context.Background(), exampleProject(), []*syntax.File{file, transFile}, bags, literalFolder{})
	v := checkedViews{prog: prog, files: []*syntax.File{file}}
	d := v.view("Thing")
	if d == nil {
		t.Fatal("view Thing not found")
	}
	code := diag.E1703.Def().Code
	if !hasCode(bag, code) {
		t.Fatalf("want %s for the translation's unresolved {missing}, findings: %v", code, bag.Findings())
	}
	if check.ViewBroken(prog.Info, d) {
		t.Errorf("view Thing: want not broken (the error is the translation's own, %s)", code)
	}
}

func hasCode(bag *diag.Bag, code diag.Code) bool {
	for _, f := range bag.Findings() {
		if f.Code == code {
			return true
		}
	}
	return false
}
