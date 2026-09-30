package edit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// API.md W5, M6 (FuzzMinimalWriteAll, log-2026-09-29 M4 B10): jobNames, read `at: "*.us"` from
// items that also hold "fr" and "gr", refuses item operations (reason format) but not a Set of an
// entry; a Set changing an object's last member and adding one after it writes within its regions.
func TestExamplesB10(t *testing.T) {
	dir, roots := exampleRoots(t)
	fz := openFuzzProject(t, dir, roots)
	for _, op := range []edit.Operation{
		{Kind: edit.OpMove, Path: "balance.parity:jobNames[#3]", Index: 0},
		{Kind: edit.OpRemove, Path: "balance.parity:jobNames[#4]"},
		{Kind: edit.OpRename, Path: "balance.parity:jobNames[#13]", Key: edit.Key("IDS_NEW")},
		{Kind: edit.OpAddEntry, Path: "balance.parity:jobNames", Key: edit.Key("IDS_NEW"), Value: edit.Str("New")},
	} {
		_, err := edit.Apply(context.Background(), fz.env, edit.NewSnapshot(fz.a), edit.Request{Ops: []edit.Operation{op}})
		var ne *edit.NotEditableError
		if !errors.As(err, &ne) || ne.Reason != edit.ReasonFormat || !strings.Contains(ne.File, "etc.json#/") {
			t.Errorf("operation %d on %s: %v, want not editable, format, naming the data beside", op.Kind, op.Path, err)
		}
	}
	src, ok := fz.byPath["resource.adventurequest:adventureQuests.styles[#0]"]
	if !ok {
		t.Fatal("no adventureQuests.styles[#0] in the examples")
	}
	lit, ok := fz.lit(fz.cands[src].v, draw{n: seedN, s: seedS})
	if !ok {
		t.Fatal("styles[#0] has no literal")
	}
	for _, op := range []edit.Operation{
		{Kind: edit.OpSet, Path: "balance.parity:jobNames[#3]", Value: edit.Str("Assistant")},
		{Kind: edit.OpSet, Path: "resource.adventurequest:adventureQuests.styles[#1]", Value: lit},
	} {
		if !applyChecked(t, fz.env, fz.a, []edit.Operation{op}, false) {
			t.Errorf("operation %d on %s: refused", op.Kind, op.Path)
		}
	}
}

// starFS is a map read `at: "*.us"`, its item B holding "fr" beside the selection, or not.
func starFS(beside bool) mapFS {
	b := `    "us": "Bee"`
	if beside {
		b += ",\n    \"fr\": \"Be\""
	}
	return mapFS{
		"law/project.canon": file(projectCanon),
		"law/d/d.canon":     file("/// D.\npackage d\n\n/// Names.\nlet names: {String: String} = load(\"etc.json\", at: \"*.us\")\n"),
		"law/d/etc.json":    file("{\n  \"A\": {\n    \"us\": \"Ay\"\n  },\n  \"B\": {\n" + b + "\n  }\n}\n"),
	}
}

// API.md 7.2, W5 (log-2026-09-29 M4 B10): Editable decides the ruling, so reads and edits
// agree: with data beside the selection, the item operations and a Set of the map are format,
// the data named; a Set of an entry is editable; without it, everything is.
func TestStarEditability(t *testing.T) {
	for _, c := range []struct {
		name, path string
		op         edit.Op
		beside     bool
		file       string
	}{
		{"Remove, beside", "names[A]", edit.OpRemove, true, "d/etc.json#/B/fr"},
		{"Move, beside", "names[A]", edit.OpMove, true, "d/etc.json#/B/fr"},
		{"Rename, beside", "names[A]", edit.OpRename, true, "d/etc.json#/B/fr"},
		{"AddEntry, beside", "names", edit.OpAddEntry, true, "d/etc.json#/B/fr"},
		{"Set map, beside", "names", edit.OpSet, true, "d/etc.json#/B/fr"},
		{"Set entry, beside", "names[B]", edit.OpSet, true, ""},
		{"Remove, alone", "names[A]", edit.OpRemove, false, ""},
		{"AddEntry, alone", "names", edit.OpAddEntry, false, ""},
	} {
		s := open(t, starFS(c.beside), nil, "", "d")
		p, err := edit.Parse(c.path)
		if err != nil {
			t.Fatal(err)
		}
		r, err := edit.Resolve(s.snap, p)
		if err != nil {
			t.Fatal(err)
		}
		e, err := s.snap.Editable(r, c.op, "")
		want := edit.ReasonNone
		if c.file != "" {
			want = edit.ReasonFormat
		}
		if err != nil || e.Reason != want || c.file != "" && e.File != c.file {
			t.Errorf("%s: %+v, %v; want reason %d naming %q", c.name, e, err, want, c.file)
		}
	}
}

// API.md E22 (log-2026-09-29 M4 B10): with nothing beside the selection, a Remove gives the
// file back byte for byte after its Undo.
func TestStarRoundTrip(t *testing.T) {
	roundTrip(t, "Remove", starFS(false), []edit.Operation{{Kind: edit.OpRemove, Path: "names[A]"}})
}

// API.md N6, W5 (log-2026-09-29 M4 B10-r): a Remove of a load.dir element deletes its file, so a
// file holding data outside what the `at:` path selects refuses it (format, the datum named);
// a file holding nothing else is removed.
func TestLoadDirBeside(t *testing.T) {
	fsys := mapFS{
		"law/project.canon":  file(projectCanon),
		"law/d/d.canon":      file("/// D.\npackage d\n\n/// Names.\nlet names: [{String: String}] = load.dir(\"names/*.json\", at: \"*.us\")\n"),
		"law/d/names/a.json": file("{\n  \"A\": {\n    \"us\": \"x\",\n    \"fr\": \"y\"\n  }\n}\n"),
		"law/d/names/b.json": file("{\n  \"B\": {\n    \"us\": \"z\"\n  }\n}\n"),
	}
	s := open(t, fsys, nil, "", "d")
	for _, c := range []struct {
		path, file string
	}{
		{"names[#0]", "d/names/a.json#/A/fr"},
		{"names[#1]", ""},
	} {
		op := edit.Operation{Kind: edit.OpRemove, Path: c.path}
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
		var ne *edit.NotEditableError
		switch {
		case c.file == "" && (err != nil || len(plan.Changes) == 0 || plan.Changes[0].Kind != edit.ChangeDeleted):
			t.Errorf("%s: %v, want its file deleted", c.path, err)
		case c.file != "" && (!errors.As(err, &ne) || ne.Reason != edit.ReasonFormat || ne.File != c.file):
			t.Errorf("%s: %v, want not editable, format, naming %s", c.path, err, c.file)
		}
	}
}
