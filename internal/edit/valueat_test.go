package edit_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

// questsLaw states refs into a record field's collection, in a literal and in loaded JSON; early,
// declared first, holds an entry in a plain list, where it is no entry.
const questsLaw = `/// P.
package p

/// A step.
record Step {
  /// After it.
  next: [ref Step] = []
}

/// A quest.
record Quest {
  /// Its steps.
  steps: table Step
}

/// Early.
let early: [Step] = [quests.q1.steps.b]

/// Quests.
let quests: table Quest = {
  q1 { steps: { a { next: [b] }, b {} } },
  q2 { steps: { a {}, b { next: [a] } } },
}

/// Loaded.
let more: table Quest = load("more.json")
`

const questsJSON = `{"q3": {"steps": {"a": {"next": ["b"]}, "b": {}}}}
`

// defaultsLaw states one ref in a field default, evaluated once per quest.
const defaultsLaw = `/// P.
package p

/// A step.
record Step {
  /// After it.
  next: [ref Step] = [b]
}

/// A quest.
record Quest {
  /// Its steps.
  steps: table Step
}

/// Quests.
let quests: table Quest = {
  q1 { steps: { a {}, b { next: [] } } },
  q2 { steps: { a {}, b { next: [] } } },
}
`

// ValuesAt finds the refs a source states, EntriesOf their entries, each in its ref's own instance.
func TestValuesAtEntriesOf(t *testing.T) {
	// API.md R7; IMPLEMENTATION-PLAN §8.4 Features; TYPES.md §10.2; DECISIONS 285
	quests := mapFS{"law/project.canon": file(projectCanon), "law/p/p.canon": file(questsLaw), "law/p/more.json": file(questsJSON)}
	defaults := mapFS{"law/project.canon": file(projectCanon), "law/p/p.canon": file(defaultsLaw)}
	for _, c := range []struct {
		fsys             mapFS
		file, text, near string
		want             []string
	}{
		{quests, "p/p.canon", questsLaw, "[b]", []string{"p:quests.q1.steps.b"}},
		{quests, "p/p.canon", questsLaw, "[a]", []string{"p:quests.q2.steps.a"}},
		{quests, "p/more.json", questsJSON, `["b"]`, []string{"p:more.q3.steps.b"}},
		{defaults, "p/p.canon", defaultsLaw, "[b]", []string{"p:quests.q1.steps.b", "p:quests.q2.steps.b"}},
	} {
		p, err := build.Open(c.fsys, "/law", build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		f := analyze(t, p, []string{"p"}, true)
		if got := entriesAt(t, f, c.file, strings.Index(c.text, c.near)+1); !slices.Equal(got, c.want) {
			t.Errorf("%s %s: entries %q, want %q", c.file, c.near, got, c.want)
		}
	}
}

// A value an amendment replaced is still found where its base JSON states it.
func TestValuesAtReplaced(t *testing.T) {
	// DECISIONS 285; API.md R7
	f := refsFixture(t)
	if got := entriesAt(t, f, "r/s.json", strings.Index("{\"first\": \"done\"}\n", "done")); !slices.Equal(got, []string{"r:statuses.done"}) {
		t.Errorf("the replaced ref names %q, want r:statuses.done", got)
	}
	if v, err := f.ValuesAt(context.Background(), fileID(f, "r/r.canon"), 0); v != nil || err != nil {
		t.Errorf("no value is stated at a package clause: %v, %v", v, err)
	}
}

// entriesAt are the canonical paths of the entries the refs at off of file name.
func entriesAt(t *testing.T, f fixture, file string, off int) []string {
	t.Helper()
	ctx := context.Background()
	vs, err := f.ValuesAt(ctx, fileID(f, file), off)
	if err != nil {
		t.Fatal(err)
	}
	var refs []*value.Ref
	for _, v := range vs {
		if ref, ok := v.(*value.Ref); ok {
			refs = append(refs, ref)
		}
	}
	rs, err := f.EntriesOf(ctx, refs)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range rs {
		out = append(out, r.Canonical)
		if _, err := f.Refs(ctx, r); err != nil {
			t.Errorf("Refs of %s: %v", r.Canonical, err)
		}
	}
	return out
}

// fileID is the id of the file of display path name.
func fileID(f fixture, name string) source.FileID {
	for id := source.NoFile + 1; f.files.Path(id) != ""; id++ {
		if f.files.Path(id) == name {
			return id
		}
	}
	return source.NoFile
}
