package edit_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
	"golang.org/x/tools/txtar"
)

const (
	spreadOwned   = "../eval/testdata/prov/spread_owned.txtar"
	spreadOwnedIn = "a/a.canon"
	spreadSrc     = "/// D.\npackage d\n\n/// Tier.\nrecord Tier {\n  /// Weight.\n  weight: Int = 1\n}\n\n" +
		"/// Box.\nrecord Box {\n  /// Tags.\n  tags: [String]\n  /// Inner.\n  inner: Tier\n  /// N.\n  n: Int\n}\n\n" +
		"/// The base.\nlet base: Box = { tags: [\"a\"], inner: { weight: 2 }, n: 1 }\n\n" +
		"/// Derived.\nlet derived: Box = { ...base, n: 2 }\n\n" +
		"/// Reads its tags.\nlet readTags: [String] = derived.tags\n\n" +
		"/// Reads its inner.\nlet readInner: Tier = derived.inner\n"
	spreadDev = "package d\nlayer dev\n\namend derived {\n  n: 4\n}\n"
)

// API.md R7, R11; EVALUATION.md §4.2, DECISIONS 199, 410: the entries a spread copy holds are its source's, whatever root is verified last.
func TestSpreadCopyGraphAndRefs(t *testing.T) {
	a, err := txtar.ParseFile(spreadOwned)
	if err != nil {
		t.Fatal(err)
	}
	var src []byte
	for _, f := range a.Files {
		if f.Name == spreadOwnedIn {
			src = f.Data
		}
	}
	s := open(t, mapFS{"law/project.canon": file(projectCanon), "law/" + spreadOwnedIn: file(string(src))}, nil, "", "a")
	g, err := s.snap.Graph(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, n := range g.Nodes {
		ids = append(ids, n.ID)
	}
	for _, id := range []string{"a:base.many.items[i1]", "a:base.many.items[i2]", "a:base.h.items[j1]", "a:over.many.items[i2]", "a:ta.a.items[i1]"} {
		if !slices.Contains(ids, id) {
			t.Errorf("graph: no node %s in %v", id, ids)
		}
	}
	for _, id := range ids {
		if strings.HasPrefix(id, "a:copy.") || strings.HasPrefix(id, "a:again.") || strings.HasPrefix(id, "a:tc.a.") {
			t.Errorf("graph: node %s, an entry its source holds", id)
		}
	}
	want := refPaths(t, s, "a:base.many.items[i2]")
	for _, path := range []string{"a:copy.many.items[i2]", "a:again.many.items[i2]"} {
		if got := refPaths(t, s, path); !slices.Equal(got, want) || !slices.Contains(got, "base.many.pick") {
			t.Errorf("Refs(%s) = %v, want %v naming base.many.pick", path, got, want)
		}
	}
}

// refPaths is the path of each reference Refs lists to path's entry.
func refPaths(t *testing.T, s session, path string) []string {
	t.Helper()
	refs, err := s.snap.Refs(context.Background(), resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, path))
	if err != nil {
		t.Fatalf("Refs(%s): %v", path, err)
	}
	var out []string
	for _, r := range refs {
		out = append(out, r.Path)
	}
	return out
}

// API.md §7.2, EVALUATION.md §13: a let computed from a spread-copied composite names that copy, not the spread's source, with and without a layer.
func TestSpreadCopyComputedOrigin(t *testing.T) {
	fs := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(spreadSrc), "law/d/dev.layer.canon": file(spreadDev)}
	cases := []struct {
		path, value, origin string
	}{
		{"d:readTags", `["x"]`, "d:derived.tags"},
		{"d:readInner", "{ weight: 3 }", "d:derived.inner"},
	}
	for _, layers := range [][]string{nil, {"dev"}} {
		s := open(t, fs, layers, "", "d")
		for _, c := range cases {
			e := mustEdit(t, fixture{Snapshot: s.snap, files: s.a.Files()}, resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, c.path), edit.OpSet, "")
			_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpSet, Path: c.path, Value: edit.Source(c.value)}}})
			var ne *edit.NotEditableError
			if e.Reason != edit.ReasonComputed || e.Origin != c.origin || !errors.As(err, &ne) || ne.Reason != edit.ReasonComputed || ne.Origin != c.origin {
				t.Errorf("%v %s: Editable %v origin %q, Apply %v; want computed naming %s", layers, c.path, e.Reason, e.Origin, err, c.origin)
			}
		}
	}
}
