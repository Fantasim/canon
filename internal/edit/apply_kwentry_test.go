package edit_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
)

// reservedWords is GRAMMAR.md §4.1, the whole set.
var reservedWords = strings.Fields("and amend as asset break check const continue else emit entry enum expect export " +
	"false fn for if import in input is layer let load local match none not or package project record ref retired " +
	"return self stable table test translation true type var variant view warn where while widget")

// unreachableKeys are the words that may never name a table key (GRAMMAR.md §4.3).
var unreachableKeys = []string{"none", "true", "false", "self"}

// joiningKeys are the entry keys that need a comma above them (GRAMMAR.md §3.1 rule 3, §5.11).
var joiningKeys = []string{"and", "or", "in", "is", "else", "where", "for"}

// kwEntryFS is a table literal of two entries and a declared one, in a project root /law.
var kwEntryFS = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon": file("package a\n\nrecord T {\n  w: Int\n}\n\nlet tb: table T = {\n  x { w: 1 }\n  y { w: 2 }\n}\n\n" +
		"entry tb.z { w: 3 }\n"),
}

// kwEntryWant is kwEntryFS's table once the entry at line index at (0 or 1) is spelled w, with the
// comma a joining word needs after the entry above it.
func kwEntryWant(w string, at int) string {
	first, second := "x", "y"
	comma := ""
	if slices.Contains(joiningKeys, w) {
		comma = ","
	}
	if at == 0 {
		first = w
		comma = ""
	} else {
		second = w
	}
	return "let tb: table T = {\n  " + first + " { w: 1 }" + comma + "\n  " + second + " { w: 2 }\n}\n"
}

// kwApply applies op to kwEntryFS.
func kwApply(t *testing.T, op edit.Operation) (*edit.Plan, error) {
	t.Helper()
	s := open(t, kwEntryFS, nil, "", "a")
	return edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
}

// kwRefused is err the refusal of a table key that can never be read back: a *ValueError that says
// why and names the way out (DECISIONS 305), never an internal error.
func kwRefused(t *testing.T, w string, err error) {
	t.Helper()
	var ve *edit.ValueError
	switch {
	case errors.Is(err, edit.ErrInternal):
		t.Errorf("%s: GRAMMAR.md §4.3: an internal error, want a refusal: %v", w, err)
	case !errors.As(err, &ve) || !errors.Is(err, edit.ErrBadValue):
		t.Errorf("%s: API.md V1: %v, want a *ValueError", w, err)
	case !strings.Contains(ve.Detail, "unreachable") || !strings.Contains(ve.Detail, "String field"):
		t.Errorf("%s: DECISIONS 305: detail %q must give the reason and the way out", w, ve.Detail)
	}
}

// kwWritten checks the plan's files (API.md M5, M6, N12), the file a/a.canon holding want, a
// clean re-check, and that Undo gives the values back (E22).
func kwWritten(t *testing.T, w string, op edit.Operation, plan *edit.Plan, want string) {
	t.Helper()
	for _, ch := range plan.Changes {
		checkWritten(t, plan, ch)
	}
	after := written(kwEntryFS, plan)
	if got := fileText(after, "law/a/a.canon"); !strings.Contains(got, want) {
		t.Errorf("%s: a/a.canon holds:\n%s\nwant it to hold:\n%s", w, got, want)
	}
	for _, f := range open(t, after, nil, "", "a").a.Bag("a").Findings() {
		if f.Severity == diag.Error {
			t.Errorf("%s: after the edit (API.md E18): %s %s", w, f.Code, f.Message)
		}
	}
	checkUndo(t, goldenCase{fsys: kwEntryFS, pkgs: []string{"a"}}, []edit.Operation{op}, plan)
}

// API.md E11, V1; GRAMMAR.md §3.1 rule 3, §4.3: Rename of a literal's entry to every reserved word.
func TestKWEntryRenameLiteralEveryWord(t *testing.T) {
	for _, w := range reservedWords {
		for at, from := range []string{"a:tb.x", "a:tb.y"} {
			t.Run(w+" from "+from, func(t *testing.T) {
				op := edit.Operation{Kind: edit.OpRename, Path: from, Key: edit.Key(w)}
				plan, err := kwApply(t, op)
				if slices.Contains(unreachableKeys, w) {
					kwRefused(t, w, err)
					return
				}
				if err != nil {
					t.Fatalf("Rename %s to %s: %v, want it applied (GRAMMAR.md §4.3 (a))", from, w, err)
				}
				kwWritten(t, w, op, plan, kwEntryWant(w, at))
			})
		}
	}
}

// API.md E3, E21, V1; GRAMMAR.md §3.1 rule 3, §4.3, §5.11: AddEntry of every reserved word.
func TestKWEntryAddEveryWord(t *testing.T) {
	for _, w := range reservedWords {
		t.Run(w, func(t *testing.T) {
			op := edit.Operation{Kind: edit.OpAddEntry, Path: "a:tb", Key: edit.Key(w), Value: edit.Source("{ w: 4 }")}
			plan, err := kwApply(t, op)
			if slices.Contains(unreachableKeys, w) {
				kwRefused(t, w, err)
				return
			}
			if err != nil {
				t.Fatalf("AddEntry %s: %v, want it applied (GRAMMAR.md §4.3 (a))", w, err)
			}
			comma := ""
			if slices.Contains(joiningKeys, w) {
				comma = ","
			}
			kwWritten(t, w, op, plan, "  y { w: 2 }"+comma+"\n  "+w+" { w: 4 }\n}\n")
		})
	}
}

// API.md E11, V1; GRAMMAR.md §4.3 (a): Rename of an `entry t.KEY` to every reserved word.
func TestKWEntryRenameDeclEveryWord(t *testing.T) {
	for _, w := range reservedWords {
		t.Run(w, func(t *testing.T) {
			op := edit.Operation{Kind: edit.OpRename, Path: "a:tb.z", Key: edit.Key(w)}
			plan, err := kwApply(t, op)
			if slices.Contains(unreachableKeys, w) {
				kwRefused(t, w, err)
				return
			}
			if err != nil {
				t.Fatalf("Rename a:tb.z to %s: %v, want it applied (GRAMMAR.md §4.3 (a))", w, err)
			}
			kwWritten(t, w, op, plan, "entry tb."+w+" { w: 3 }\n")
		})
	}
}

// API.md E30, V1: RenameName of a field, a function or a `let` to each reserved word is a
// *ValueError (a name), never an internal error.
func TestKWEntryRenameNameEveryWord(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon": file("package a\n\nrecord T {\n  w: Int\n}\n\nfn go(a: Int) -> Int {\n  return a\n}\n\n" +
			"let one: T = { w: 1 }\n"),
	}
	for _, w := range reservedWords {
		for _, path := range []string{"a:T.w", "a:go", "a:one"} {
			t.Run(w+" "+path, func(t *testing.T) {
				s := open(t, fsys, nil, "", "a")
				op := edit.Operation{Kind: edit.OpRenameName, Path: path, Name: w}
				_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
				var ve *edit.ValueError
				if errors.Is(err, edit.ErrInternal) || !errors.As(err, &ve) || ve.Expected != "a name" {
					t.Errorf("RenameName %s to %s: %v, want a *ValueError expecting a name (API.md E30)", path, w, err)
				}
			})
		}
	}
}

// kwJSONFS is a table loaded from a JSON file, and a dev layer file amending it.
var kwJSONFS = mapFS{
	"law/project.canon":     file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon":         file("package a\n\nrecord T {\n  w: Int\n}\n\nlet tb: table T = load(\"rows.json\")\n"),
	"law/a/rows.json":       file("{\n  \"x\": {\n    \"w\": 1\n  }\n}\n"),
	"law/a/dev.layer.canon": file("package a\nlayer dev\n\namend tb {\n  x.w: 2\n}\n"),
}

// GRAMMAR.md §4.3, API.md V1, W11: a key a .canon layer file would hold follows the grammar.
func TestKWEntryJSONTableUnderEditLayer(t *testing.T) {
	for _, w := range unreachableKeys {
		add := edit.Operation{Kind: edit.OpAddEntry, Path: "a:tb", Key: edit.Key(w), Value: edit.Source("{ w: 5 }")}
		s := open(t, kwJSONFS, []string{"dev"}, "dev", "a")
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{add}})
		kwRefused(t, w, err)
		s = open(t, kwJSONFS, nil, "", "a")
		if _, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{add}}); err != nil {
			t.Errorf("%s: AddEntry to the JSON file: %v, want it applied (WIRE.md §6)", w, err)
		}
	}
}

// API.md E11, M6, DECISIONS 459: a Rename that stops an entry joining drops its comma within the regions.
func TestKWEntryRenameStopsJoining(t *testing.T) {
	for _, w := range []string{"and", "for"} {
		t.Run(w, func(t *testing.T) {
			body := "  v { w: 0 }\n  x { w: 1 },\n  " + w + " { w: 2 }\n  z { w: 3 }\n"
			fsys := mapFS{
				"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
				"law/a/a.canon":     file("package a\n\nrecord T {\n  w: Int\n}\n\nlet tb: table T = {\n" + body + "}\n"),
			}
			op := edit.Operation{Kind: edit.OpRename, Path: "a:tb." + w, Key: edit.Key("y")}
			s := open(t, fsys, nil, "", "a")
			plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
			if err != nil {
				t.Fatalf("Rename a:tb.%s to y: %v", w, err)
			}
			for _, ch := range plan.Changes {
				checkWritten(t, plan, ch)
			}
			want := "  v { w: 0 }\n  x { w: 1 }\n  y { w: 2 }\n  z { w: 3 }\n}\n"
			if got := fileText(written(fsys, plan), "law/a/a.canon"); !strings.Contains(got, want) {
				t.Errorf("a/a.canon holds:\n%s\nwant it to hold:\n%s", got, want)
			}
		})
	}
}
