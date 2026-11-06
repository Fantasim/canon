package canon_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// rulesSharedSource is package s: a chain of lists w1…w<depth>, each level holding the level below
// twice, over w0 holding one Leaf instance, written as leaf0, whose check fails.
func rulesSharedSource(depth int, leaf0 string) string {
	var src strings.Builder
	src.WriteString("/// S.\npackage s\n\n/// Leaf.\nrecord Leaf {\n  /// N.\n  n: Int = 1\n\n  check n > 1 else \"small\"\n}\n\n")
	fmt.Fprintf(&src, "/// Leaf.\nlet leaf: Leaf = {}\n\n/// W0.\nlet w0: [Leaf] = [%s]\n", leaf0)
	typ := "[Leaf]"
	for i := 1; i <= depth; i++ {
		typ = "[" + typ + "]"
		fmt.Fprintf(&src, "\n/// W%d.\nlet w%d: %s = [w%d, w%d]\n", i, i, typ, i-1, i-1)
	}
	return src.String()
}

// rulesSharedFindings are the code and path of each finding of package s at depth, w0 holding leaf0.
func rulesSharedFindings(t *testing.T, depth int, leaf0 string) []string {
	t.Helper()
	p, err := canon.Open("/law", project(map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"s/s.canon":     rulesSharedSource(depth, leaf0),
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	res, err := p.Check(context.Background(), "s")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range res.Findings {
		out = append(out, f.Code+" "+f.Path)
	}
	return out
}

// EVALUATION.md §8.1, §8.3, DECISIONS 504, 507: the Leaf wN holds 2^N times is checked once, at its first place.
func TestRulesSharedFirstPlace(t *testing.T) {
	small := string(diag.E5001.Def().Code)
	for _, c := range []struct {
		leaf0 string
		want  []string
	}{
		{"leaf", []string{small + " leaf"}},
		{"{}", []string{small + " leaf", small + " w0[0]"}},
	} {
		if got := rulesSharedFindings(t, 12, c.leaf0); !slices.Equal(got, c.want) {
			t.Errorf("w0 = [%s]: findings %q, want %q", c.leaf0, got, c.want)
		}
	}
}
