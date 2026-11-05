package canon_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// heldTwiceSrc is two, whose l holds one record twice: dup returns its argument at both places.
const heldTwiceSrc = "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\nrecord I {\n  id: String\n  oo: [S] keyed by k\n}\n\n" +
	"record T2 {\n  l: [I]\n}\n\nfn dup(i: I) -> [I] {\n  return [i, i]\n}\n\n" +
	"let two: T2 = { l: dup({ id: \"t\", oo: [{ k: \"a\" }, { k: \"b\" }] }) }\n"

// API.md R11 (DECISIONS 478, 486, 492): of the two places holding one instance, l[0] is its own;
// a layer line amending l[0] makes it an amended copy that keeps the instance's untouched entries,
// so their nodes stay at p:two.l[0].oo[…], active or not, and none lands at the copy's place l[1].
func TestGraphAmendedFirstPlaceKeepsNodes(t *testing.T) {
	want := []string{"element p:two.l[0].oo[a]", "element p:two.l[0].oo[b]"}
	files := map[string]string{srcMain: formatted(t, srcMain, heldTwiceSrc), sharedLayer: formatted(t, sharedLayer, "package p\nlayer dev\n\namend two {\n  l[0].id: \"z\"\n}\n")}
	for _, active := range []bool{false, true} {
		var layers []string
		if active {
			layers = []string{"dev"}
		}
		t.Run(fmt.Sprintf("active %v", active), func(t *testing.T) {
			p, _ := sharedOpen(t, files, layers)
			nodes, _ := graphOf(t, p)
			var got []string
			for _, n := range nodes {
				if n.Package == "p" && strings.HasPrefix(n.ID, "p:two.") {
					got = append(got, fmt.Sprintf("%s %s", n.Kind, n.ID))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("API.md R11, DECISIONS 478, 492: Graph nodes below two are\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}
