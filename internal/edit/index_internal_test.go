package edit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// indexEntries is how many entries the indexed table holds: enough that a scan per read shows.
const indexEntries = 300

// indexTree is package t, a table of indexEntries entries in its literal, two more declared
// beside it, and a second table with one entry declared apart; package u declares a public
// `items` too, and a local `k` as t does.
func indexTree() rowsFS {
	var b strings.Builder
	b.WriteString("/// T.\npackage t\n\n/// An item.\nrecord Item {\n  /// N.\n  n: Int\n}\n\n/// Items.\nlet items: table Item = {\n")
	for i := range indexEntries {
		fmt.Fprintf(&b, "  k%d { n: %d }\n", i, i)
	}
	b.WriteString("}\n\n/// Others.\nlet others: table Item = {}\n\n/// K.\nlocal let k: Int = 1\n")
	return rowsFS{
		"law/project.canon": {Data: []byte("project acme {\n  canon: \"0.1\"\n}\n")},
		"law/t/t.canon":     {Data: []byte(b.String())},
		"law/t/more.canon": {Data: []byte("package t\n\n/// E1.\nentry items.e1 { n: 1 }\n\n/// O1.\nentry others.o1 { n: 5 }\n\n" +
			"/// E2.\nentry items.e2 { n: 2 }\n")},
		"law/u/u.canon": {Data: []byte("/// U.\npackage u\n\n/// Items.\nlet items: [Int] = [1]\n\n/// K.\nlocal let k: Int = 2\n")},
	}
}

// indexSnapshot is the every-package analysis of indexTree.
func indexSnapshot(t *testing.T) *Snapshot {
	t.Helper()
	p, err := build.Open(indexTree(), "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := a.Result().Summary.Errors; n > 0 {
		t.Fatalf("%d errors: %v", n, a.Result().List)
	}
	return NewSnapshot(a)
}

// scanLookup and scanEntries are lookup and entryDecls as a walk over every declaration.
func scanLookup(s *Snapshot, p Path) []rootRef {
	var found []rootRef
	for _, pkg := range s.pkgs {
		for _, obj := range pkg.Decls {
			if r, ok := rootOf(pkg, obj, p.Root); ok && (p.Package == "" && public(obj) || pkg.Path == p.Package) {
				found = append(found, r)
			}
		}
	}
	return found
}

func scanEntries(s *Snapshot, r rootRef) []item {
	var out []item
	for _, pkg := range s.pkgs {
		for _, obj := range pkg.Decls {
			if d, ok := obj.Decl().(*syntax.EntryDecl); ok && obj.Kind() == check.ObjEntry && s.info.NameUses[d.Table] == r.obj {
				out = append(out, item{d, obj.File()})
			}
		}
	}
	return out
}

// API.md P6, P7, P7a, W2: the index finds a path's root, an ambiguous or a missing one, and a
// collection's entry declarations, as a walk over every declaration does.
func TestDeclIndex(t *testing.T) {
	s := indexSnapshot(t)
	for _, text := range []string{"items", "t:items", "u:items", "others", "k", "t:k", "u:k", "Item", "missing", "u:others"} {
		p, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		want := scanLookup(s, p)
		got, err := s.lookup(p)
		var pe *PathError
		switch {
		case len(want) == 1 && (err != nil || got != want[0]):
			t.Errorf("P6 %s: %v, %v, want %v", text, got, err, want[0])
		case len(want) == 0 && (!errors.As(err, &pe) || !errors.Is(err, ErrNoPath)):
			t.Errorf("P6 %s: %v, want ErrNoPath", text, err)
		case len(want) > 1 && (!errors.As(err, &pe) || !errors.Is(err, ErrAmbiguousPath) || len(pe.Candidates) != len(want)):
			t.Errorf("P6 %s: %v, want ErrAmbiguousPath of %d", text, err, len(want))
		}
		for _, r := range want {
			if got := s.entryDecls(r); !reflect.DeepEqual(got, scanEntries(s, r)) || cap(got) != len(got) {
				t.Errorf("W2 %s: entries %v, want %v", r.qualified(), got, scanEntries(s, r))
			}
		}
	}
}

// API.md W2: a value of a collection matches the entry declared for it, of that collection only.
func TestEntryAt(t *testing.T) {
	s := indexSnapshot(t)
	items, err1 := s.lookup(Path{Package: "t", Root: "items"})
	others, err2 := s.lookup(Path{Package: "t", Root: "others"})
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	for _, c := range []struct {
		of, at rootRef
		n      int
	}{{items, items, 2}, {others, others, 1}} {
		decls := s.entryDecls(c.of)
		if len(decls) != c.n {
			t.Fatalf("W2 %s: %d entries, want %d", c.of.qualified(), len(decls), c.n)
		}
		for _, it := range decls {
			span := it.file.Span(it.node)
			if got, ok := s.entryAt(c.at.obj, span); !ok || got != it {
				t.Errorf("W2 %s: entry at %v not found", c.of.qualified(), span)
			}
			other := items
			if c.of == items {
				other = others
			}
			if _, ok := s.entryAt(other.obj, span); ok {
				t.Errorf("W2: an entry of %s matches in %s", c.of.qualified(), other.qualified())
			}
		}
	}
}

// API.md W2: every entry of a large table (API.md 6.2) resolves, and judges editable, by the
// index as by a scan; the entries declared apart are edited in their own file.
func TestTableKeysIndex(t *testing.T) {
	s := indexSnapshot(t)
	root, err := s.lookup(Path{Package: "t", Root: "items"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.force(root)
	table, ok := v.(*value.Table)
	if err != nil || !ok {
		t.Fatalf("t:items: %v, %v", v, err)
	}
	names := []string{"e1", "e2", "missing", "0"}
	for i := range indexEntries {
		names = append(names, fmt.Sprintf("k%d", i))
	}
	for _, name := range names {
		p, err := Parse(`t:items["` + name + `"]`)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Resolve(s, p)
		i, ok := s.keys.find(table, name)
		j, scanOK := (*tableKeys)(nil).find(table, name)
		if ok != scanOK || i != j || (err == nil) != ok {
			t.Fatalf("§6.2 %s: index %d %v, scan %d %v, Resolve %v", name, i, ok, j, scanOK, err)
		}
		if !ok {
			continue
		}
		e, err := s.Editable(got, OpSet, "")
		want := "t/t.canon"
		if strings.HasPrefix(name, "e") {
			want = "t/more.canon"
		}
		if err != nil || e.Mode != ModeCanon || e.File != want {
			t.Errorf("W2 %s: %+v, %v, want %s", name, e, err, want)
		}
	}
	if len(s.keys.by) != 1 {
		t.Errorf("§6.2: %d tables indexed, want items alone", len(s.keys.by))
	}
}
