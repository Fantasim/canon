package workspace_test

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
)

// viewLaw's package a heads a record with a view whose subtitle reads b.loaded, a load the
// compiler does not read (DECISIONS 196), and lists and maps whose elements have no view.
var viewLaw = map[string]string{
	"/law/project.canon": "project acme {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n",
	"/law/b/b.canon":     "/// B.\npackage b\n\n/// Loaded.\nlet loaded: [Int] = load.dir(\"x.txt\")\n\n/// A label.\nlet label: String = \"B\"\n",
	"/law/b/x.txt":       "1\n",
	"/law/a/a.canon": `/// A.
package a

import b

/// A use.
record Use {
  /// How many.
  count: Int
}

view Use {
  title "Use {count} of {b.label}"
  subtitle "{b.loaded}"
}

/// Uses.
let uses: [Use] = [{ count: 1 }, { count: 2 }]

/// Tags.
let tags: [String] = ["x", "y"]

/// Sizes.
let sizes: {String: Int} = { "small": 1, "large": 1 }
`,
}

// evaluate is Evaluate of path on s, with a analyzing the packages sel.
func evaluate(t *testing.T, s *workspace.Snapshot, sel []string, path string) (*workspace.Evaluation, error) {
	t.Helper()
	ctx := context.Background()
	a, err := s.Build().Analyze(ctx, sel)
	if err != nil {
		t.Fatal(err)
	}
	es := edit.NewSnapshot(a)
	parsed, err := edit.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	at, err := edit.Resolve(es, parsed)
	if err != nil {
		t.Fatal(err)
	}
	return workspace.Evaluate(ctx, s, workspace.Eval{Analysis: a, Edit: es, At: at})
}

// API.md X2, API.md V11, DECISIONS 196: an unsupported load the view evaluator meets fails Evaluate
// with it (Analysis.ViewErr); a template's own failure does not.
func TestEvaluateSurfacesViewErr(t *testing.T) {
	s := read(t, open(t, newMemFS(viewLaw)))
	if _, err := evaluate(t, s, []string{"a"}, "a:uses[0]"); !errors.Is(err, build.ErrLoad) {
		t.Errorf("API.md X2: Evaluate over a view reading an unsupported load: %v, want ErrLoad", err)
	}
	ev, err := evaluate(t, s, []string{"a"}, "a:tags")
	if err != nil || ev.Package != "a" || len(ev.State.Headings) != 2 {
		t.Errorf("API.md V11: Evaluate(a:tags): %+v, %v", ev, err)
	}
}

// API.md V7, VIEWMODEL.md S9: without a view, a plain element is titled `#<n>` from 1 and a
// map entry by its key, alone as in their collection's headings.
func TestEvaluateTargetNames(t *testing.T) {
	law := maps.Clone(viewLaw)
	law["/law/b/b.canon"] = "/// B.\npackage b\n\n/// A label.\nlet label: String = \"B\"\n\n/// Loaded.\nlet loaded: Int = 1\n"
	s := read(t, open(t, newMemFS(law)))
	for _, c := range []struct{ path, title, owner, key string }{
		{"a:tags[1]", "#2", "a:tags", "[1]"},
		{"a:sizes[large]", "large", "a:sizes", "[large]"},
		{"a:uses[1]", "Use 2 of B", "a:uses", "[1]"},
	} {
		alone, err := evaluate(t, s, nil, c.path)
		if err != nil {
			t.Fatal(err)
		}
		in, err := evaluate(t, s, nil, c.owner)
		if err != nil {
			t.Fatal(err)
		}
		if h := in.State.Headings[c.key]; alone.State.Title.Value != c.title || h.Title != alone.State.Title {
			t.Errorf("API.md V7 %s: alone %+v, in %s %+v, want %q", c.path, alone.State.Title, c.owner, h.Title, c.title)
		}
	}
}
