package jsonsrc

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// EditKind is what an Edit does (FORMATTER.md §14.2).
type EditKind uint8

// Edit is one edit, its node at Pointer in the text the edits before it left: Set writes Value
// there, Insert adds Value as element or member Key at position At of the container there,
// Remove deletes the member or element there. Value is printed as Format prints.
type Edit struct {
	Kind    EditKind
	Pointer string
	Key     string
	At      int
	Value   *Node
}

// change is one change of a text: the bytes lo to hi become text.
type change struct {
	lo, hi int
	text   []byte
}

var editTable [editKindCount]func([]byte, *Node, Edit) (change, error)

func init() {
	editTable = [editKindCount]func([]byte, *Node, Edit) (change, error){
		Set: setEdit, Insert: insertEdit, Remove: removeEdit,
	}
}

// Rewrite applies edits in order to src, keeping every byte outside the members and elements
// they change. ErrEdit for an edit that does not apply; Parse's errors for a src that does
// not parse.
func Rewrite(src []byte, edits []Edit) ([]byte, error) {
	// FORMATTER.md §14.2
	content, root, err := reread(src)
	if err != nil {
		return nil, err
	}
	for _, e := range edits {
		if e.Kind >= editKindCount {
			return nil, fmt.Errorf("%w: kind %d", ErrEdit, e.Kind)
		}
		c, err := editTable[e.Kind](content, root, e)
		if err != nil {
			return nil, err
		}
		if content, root, err = reread(slices.Concat(content[:c.lo], c.text, content[c.hi:])); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrEdit, err)
		}
	}
	return content, nil
}

// reread parses src, its line ends normalized, as the source a pointer names nodes in.
func reread(src []byte) ([]byte, *Node, error) {
	var fs source.FileSet
	f, err := fs.Add(editPath, editPath, src)
	if err != nil {
		return nil, nil, fmt.Errorf("jsonsrc: %w", err)
	}
	root, err := Parse(f, diag.NewBag(&fs, ""))
	return f.Content, root, err
}

// target is the node at e.Pointer, of one of kinds when any is given.
func target(root *Node, e Edit, kinds ...Kind) (*Node, error) {
	n := root.Find(e.Pointer)
	if n == nil || len(kinds) > 0 && !slices.Contains(kinds, n.Kind) {
		return nil, fmt.Errorf("%w: nothing to edit at %q", ErrEdit, e.Pointer)
	}
	return n, nil
}

// setEdit writes Value over the value at Pointer.
func setEdit(content []byte, root *Node, e Edit) (change, error) {
	n, err := target(root, e)
	if err != nil || !printable(e.Value, map[*Node]bool{}) {
		return change{}, fmt.Errorf("%w: no value to set at %q", ErrEdit, e.Pointer)
	}
	return unit(content, n, e.Value), nil
}

// printable reports a value Format can print: every node of a known kind, no child nil, none
// its own ancestor, at most maxDepth deep (log-2026-09-29 M4 U1r).
func printable(v *Node, path map[*Node]bool) bool {
	if v == nil || v.Kind >= kindCount || path[v] || len(path) >= maxDepth {
		return false
	}
	path[v] = true
	defer delete(path, v)
	for _, e := range v.Elems {
		if !printable(e, path) {
			return false
		}
	}
	for _, m := range v.Members {
		if !printable(m.Value, path) {
			return false
		}
	}
	return true
}

// unit prints repl over n in canonical layout, or over n's container, with n replaced, when that
// container is written on one line, and so on up.
func unit(content []byte, n, repl *Node) change {
	for p := n.at.parent; p != nil && oneLine(content, p); p = p.at.parent {
		repl, n = replaced(p, n, repl), p
	}
	return change{int(n.Span.Start), int(n.Span.End), AppendPretty(nil, repl, n.depth())}
}

// oneLine reports a node written on one line.
func oneLine(content []byte, n *Node) bool {
	return !bytes.Contains(content[n.Span.Start:n.Span.End], []byte{newline})
}

// replaced is a copy of container p with its child old replaced by repl, or removed when repl
// is nil.
func replaced(p, old, repl *Node) *Node {
	cp := *p
	cp.Elems, cp.Members = nil, nil
	for _, e := range p.Elems {
		if e != old {
			cp.Elems = append(cp.Elems, e)
		} else if repl != nil {
			cp.Elems = append(cp.Elems, repl)
		}
	}
	for _, m := range p.Members {
		if m.Value != old {
			cp.Members = append(cp.Members, m)
		} else if repl != nil {
			cp.Members = append(cp.Members, Member{Key: m.Key, Value: repl})
		}
	}
	return &cp
}

// insertEdit adds Value to the array or object at Pointer: on a line of its own after the item
// before it, with the "," that line needs, in a container laid out one item per line.
func insertEdit(content []byte, root *Node, e Edit) (change, error) {
	c, err := target(root, e, Array, Object)
	switch {
	case err != nil:
		return change{}, err
	case !printable(e.Value, map[*Node]bool{}) || e.At < 0 || e.At > len(c.itemsOf()):
		return change{}, fmt.Errorf("%w: no value or position to insert at %q", ErrEdit, e.Pointer)
	case c.Kind == Object && c.member(e.Key) >= 0:
		return change{}, fmt.Errorf("%w: key %q exists at %q", ErrEdit, e.Key, e.Pointer)
	}
	if len(c.itemsOf()) == 0 || oneLine(content, c) {
		return unit(content, c, c.grown(e)), nil
	}
	var w printer
	w.indent(c.depth() + 1)
	if c.Kind == Object {
		w.buf = append(diag.AppendJSONString(w.buf, e.Key), keySep...)
	}
	w.value(e.Value, c.depth()+1)
	if e.At == 0 {
		at := int(c.Span.Start) + 1
		return change{at, at, append(w.buf, comma)}, nil
	}
	end := int(c.itemsOf()[e.At-1].Span.End)
	return change{end, end, slices.Concat([]byte{comma}, w.buf)}, nil
}

// grown is a copy of container n with the value of an Insert added.
func (n *Node) grown(e Edit) *Node {
	cp := *n
	if n.Kind == Array {
		cp.Elems = slices.Insert(slices.Clone(n.Elems), e.At, e.Value)
	} else {
		cp.Members = slices.Insert(slices.Clone(n.Members), e.At, Member{Key: e.Key, Value: e.Value})
	}
	return &cp
}

// itemsOf are the values of an array's elements or of an object's members.
func (n *Node) itemsOf() []*Node {
	if n.Kind == Array {
		return n.Elems
	}
	out := make([]*Node, len(n.Members))
	for i, m := range n.Members {
		out[i] = m.Value
	}
	return out
}

// removeEdit deletes the member or element at Pointer with the "," that separates it.
func removeEdit(content []byte, root *Node, e Edit) (change, error) {
	n, err := target(root, e)
	if err != nil || n.at.parent == nil {
		return change{}, fmt.Errorf("%w: nothing to remove at %q", ErrEdit, e.Pointer)
	}
	p := n.at.parent
	items := p.itemsOf()
	if len(items) == 1 || oneLine(content, p) {
		return unit(content, p, replaced(p, n, nil)), nil
	}
	i := slices.Index(items, n)
	if i > 0 {
		return change{lo: int(items[i-1].Span.End), hi: int(n.Span.End)}, nil
	}
	return change{lo: p.itemStart(0), hi: p.itemStart(1)}, nil
}

// itemStart is the offset of item i of a container: its key for a member.
func (n *Node) itemStart(i int) int {
	if n.Kind == Object {
		return int(n.Members[i].KeySpan.Start)
	}
	return int(n.Elems[i].Span.Start)
}
