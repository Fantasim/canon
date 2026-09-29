package jsonsrc_test

import (
	"bytes"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// newKey is a key no example source uses, for the members the invariant tests insert.
const newKey = "canon.test.new"

// checkEdits checks node n of canonical source src: setting n to itself changes no byte;
// removing it, or inserting a copy of its first item first and last in it, leaves a fixed
// point of Format.
func checkEdits(t *testing.T, name string, src []byte, n *jsonsrc.Node) {
	t.Helper()
	ptr := n.Pointer()
	if out, err := jsonsrc.Rewrite(src, []jsonsrc.Edit{{Kind: jsonsrc.Set, Pointer: ptr, Value: n}}); err != nil || !bytes.Equal(out, src) {
		t.Fatalf("%s: setting %q to itself: %v\n%s", name, ptr, err, out)
	}
	edits := []jsonsrc.Edit{{Kind: jsonsrc.Remove, Pointer: ptr}}
	switch {
	case len(n.Elems) > 0:
		edits = append(edits, jsonsrc.Edit{Kind: jsonsrc.Insert, Pointer: ptr, Value: n.Elems[0]},
			jsonsrc.Edit{Kind: jsonsrc.Insert, Pointer: ptr, At: len(n.Elems), Value: n.Elems[0]})
	case len(n.Members) > 0:
		edits = append(edits, jsonsrc.Edit{Kind: jsonsrc.Insert, Pointer: ptr, Key: newKey, Value: n.Members[0].Value},
			jsonsrc.Edit{Kind: jsonsrc.Insert, Pointer: ptr, Key: newKey, At: len(n.Members), Value: n.Members[0].Value})
	}
	for _, e := range edits {
		out, err := jsonsrc.Rewrite(src, []jsonsrc.Edit{e})
		if errors.Is(err, jsonsrc.ErrEdit) && ptr == "" {
			continue
		}
		if err != nil {
			t.Fatalf("%s: %+v: %v", name, e, err)
		}
		if again := jsonsrc.Format(mustParse(t, string(out)).root); !bytes.Equal(again, out) {
			t.Fatalf("%s: %+v: not a fixed point:\n%s\n%s", name, e, out, again)
		}
		if lo, hi := unitOf(t, src, e); !bytes.HasPrefix(out, src[:lo]) || !bytes.HasSuffix(out[lo:], src[hi:]) {
			t.Fatalf("%s: %+v: bytes outside %d to %d changed (API.md M6):\n%s", name, e, lo, hi, out)
		}
	}
}

// unitOf is the bytes an edit of a normalized source may change: a removed item with the ","
// before it, or after it for the first one; the point a new item goes; a container left
// empty, or empty before an insert, whole.
func unitOf(t *testing.T, src []byte, e jsonsrc.Edit) (lo, hi int) {
	// API.md M6, FORMATTER.md §14.2
	t.Helper()
	root := mustParse(t, string(src)).root
	c, at := root.Find(e.Pointer), e.At
	if e.Kind == jsonsrc.Remove {
		c = root.Find(e.Pointer[:strings.LastIndex(e.Pointer, "/")])
	}
	starts, ends := itemBounds(c)
	switch {
	case len(ends) == 0 || e.Kind == jsonsrc.Remove && len(ends) == 1:
		return int(c.Span.Start), int(c.Span.End)
	case e.Kind == jsonsrc.Insert && at == 0:
		return int(c.Span.Start) + 1, int(c.Span.Start) + 1
	case e.Kind == jsonsrc.Insert:
		return ends[at-1], ends[at-1]
	}
	i := slices.IndexFunc(ends, func(end int) bool { return end == int(root.Find(e.Pointer).Span.End) })
	if i == 0 {
		return starts[0], starts[1]
	}
	return ends[i-1], ends[i]
}

// itemBounds are where each member (its key) or element of a container starts and ends.
func itemBounds(c *jsonsrc.Node) (starts, ends []int) {
	for _, m := range c.Members {
		starts, ends = append(starts, int(m.KeySpan.Start)), append(ends, int(m.Value.Span.End))
	}
	for _, v := range c.Elems {
		starts, ends = append(starts, int(v.Span.Start)), append(ends, int(v.Span.End))
	}
	return starts, ends
}

// FORMATTER.md §14.2: checkEdits on every node of every normalized example source.
func TestRewriteExampleSources(t *testing.T) {
	files := exampleSources(t)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if slices.Contains(notNormalized, name) {
			continue
		}
		src := files[name]
		walk(mustParse(t, string(src)).root, func(n *jsonsrc.Node) { checkEdits(t, name, src, n) })
	}
}

// FuzzRewrite is checkEdits on a node of any normalized source (IMPLEMENTATION-PLAN.md §7.7).
func FuzzRewrite(f *testing.F) {
	sources := exampleSources(f)
	for i, name := range slices.Sorted(maps.Keys(sources)) {
		f.Add(sources[name], uint16(i))
	}
	f.Fuzz(func(t *testing.T, data []byte, pick uint16) {
		p := parse(t, string(data))
		if p.err != nil {
			return
		}
		src := jsonsrc.Format(p.root)
		var all []*jsonsrc.Node
		walk(mustParse(t, string(src)).root, func(n *jsonsrc.Node) { all = append(all, n) })
		checkEdits(t, "fuzz", src, all[int(pick)%len(all)])
	})
}
