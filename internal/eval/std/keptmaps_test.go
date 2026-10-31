package std_test

import (
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

// TYPES.md §11.5, STDLIB.md §6, DECISIONS 408: kept maps' reads, unions and compared literals evaluate, bound per key.
func TestKeptDependentMaps(t *testing.T) {
	for _, c := range []struct {
		file string
		want map[string]string
	}{
		{"testdata/depmap_kept.txtar", map[string]string{
			"a.viaField":   `{a: L{lim: 3}, b: L{lim: 5}}`,
			"a.eqField":    `true`,
			"a.eqLetPath":  `false`,
			"a.viaLet":     `{a: L{lim: 3}, b: L{lim: 5}}`,
			"a.viaMapped":  `{a: L{lim: 3}, b: L{lim: 5}}`,
			"a.eqMapped":   `true`,
			"a.viaParam":   `2`,
			"a.eqParam":    `true`,
			"a.eqNested":   `false`,
			"a.viaNested":  `{b: {"k": L{lim: 5}}}`,
			"a.eqInferred": `true`,
		}},
		{"testdata/depmap_union.txtar", map[string]string{
			"a.depUnion": `{a: 1, c: soft, b: "w"}`,
			"a.depEq":    `false`,
			"a.depEmpty": `false`,
			"a.depBoth":  `true`,
		}},
	} {
		a, err := txtar.ParseFile(c.file)
		if err != nil {
			t.Fatal(err)
		}
		out, values := evaluateAll(t, a)
		if !strings.Contains(out, "0 errors") {
			t.Errorf("%s: findings:\n%s", c.file, out)
		}
		for name, want := range c.want {
			if got := values[name]; got != want {
				t.Errorf("%s: %s = %s, want %s", c.file, name, got, want)
			}
		}
	}
}
