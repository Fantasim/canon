package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// typedFilesFS is branchFS with tables and a keyed list whose entries live in files of their own: in d,
// and in e, which names d's types through an import that a new entry file does not have (N5).
func typedFilesFS() mapFS {
	fs := branchFS()
	fs["law/d/d.canon"] = file(branchSrc + "\n/// Filed.\n@files(\"q/{id}.canon\")\nlet dq: table OT = {}\n")
	fs["law/e/e.canon"] = file("package e\n\nimport d\n\n/// A keyed row.\nrecord KR {\n  /// Its key.\n  id: String\n  /// The holder.\n  h: d.OT = {}\n}\n\n" +
		"/// Filed.\n@files(\"q/{id}.canon\")\nlet et: table d.OT = {}\n\n/// Filed, keyed.\n@files(\"k/{id}.canon\")\nlet kl: [KR] keyed by id = []\n")
	return fs
}

// API.md N1, N5, M7 (DECISIONS 366, 475): a new entry file holds no import, so a record of another
// package's branch, which only a typed literal states there, is a ValueError, nothing written; one
// of the file's own package is written `RB { … }`, no import, and its Undo restores.
func TestTypedLiteralNewEntryFile(t *testing.T) {
	kb, x1 := edit.Member("b"), edit.Obj{"x": edit.Int(1)}
	for _, ops := range [][]edit.Operation{
		{{Kind: edit.OpAddEntry, Path: "e:et", Key: edit.Key("two"), Value: edit.Obj{"k": kb, "t": x1}}},
		{{Kind: edit.OpAdd, Path: "e:kl", Value: edit.Obj{"id": edit.Str("two"), "h": edit.Obj{"k": kb, "t": x1}}}},
	} {
		s := open(t, typedFilesFS(), nil, "", "d", "e")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
		if !errors.Is(err, edit.ErrBadValue) || plan != nil {
			t.Errorf("%s: planned %v, %v, want ErrBadValue and no plan", ops[0].Path, plan != nil, err)
		}
	}
	ops := []edit.Operation{{Kind: edit.OpAddEntry, Path: "dq", Key: edit.Key("two"), Value: edit.Obj{"k": kb, "t": x1}}}
	edited, ok := applied(t, "same package", typedFilesFS(), ops)
	if !ok {
		return
	}
	text := string(edited.fsys["law/d/q/two.canon"].Data)
	if want := "package d\n\nentry dq.two {\n  k: b\n  t: RB { x: 1 }\n}\n"; text != want {
		t.Errorf("same package: q/two.canon = %q, want %q", text, want)
	}
	checkClean(t, "same package", typedFilesFS(), ops)
	undoTwice(t, "same package", typedFilesFS(), ops)
}

// API.md N1, N5, N6, E22 (DECISIONS 366, 466, 474): the Undo of a Remove of an entry file recreates the
// file, which holds no import: where it would state another package's typed literal the Remove is
// refused with that ValueError, nothing written; in the type's own package it applies and undoes.
func TestTypedLiteralRemoveEntryFile(t *testing.T) {
	fs := typedFilesFS()
	fs["law/e/q/one.canon"] = file("package e\n\nimport d\n\n/// One.\nentry et.one { k: b, t: d.RB { x: 1 } }\n")
	fs["law/d/q/one.canon"] = file("package d\n\n/// One.\nentry dq.one { k: b, t: RB { x: 1 } }\n")
	s := open(t, fs, nil, "", "d", "e")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpRemove, Path: "e:et.one"}}})
	if !errors.Is(err, edit.ErrBadValue) || plan != nil {
		t.Errorf("another package: planned %v, %v, want ErrBadValue and no plan", plan != nil, err)
	}
	undoTwice(t, "same package", fs, []edit.Operation{{Kind: edit.OpRemove, Path: "dq.one"}})
}
