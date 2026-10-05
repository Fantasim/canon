package canon_test

import (
	"context"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// n1Table is ms, a literal of wolf and bat, with owl declared in the let's own file.
const n1Table = srcHead + "let ms: table M = {\n  wolf { hp: 1 }\n  bat { hp: 2 }\n}\n\nentry ms.owl { hp: 3 }\n"

// API.md N1: an AddEntry gets a new file only under @files or beside an entry declared in a file
// other than the let's; an `entry` in the let's own file leaves the new entry in the literal.
func TestEditN1Placement(t *testing.T) {
	eel := canon.AddEntry("p:ms", canon.Key("eel"), canon.Obj{"hp": canon.Int(4)})
	other := srcLaw(strings.Replace(n1Table, "\nentry ms.owl { hp: 3 }\n", "", 1), "p/owl.canon", "package p\n\nentry ms.owl { hp: 3 }\n")
	for _, c := range []struct {
		name    string
		law     map[string]string
		newFile bool
	}{
		{"declaration in the let's file", srcLaw(n1Table), false},
		{"declaration in another file", other, true},
	} {
		p, m := openEdit(t, c.law)
		if _, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{eel}}); err != nil {
			t.Errorf("API.md N1, %s: %v", c.name, err)
			continue
		}
		_, err := m.ReadFile("/law/p/ms/eel.canon")
		inLiteral := strings.Contains(read(t, m, srcMain), "  bat { hp: 2 }\n  eel { hp: 4 }\n}")
		if (err == nil) != c.newFile || inLiteral == c.newFile {
			t.Errorf("API.md N1, %s: new file %v, in the literal %v:\n%s", c.name, err == nil, inLiteral, read(t, m, srcMain))
		}
	}
}

// API.md E22, E23, N1: a keyed list's literal orders its elements beside an `entry` of the let's
// own file, so the Undo of a Remove inserts the element back at its place (a list's order is
// its value), not after the others.
func TestEditN1KeyedListUndo(t *testing.T) {
	ks := "package p\n\nrecord K {\n  id: String\n  v: Int = 0\n}\n\nlet ks: [K] keyed by id = [{ id: \"a\" }, { id: \"b\" }]\n\nentry ks.c { v: 1 }\n"
	for _, c := range []srcCase{
		{name: "the first element", edit: canon.Edit{Ops: []canon.Op{canon.Remove("p:ks.a")}}, undo: []canon.Op{{Kind: canon.OpInsert, Path: "p:ks"}}},
		{name: "the literal's last", edit: canon.Edit{Ops: []canon.Op{canon.Remove("p:ks.b")}}, undo: []canon.Op{{Kind: canon.OpInsert, Path: "p:ks"}}},
	} {
		c.law, c.file = srcLaw(ks), srcMain
		runSource(t, "API.md E22, E23, N1", c)
	}
	p, _ := openEdit(t, srcLaw(ks))
	before, _ := p.Value(context.Background(), "p:ks")
	res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Remove("p:ks.a")}})
	if err == nil {
		_, err = p.Edit(context.Background(), canon.Edit{Base: res.Revision, Ops: res.Undo})
	}
	if after, _ := p.Value(context.Background(), "p:ks"); err != nil || after.Text != before.Text {
		t.Errorf("API.md E22: p:ks after the Undo %q, %v, want %q", after.Text, err, before.Text)
	}
}

// API.md N1, E23, E22: beside an `entry` of the let's own file, the Undo of a Remove of a
// literal entry adds it back into the literal and moves it to its place; on a table the Remove
// repairs (the reviewer's probe) as on a healthy one, the literal comes back exactly.
func TestEditN1RemoveUndo(t *testing.T) {
	broken := srcHead + "let ms: table M = {\n  wolf { hp: 1 }\n  bat {}\n}\n\nentry ms.owl { hp: 2 }\n"
	for _, c := range []srcCase{
		{name: "Remove of the first", law: srcLaw(n1Table), edit: canon.Edit{Ops: []canon.Op{canon.Remove("p:ms.wolf")}},
			undo: []canon.Op{canon.AddEntry("p:ms", canon.Key("wolf"), canon.Source("")), canon.Move("p:ms.wolf", 0)}},
		{name: "Remove of the last", law: srcLaw(n1Table), edit: canon.Edit{Ops: []canon.Op{canon.Remove("p:ms.bat")}},
			undo: []canon.Op{canon.AddEntry("p:ms", canon.Key("bat"), canon.Source(""))}},
		{name: "Remove repairing the table", law: srcLaw(broken), edit: canon.Edit{Ops: []canon.Op{canon.Remove("p:ms.bat")}},
			undo: []canon.Op{canon.AddEntry("p:ms", canon.Key("bat"), canon.Source(""))}},
	} {
		c.file = srcMain
		runSource(t, "API.md N1, E23", c)
	}
	p, _ := openEdit(t, srcLaw(n1Table))
	_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Move("p:ms.owl", 0)}})
	if err == nil {
		t.Errorf("API.md E9, N1: Move of a declared entry before the literal's: %v, want refused", err)
	}
}
