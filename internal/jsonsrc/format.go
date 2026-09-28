package jsonsrc

import "github.com/fantasim/canonlang/internal/diag"

// printTable prints a node by kind.
var printTable [kindCount]func(*printer, *Node, int)

func init() {
	printTable = [kindCount]func(*printer, *Node, int){
		Null: (*printer).token, Bool: (*printer).token, Number: (*printer).token,
		String: (*printer).str, Array: (*printer).array, Object: (*printer).object,
	}
}

// Format prints a tree in the canonical layout of JSON sources, numbers as their Text: a caller
// that knows a number's Canon type sets its Text to the type's canonical text (DECISIONS 165).
func Format(n *Node) []byte {
	// FORMATTER.md §14.1
	return append(AppendPretty(nil, n, 0), newline)
}

// AppendPretty appends n's pretty layout (WIRE.md §7.4) to b at indentation depth, Null/Bool/Number's Text written verbatim, for a caller embedding n inside a larger hand-assembled document (WIRE.md §8.2) rather than printing it whole with Format.
func AppendPretty(b []byte, n *Node, depth int) []byte {
	w := printer{buf: b}
	w.value(n, depth)
	return w.buf
}

type printer struct {
	buf []byte
}

func (w *printer) value(n *Node, depth int) {
	printTable[n.Kind](w, n, depth)
}

func (w *printer) token(n *Node, _ int) {
	w.buf = append(w.buf, n.Text...)
}

// str writes a string with the escaping of WIRE.md §7.3 (FORMATTER.md §14.1).
func (w *printer) str(n *Node, _ int) {
	w.buf = diag.AppendJSONString(w.buf, n.Text)
}

func (w *printer) array(n *Node, depth int) {
	if len(n.Elems) == 0 {
		w.buf = append(w.buf, emptyArray...)
		return
	}
	w.buf = append(w.buf, openArray)
	for i, e := range n.Elems {
		w.line(i, depth+1)
		w.value(e, depth+1)
	}
	w.end(closeArray, depth)
}

func (w *printer) object(n *Node, depth int) {
	if len(n.Members) == 0 {
		w.buf = append(w.buf, emptyObject...)
		return
	}
	w.buf = append(w.buf, openObject)
	for i, m := range n.Members {
		w.line(i, depth+1)
		w.buf = append(diag.AppendJSONString(w.buf, m.Key), keySep...)
		w.value(m.Value, depth+1)
	}
	w.end(closeObject, depth)
}

// line starts the line of item i of a container, its comma ending the line before.
func (w *printer) line(i, depth int) {
	if i > 0 {
		w.buf = append(w.buf, comma)
	}
	w.indent(depth)
}

// end writes a container's closing byte on its own line.
func (w *printer) end(c byte, depth int) {
	w.indent(depth)
	w.buf = append(w.buf, c)
}

func (w *printer) indent(depth int) {
	w.buf = append(w.buf, newline)
	for range depth {
		w.buf = append(w.buf, indentUnit...)
	}
}
