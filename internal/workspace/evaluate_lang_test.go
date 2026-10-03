package workspace_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
)

// langLaw's package a has a named warning translated into French and an unnamed one.
var langLaw = map[string]string{
	"/law/project.canon": "project acme {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n",
	"/law/a/a.canon": `/// A.
package a

/// A use.
record Use {
  /// How many.
  count: Int

  warn big: count < 10 else "count {count} is big"
  warn count < 8 else "count {count} is over eight"
}

/// Uses.
let uses: table Use = {
  first { count: 20 }
}
`,
	"/law/a/a.fr.canon": "package a\ntranslation fr\n\nUse.check.big \"compte {count} trop grand\"\n",
}

// evaluateIn is Evaluate of path over package a in lang, the language API.md §11 resolved.
func evaluateIn(t *testing.T, lang, path string) map[string]string {
	t.Helper()
	ctx := context.Background()
	s := read(t, open(t, newMemFS(langLaw)))
	a, err := s.Build().Analyze(ctx, []string{"a"})
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
	ev, err := workspace.Evaluate(ctx, s, workspace.Eval{Analysis: a, Edit: es, At: at, Lang: lang})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range ev.Findings {
		out[f.Check] = f.Message
	}
	return out
}

// I18N.md B5, K8, API.md §11, DECISIONS 281: a live named-check finding follows the Evaluate's language.
func TestEvaluateFindingsLang(t *testing.T) {
	for _, c := range []struct{ lang, big string }{
		{"fr", "compte 20 trop grand"},
		{"", "count 20 is big"},
		{"en", "count 20 is big"},
		{"xx", "count 20 is big"},
	} {
		got := evaluateIn(t, c.lang, "a:uses[first]")
		if got["big"] != c.big || got[""] != "count 20 is over eight" {
			t.Errorf("lang %q: findings %v, want big %q", c.lang, got, c.big)
		}
	}
}
