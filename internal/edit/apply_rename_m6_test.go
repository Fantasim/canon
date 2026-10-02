package edit_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// renameM6 is a project where a field renamed sits between declarations it does not touch.
var renameM6 = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon": file("package a\n\n/// An item.\nrecord Item {\n  /// How many.\n  count: Int\n}\n\n// Kept as written.\n/// Untouched.\n" +
		"const LIMIT = 3\n\n/// The items.\nlet items: [Item] = [{ count: 1 }]\n"),
}

// API.md M6, API.md E33: a RenameName's whole-file write records the items holding a renamed
// token as its regions, so M6 judges it: the write keeps every other byte, and the same write
// with one byte changed outside them (in a declaration it does not touch) fails M6.
func TestRenameNameRegions(t *testing.T) {
	s := open(t, renameM6, nil, "", "a")
	op := edit.Operation{Kind: edit.OpRenameName, Path: "a:Item.count", Name: "amount"}
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	if err != nil {
		t.Fatal(err)
	}
	ws := edit.Writes(plan, "a/a.canon")
	if len(ws) != 1 || len(ws[0].Regions) != 2 || ws[0].Regions[0].Kind != edit.RegionNode {
		t.Fatalf("API.md M6: writes %+v", ws)
	}
	if !keptOutside(t, "a/a.canon", ws[0]) {
		t.Errorf("API.md M6: the rename changes bytes outside its regions:\n%s", ws[0].After)
	}
	bad := ws[0]
	bad.After = bytes.Replace(bad.After, []byte("Kept as written"), []byte("Kept as wrItten"), 1)
	if keptOutside(t, "a/a.canon", bad) {
		t.Error("API.md M6: a byte changed outside the renamed items passes M6")
	}
}
