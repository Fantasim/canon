package canon_test

import (
	"context"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// originTexts is each origin's kind, layer and text at path, newest first (API.md §5.2).
func originTexts(t *testing.T, p *canon.Project, path string) []string {
	t.Helper()
	v, err := p.Value(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for o := &v.Origin; o != nil; o = o.Replaced {
		out = append(out, string(o.Kind)+" "+o.Layer+" "+o.Text)
	}
	return out
}

// API.md §5.2 Origin.Text, EVALUATION.md §9.3 step 4: an origin's text precedes dev's amendment, though verification rebuilt it.
func TestOriginTextSurvivesVerificationCopies(t *testing.T) {
	p, _ := depOpen(t, true)
	const base = "literal  Q{items: false, rewards: [], best: none}"
	for _, c := range []struct {
		path string
		want []string
	}{
		{"d:q", []string{base}},
		{"d:q.items", []string{"layer dev true", "literal  false"}},
		{"d:qs.a", []string{base}},
	} {
		path, want := c.path, c.want
		got := originTexts(t, p, path)
		if len(got) != len(want) {
			t.Errorf("%s origins %q, want %q", path, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s origin %d = %q, want %q", path, i, got[i], want[i])
			}
		}
	}
}

const (
	narrowSrc = "/// N.\npackage n\n\n/// A number: narrow, or whole.\ntype N(f: Bool) = match f {\n  true => Float32\n  false => Int\n}\n\n" +
		"/// X.\nrecord X {\n  /// Whether n is a float.\n  f: Bool = true\n  /// The number.\n  n: N(f)\n  /// Unrelated.\n  k: Int = 0\n}\n\n" +
		"/// The x.\nlet x: X = { n: 0.123456789123 }\n\n" +
		"/// P.\nrecord P {\n  /// A plain narrow number.\n  v: Float32\n}\n\n/// The p.\nlet p: P = { v: 0.123456789123 }\n"
	narrowDev = "package n\nlayer dev\n\namend x {\n  k: 1\n}\n"
)

// DECISIONS 468, EVALUATION.md §4.3: an origin's text is the value evaluated, a dependent n as written, a plain Float32 rounded, with or without dev.
func TestOriginTextBeforeVerification(t *testing.T) {
	for _, layers := range [][]string{nil, {"dev"}} {
		opts := project(map[string]string{"project.canon": editLaw["project.canon"], "n/n.canon": narrowSrc, "n/dev.layer.canon": narrowDev})
		opts.Layers = layers
		p, err := canon.Open("/law", opts)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct{ path, want string }{
			{"n:x", "literal  X{f: true, n: 0.123456789123, k: 0}"},
			{"n:x.n", "literal  0.123456789123"},
			{"n:p", "literal  P{v: 0.12345679}"},
			{"n:p.v", "literal  0.12345679"},
		} {
			if got := originTexts(t, p, c.path); len(got) != 1 || got[0] != c.want {
				t.Errorf("layers %v: %s origins %q, want [%q]", layers, c.path, got, c.want)
			}
		}
		_ = p.Close()
	}
}
