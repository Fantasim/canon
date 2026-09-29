package canon_test

import (
	"cmp"
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// refLaw holds a table whose entries are named by values in two packages, a map key and code,
// and an enum whose member a value holds.
var refLaw = map[string]string{
	"project.canon": "project demo {\n  canon: \"0.1\"\n}\n",
	"a/a.canon": `/// A.
package a

/// A mood.
enum Mood { calm, angry }

/// A status.
record Status {
  /// Its label.
  label: String
  /// Its mood.
  mood: Mood
}

/// The statuses.
let statuses: table Status = {
  open { label: "Open", mood: calm }
  done { label: "Done", mood: angry }
}

/// The first status.
let first: ref statuses = open

/// Whether s is the open status.
fn isFirst(s: ref statuses) -> Bool {
  return s == open or s == statuses.open
}
`,
	"b/b.canon": `/// B.
package b

import a

/// A board.
record Board {
  /// Where a card starts.
  start: ref a.statuses
  /// Weights by status.
  weights: {ref a.statuses: Int}
}

/// The board.
let board: Board = { start: open, weights: { done: 2 } }
`,
}

// refsOf is the kind, package and path or file:line of each of p's refs to path.
func refsOf(t *testing.T, p *canon.Project, path string) (string, []string) {
	t.Helper()
	res, err := p.Refs(context.Background(), path)
	if err != nil {
		t.Fatalf("Refs(%s): %v", path, err)
	}
	var out []string
	for _, r := range res.Refs {
		out = append(out, string(r.Kind)+" "+r.Package+" "+r.Path+" "+r.File)
	}
	return res.Target, out
}

// API.md R7: the refs of an entry in every package of the project, values and map keys by path,
// code by file; the target comes back canonical.
func TestRefsAcrossPackages(t *testing.T) {
	p, _ := openLaw(t, refLaw)
	target, got := refsOf(t, p, "statuses[open]")
	want := []string{"value a first a/a.canon", "code a  a/a.canon", "code a  a/a.canon", "value b board.start b/b.canon"}
	if target != "a:statuses.open" || !slices.Equal(got, want) {
		t.Errorf("API.md R7: %s %q, want %q", target, got, want)
	}
	_, got = refsOf(t, p, "a:statuses.done")
	if want := []string{"key b board.weights[done] b/b.canon"}; !slices.Equal(got, want) {
		t.Errorf("API.md R7 map key: %q, want %q", got, want)
	}
	_, got = refsOf(t, p, "a:Mood.angry")
	if want := []string{"value a statuses.done.mood a/a.canon"}; !slices.Equal(got, want) {
		t.Errorf("API.md R7, API.md P7a member: %q, want %q", got, want)
	}
}

// API.md R7, API.md R8 over the examples: a vocab event type is referenced from heistia, another
// package, and the refs come in F2 order of their spans.
func TestRefsExamples(t *testing.T) {
	p, _ := openViewExamples(t)
	heistia := mustValue(t, p, "resource.heistia:heistia.tasks[0].eventType")
	key, ok := heistia.Key()
	if !ok {
		t.Fatalf("no key in %s", heistia.Path)
	}
	res, err := p.Refs(context.Background(), "resource.vocab:eventTypes["+key+"]")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(res.Refs, func(r canon.Ref) bool { return r.Package == "resource.heistia" && r.Kind == canon.RefValue }) {
		t.Errorf("API.md R7: no ref from resource.heistia among %+v", res.Refs)
	}
	sorted := slices.IsSortedFunc(res.Refs, func(a, b canon.Ref) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	})
	if !sorted {
		t.Errorf("API.md R8: refs out of F2 order: %+v", res.Refs)
	}
}

// API.md R7, API.md R6: a path naming no entry, keyed element or member is ErrBadOp; a value
// that could hold a ref but was not computed fails the call naming it, with its cause.
func TestRefsErrors(t *testing.T) {
	p := openValueLaw(t)
	for _, c := range []struct {
		path string
		want error
	}{
		{"config", canon.ErrBadOp},
		{"nums[0]", canon.ErrBadOp},
		{"gen.key", canon.ErrBadOp},
		{"config..x", canon.ErrBadPath},
		{"nowhere", canon.ErrNoPath},
	} {
		var pe *canon.PathError
		if _, err := p.Refs(context.Background(), c.path); !errors.Is(err, c.want) || !errors.As(err, &pe) || pe.Path != c.path {
			t.Errorf("API.md R7 %s: %v, want %v", c.path, err, c.want)
		}
	}
	law := map[string]string{"bad/bad.canon": "/// Bad.\npackage bad\n\nimport a\n\n/// A pool.\nlet pool: [ref a.statuses] = [a.statuses.open]\n\n/// Broken.\nlet broken: ref a.statuses = pool[5]\n"}
	maps.Copy(law, refLaw)
	bad, _ := openLaw(t, law)
	_, err := bad.Refs(context.Background(), "a:statuses.open")
	var pe *canon.PathError
	if !errors.Is(err, canon.ErrNoValue) || !errors.As(err, &pe) || pe.Detail != "root bad:broken" || len(pe.Findings) == 0 {
		t.Errorf("API.md R7: a poisoned let that could hold the ref: %v", err)
	}
}
