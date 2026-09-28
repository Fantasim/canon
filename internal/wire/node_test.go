package wire

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// referencePretty is a second, independent implementation of pretty(v, depth × 2) (WIRE.md §7.4), checked against toJSON's jsonsrc.Node in TestToJSONPretty.
func referencePretty(n *node, depth int) string {
	inner, outer := strings.Repeat(indentSpaces, depth+1), strings.Repeat(indentSpaces, depth)
	switch {
	case n.array && len(n.elems) == 0:
		return "[]"
	case n.isObject() && len(n.members) == 0:
		return "{}"
	case n.array:
		parts := make([]string, len(n.elems))
		for i, e := range n.elems {
			parts[i] = inner + referencePretty(e, depth+1)
		}
		return "[\n" + strings.Join(parts, ",\n") + "\n" + outer + "]"
	case n.isObject():
		parts := make([]string, len(n.members))
		for i, m := range n.members {
			parts[i] = fmt.Sprintf("%s%q: %s", inner, m.key, referencePretty(m.val, depth+1))
		}
		return "{\n" + strings.Join(parts, ",\n") + "\n" + outer + "}"
	default:
		return string(n.raw)
	}
}

// randomNode draws a random node tree of scalars, arrays and objects, plain ASCII keys and text.
func randomNode(r *rand.Rand, depth int) *node {
	const maxDepth, maxChildren, scalarBound = 4, 4, 1000
	if depth >= maxDepth || r.IntN(3) == 0 {
		return scalar([]byte(strconv.Itoa(r.IntN(scalarBound))))
	}
	n := r.IntN(maxChildren)
	if r.IntN(2) == 0 {
		a := arrayNode(n)
		for i := 0; i < n; i++ {
			a.elems = append(a.elems, randomNode(r, depth+1))
		}
		return a
	}
	o := objectNode()
	for i := 0; i < n; i++ {
		o.add(fmt.Sprintf("k%d", i), randomNode(r, depth+1))
	}
	return o
}

// TestToJSONPretty proves toJSON's jsonsrc.Node prints, through jsonsrc.Format, the same bytes as an independent pretty(v, 0) (WIRE.md §7.4) on 2000 random trees.
func TestToJSONPretty(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 2000 {
		n := randomNode(r, 0)
		want := referencePretty(n, 0) + "\n"
		if got := string(jsonsrc.Format(n.toJSON())); got != want {
			t.Fatalf("tree %d: got %s, want %s", i, got, want)
		}
	}
}
