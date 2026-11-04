package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// rederived: dev amends server.port; re-derived after it, host reads nothing it changed, url the port (EVALUATION.md §9.3).
var rederived = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon": file("/// A.\npackage a\n\n/// Server.\nrecord Server {\n  /// Port.\n  port: Int = 1\n" +
		"  /// Host.\n  host: String = \"localhost\"\n  /// URL.\n  url: String = \"http://{host}:{port}\"\n}\n\n" +
		"/// Server.\nlet server: Server = {}\n"),
	"law/a/dev.layer.canon": file("package a\nlayer dev\n\namend server {\n  port: 9000\n}\n"),
}

const rederivedLayer = "a/dev.layer.canon"

// API.md W11: no layer text states a re-derived default, dependent (url) or not (host); a Set adds a line, a Reset writes nothing.
func TestLayerRederivedDefaults(t *testing.T) {
	p, err := build.Open(rederived, "/law", build.Options{Layers: []string{"dev"}})
	if err != nil {
		t.Fatal(err)
	}
	f := analyze(t, p, []string{"a"}, true)
	for _, c := range []struct {
		path, line   string
		stated, kept bool
	}{
		{path: "a:server.host", line: "  host: \"x\"\n", kept: true},
		{path: "a:server.url", line: "  url: \"x\"\n", kept: true},
	} {
		r := resolve(t, f, c.path)
		if stated, kept := edit.LayerStates(f.Snapshot, r, "dev"); stated != c.stated || kept != c.kept {
			t.Errorf("%s: stated %v, kept %v; want %v, %v", c.path, stated, kept, c.stated, c.kept)
		}
		for _, op := range []edit.Op{edit.OpSet, edit.OpReset} {
			if e := mustEdit(t, f, r, op, "dev"); e.Mode != edit.ModeCanon || e.Reason != edit.ReasonNone || e.File != rederivedLayer {
				t.Errorf("%s op %d: %+v, want the edit layer's file", c.path, op, e)
			}
		}
		set := "package a\nlayer dev\n\namend server {\n  port: 9000\n" + c.line + "}\n"
		rederivedWrites(t, edit.Operation{Kind: edit.OpSet, Path: c.path, Value: edit.Str("x")}, set)
		rederivedWrites(t, edit.Operation{Kind: edit.OpReset, Path: c.path}, "")
	}
}

// rederivedWrites applies op under the dev edit layer: the layer file then reads want, "" for no write, and the base is untouched.
func rederivedWrites(t *testing.T, op edit.Operation, want string) {
	t.Helper()
	s := open(t, rederived, []string{"dev"}, "dev", "a")
	plan, err := edit.Apply(t.Context(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	if err != nil {
		t.Fatalf("%s op %d: %v", op.Path, op.Kind, err)
	}
	got := ""
	if ws := edit.Writes(plan, rederivedLayer); len(ws) > 0 {
		got = string(ws[len(ws)-1].After)
	}
	if got != want || len(edit.Writes(plan, "a/a.canon")) > 0 {
		t.Errorf("%s op %d: layer file %q, base writes %d; want %q, none", op.Path, op.Kind, got, len(edit.Writes(plan, "a/a.canon")), want)
	}
}
