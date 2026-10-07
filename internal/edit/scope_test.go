package edit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
)

// scopeUnits is the parsed packages of a project where p and q both declare a public `x`, q a
// local `y` beside p's public one, r imports p and aliases its record, s imports r.
func scopeUnits(t *testing.T) []*project.Unit {
	t.Helper()
	fsys := mapFS{
		"law/project.canon": file(projectCanon),
		"law/p/p.canon": file("package p\n\n/// X.\nlet x: Int = 1\n\n/// Y.\nlet y: Int = 2\n\n" +
			"/// A record.\nrecord Rec {\n  /// F.\n  f: Int\n}\n"),
		"law/q/q.canon": file("package q\n\n/// X.\nlet x: Int = 1\n\n/// Y.\nlocal let y: Int = 2\n"),
		"law/r/r.canon": file("package r\n\nimport p\n\n/// An alias.\ntype Alias = p.Rec\n\n/// Z.\nlet z: Int = p.x\n"),
		"law/s/s.canon": file("package s\n\nimport r\n\n/// W.\nlet w: Int = r.z\n"),
	}
	b, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	st, err := b.Static(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return st.Units
}

// API.md E17a, P6, P7, E27 (DECISIONS 330): the scope is the packages that may declare the ops'
// roots, by the parse: every public candidate unqualified, none for an unknown root, imports too
// for a member through a type or a position; a Rename or RenameName asks for the importers.
func TestScope(t *testing.T) {
	units := scopeUnits(t)
	for _, c := range []struct {
		name      string
		ops       []edit.Operation
		want      []string
		importers bool
	}{
		{"qualified", []edit.Operation{{Kind: edit.OpSet, Path: "q:y"}}, []string{"q"}, false},
		{"unqualified, two candidates", []edit.Operation{{Kind: edit.OpSet, Path: "x"}}, []string{"p", "q"}, false},
		{"unqualified, a local one left out", []edit.Operation{{Kind: edit.OpSet, Path: "y"}}, []string{"p"}, false},
		{"no package", []edit.Operation{{Kind: edit.OpSet, Path: "nope:x"}, {Kind: edit.OpSet, Path: "[bad"}}, nil, false},
		{"rename of a key", []edit.Operation{{Kind: edit.OpRename, Path: "r:z"}}, []string{"r"}, true},
		{"renameName of a type", []edit.Operation{{Kind: edit.OpRenameName, Path: "p:Rec"}}, []string{"p"}, true},
		{"renameName through an alias", []edit.Operation{{Kind: edit.OpRenameName, Path: "r:Alias.f"}}, []string{"p", "r"}, true},
		{"renameName at a position", []edit.Operation{{Kind: edit.OpRenameName, Path: "s/s.canon:6:15"}}, []string{"p", "r", "s"}, true},
	} {
		got, importers := edit.Scope(units, c.ops)
		if !slices.Equal(got, c.want) || importers != c.importers {
			t.Errorf("%s: Scope = %v, %v; want %v, %v", c.name, got, importers, c.want, c.importers)
		}
	}
}
