package edit

import (
	"bytes"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// jsonTree is a JSON text for its regions: its document, the items each container loses in
// the write, and, as the edits apply, each container's items (slots) by pointer.
type jsonTree struct {
	src     []byte
	root    *jsonsrc.Node
	removed map[string][]int
	slots   map[string][]slot
}

// slot is an item of a container as the edits apply: an original item, or one inserted
// (orig -1) whose text goes at point.
type slot struct {
	orig, point int
}

// jsonRegions are where the edits of the JSON text src write, taken in the order they apply:
// a set node or a renamed key, or around an inserted or removed item its separator.
func jsonRegions(src []byte, order []jsonEdit) []region {
	content, root, err := parseJSONText(src)
	if err != nil {
		return []region{{lo: 0, hi: len(src), kind: regionAny}}
	}
	t := &jsonTree{src: content, root: root, removed: map[string][]int{}, slots: map[string][]slot{}}
	for _, e := range order {
		if e.rename == nil && e.e.Kind == jsonsrc.Remove {
			holder := parentPointer(e.e.Pointer)
			t.removed[holder] = append(t.removed[holder], childIndex(root.Find(holder), root.Find(e.e.Pointer)))
		}
	}
	out := make([]region, 0, len(order))
	for _, e := range order {
		out = append(out, t.region(e))
	}
	return out
}

func (t *jsonTree) region(e jsonEdit) region {
	whole := region{lo: 0, hi: len(t.src), kind: regionAny}
	switch {
	case e.rename != nil:
		return keyRegion(t.root, e.rename, whole)
	case e.e.Kind == jsonsrc.Set:
		return nodeRegion(t.root.Find(e.e.Pointer), whole)
	}
	ptr, at := e.e.Pointer, e.e.At
	if e.e.Kind == jsonsrc.Remove {
		ptr = parentPointer(ptr)
		at = childIndex(t.root.Find(ptr), t.root.Find(e.e.Pointer))
	}
	holder := t.root.Find(ptr)
	switch {
	case holder == nil || at < 0:
		return whole
	case t.relaid(ptr, holder):
		return nodeRegion(holder, whole)
	case e.e.Kind == jsonsrc.Remove:
		t.drop(ptr, holder, at)
		return t.removeRegion(ptr, jsonItems(holder), at)
	}
	return t.insertRegion(ptr, holder, at)
}

// relaid reports the container at ptr printed again whole: on one line, or emptied.
func (t *jsonTree) relaid(ptr string, holder *jsonsrc.Node) bool {
	return len(t.removed[ptr]) >= len(jsonItems(holder)) || !bytes.Contains(t.src[holder.Span.Start:holder.Span.End], []byte(newline))
}

// slotsOf are the container's items as the edits so far left them.
func (t *jsonTree) slotsOf(ptr string, holder *jsonsrc.Node) []slot {
	if s, ok := t.slots[ptr]; ok {
		return s
	}
	var s []slot
	for i, it := range jsonItems(holder) {
		s = append(s, slot{orig: i, point: it.lo})
	}
	return s
}

// drop takes original item at out of the container's items.
func (t *jsonTree) drop(ptr string, holder *jsonsrc.Node, at int) {
	t.slots[ptr] = slices.DeleteFunc(t.slotsOf(ptr, holder), func(s slot) bool { return s.orig == at })
}

// removeRegion is removed item at of a container: through the next item's start, or, in the
// run of removed items that ends the container, from the end of the item before (its comma).
func (t *jsonTree) removeRegion(ptr string, items []region, at int) region {
	for k := at + 1; k < len(items); k++ {
		if !slices.Contains(t.removed[ptr], k) {
			return region{lo: items[at].lo, hi: items[at+1].lo, kind: regionGone}
		}
	}
	return region{lo: items[at-1].hi, hi: items[at].hi, kind: regionGone}
}

// insertRegion is the point an item inserted at at, among the items the edits so far left,
// goes: before the item at, or after the last.
func (t *jsonTree) insertRegion(ptr string, holder *jsonsrc.Node, at int) region {
	s := t.slotsOf(ptr, holder)
	items := jsonItems(holder)
	var point int
	switch {
	case len(s) == 0:
		return nodeRegion(holder, region{lo: 0, hi: len(t.src), kind: regionAny})
	case at < len(s):
		point = s[at].point
	case s[len(s)-1].orig < 0:
		point = s[len(s)-1].point
	default:
		point = items[s[len(s)-1].orig].hi
	}
	t.slots[ptr] = slices.Insert(s, min(at, len(s)), slot{orig: -1, point: point})
	return region{lo: point, hi: point, kind: regionItem}
}

// keyRegion is the key a rename writes again.
func keyRegion(root *jsonsrc.Node, rn *keyRename, whole region) region {
	holder := root.Find(rn.holder)
	if holder == nil {
		return whole
	}
	for _, m := range holder.Members {
		if m.Key == rn.from {
			return region{lo: int(m.KeySpan.Start), hi: int(m.KeySpan.End), kind: regionNode}
		}
	}
	return whole
}

func nodeRegion(n *jsonsrc.Node, whole region) region {
	if n == nil {
		return whole
	}
	return region{lo: int(n.Span.Start), hi: int(n.Span.End), kind: regionNode}
}

// jsonItems are a container's members or elements.
func jsonItems(n *jsonsrc.Node) []region {
	var out []region
	for _, m := range n.Members {
		out = append(out, region{lo: int(m.KeySpan.Start), hi: int(m.Value.Span.End), kind: regionGone})
	}
	for _, e := range n.Elems {
		out = append(out, region{lo: int(e.Span.Start), hi: int(e.Span.End), kind: regionGone})
	}
	return out
}

// childIndex is child's position in holder, -1 for none.
func childIndex(holder, child *jsonsrc.Node) int {
	if holder == nil {
		return -1
	}
	for i, m := range holder.Members {
		if m.Value == child {
			return i
		}
	}
	for i, e := range holder.Elems {
		if e == child {
			return i
		}
	}
	return -1
}

// parentPointer is the pointer of the container of the node at ptr.
func parentPointer(ptr string) string {
	return ptr[:max(strings.LastIndex(ptr, pointerSep), 0)]
}
