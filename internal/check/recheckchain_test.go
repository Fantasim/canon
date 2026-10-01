package check_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	zooTypes = "zoo/types.canon"
	zooUse   = "zoo/use.canon"
	dogLegs  = `name: "Dog", legs: 4 }`
	zooTail  = "// The end.\n"
)

// IMPLEMENTATION-PLAN §7.6 NFR-02: an entry checked again under a shared table is one object, as cold.
func TestRecheckEntryUnderSharedTable(t *testing.T) {
	w := newWorld(t, zoo)
	_, s, _ := w.session()
	types := w.shared(zooTypes, w.src[zooTypes]+zooTail)
	prog, next, bags, ok := w.recheck(s, types, w.edit(zooUse, dogLegs, `name: "Dog", legs: 3 }`))
	if !ok {
		t.Fatal("Recheck refused a shared table file and an entry")
	}
	if got, want := canonical(w.fs, prog, bags), w.cold(); got != want {
		t.Fatalf("Recheck differs from Check:\n%s", firstDiff(got, want))
	}
	if split := splitEntries(prog); len(split) > 0 {
		t.Fatalf("entries Decls and Info name apart: %v", split)
	}
	next = w.step(next, step{zooUse, `legs: 3 }`, `legs: 2 }`, true})
	w.step(next, step{zooUse, `legs: 2 }`, `legs: 4 }`, true})
}

// shared is a new version of path holding text, which extends its current content, sharing the
// current version's header and declarations as project.Reuse's graft does.
func (w *world) shared(path, text string) *syntax.File {
	w.t.Helper()
	old := w.files[path]
	if !strings.HasPrefix(text, w.src[path]) {
		w.t.Fatalf("%s: the new text does not extend the current one", path)
	}
	nf := w.parse(path, text)
	if len(nf.Decls) != len(old.Decls) || len(nf.Imports) != len(old.Imports) {
		w.t.Fatalf("%s: the new text adds declarations", path)
	}
	nf.Doc, nf.Package = old.Doc, old.Package
	copy(nf.Imports, old.Imports)
	copy(nf.Decls, old.Decls)
	return nf
}

// splitEntries names each `entry` of prog's Decls that is not the object Info defines at its key,
// or whose table Info does not name.
func splitEntries(prog *check.Program) []string {
	var out []string
	for _, p := range prog.Packages {
		for _, o := range p.Decls {
			d, ok := o.Decl().(*syntax.EntryDecl)
			if !ok {
				continue
			}
			key, _ := d.Key.(*syntax.Ident)
			if key != nil && prog.Info.Defs[key] != o || prog.Info.NameUses[d.Table] == nil {
				out = append(out, objText(o))
			}
		}
	}
	return out
}
