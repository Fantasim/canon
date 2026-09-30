package project_test

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"golang.org/x/tools/txtar"
)

const (
	graftFile = "m/m.canon"
	graftBase = `/// M.
package m

import n

/// A monster.
record Monster {
  /// Level.
  level: Int(1..=9)
}

entry things.a { level: 1 }

/// Monsters.
let monsters: table Monster = {
  a { level: 3 }
  b { level: 4 }
}

/// After.
const LAST = 2
`
	graftNFile = "n/n.canon"
	graftN     = "package n\n"
)

// graftRead is one read of the graft project through a store: m's tree.
func graftRead(t *testing.T, set *source.FileSet, reuse *project.Reuse, content string) *syntax.File {
	t.Helper()
	fsys := newMemFS(&txtar.Archive{})
	fsys.m[memPath(graftFile)] = &fstest.MapFile{Data: []byte(content)}
	fsys.m[memPath(graftNFile)] = &fstest.MapFile{Data: []byte(graftN)}
	bagOf := func(pkg string) *diag.Bag { return diag.NewBag(set, pkg) }
	r := &project.Reader{FS: fsys, Dir: projectDir, Set: set, BagOf: bagOf, Reuse: reuse}
	units, err := r.Parse(context.Background(), []string{graftFile, graftNFile})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range units {
		for _, f := range u.Files {
			if f.Src.Path == graftFile {
				return f
			}
		}
	}
	t.Fatal("m.canon was not read")
	return nil
}

// shapeOf is every node of f in walk order: its kind and span, what a fresh parse must match.
func shapeOf(f *syntax.File) string {
	var sb strings.Builder
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n != nil {
			sp := f.Span(n)
			sb.WriteString(n.Kind().String())
			sb.WriteByte(' ')
			sb.WriteString(string(f.Src.Content[sp.Start:sp.End]))
			sb.WriteByte('\n')
		}
		return true
	})
	return sb.String()
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: a new parse shares the unchanged prefix and reads as a fresh one.
func TestGraftSharesUnchangedPrefix(t *testing.T) {
	cases := []struct {
		name, from, to string
		shared         []bool // by declaration of m: record, entry, let, const
		header         bool
	}{
		{"row edit", "a { level: 3 }", "a { level: 7 }", []bool{true, false, false, false}, true},
		{"record edit", "level: Int(1..=9)", "level: Int(1..=8)", []bool{false, false, false, false}, true},
		{"last edit", "LAST = 2", "LAST = 3", []bool{true, false, true, false}, true},
		{"doc edit", "/// M.\n", "/// Mm.\n", []bool{false, false, false, false}, false},
		{"length change", "a { level: 3 }", "a { level: 31 }", []bool{true, false, false, false}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set := &source.FileSet{}
			reuse := project.NewReuse(set)
			old := graftRead(t, set, reuse, graftBase)
			edited := strings.Replace(graftBase, tc.from, tc.to, 1)
			nf := graftRead(t, set, reuse, edited)
			for i, want := range tc.shared {
				if got := nf.Decls[i] == old.Decls[i]; got != want {
					t.Errorf("declaration %d shared %v, want %v", i, got, want)
				}
			}
			if got := nf.Package == old.Package && nf.Imports[0] == old.Imports[0]; got != tc.header {
				t.Errorf("header shared %v, want %v", got, tc.header)
			}
			fresh := syntax.Parse(nf.Src, syntax.FileSource, diag.NewBag(set, ""))
			if shapeOf(nf) != shapeOf(fresh) {
				t.Errorf("the grafted tree differs from a fresh parse:\n%s\nwant:\n%s", shapeOf(nf), shapeOf(fresh))
			}
		})
	}
}

// NFR-02: a parse holding a syntax error shares nothing, and a later clean one shares nothing with it.
func TestGraftSkipsBrokenParses(t *testing.T) {
	set := &source.FileSet{}
	reuse := project.NewReuse(set)
	graftRead(t, set, reuse, graftBase)
	broken := strings.Replace(graftBase, "LAST = 2", "LAST = ", 1)
	bad := graftRead(t, set, reuse, broken)
	clean := graftRead(t, set, reuse, strings.Replace(graftBase, "LAST = 2", "LAST = 5", 1))
	if bad.Decls[0] == clean.Decls[0] {
		t.Error("a clean parse shared a node with a broken one")
	}
}
