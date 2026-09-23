package jsonsrc

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/source"
)

// Kind is the JSON kind of a node.
type Kind uint8

// Node is one value of a JSON source: its bytes, quotes and brackets included, and as Text a
// string's decoded value, or a number, true, false or null as written.
type Node struct {
	Kind    Kind
	Span    source.Span
	Text    string
	Elems   []*Node
	Members []Member // in document order
	at      link
}

// Member is one "key": value pair of an object; KeySpan is the key's string token.
type Member struct {
	Key     string
	KeySpan source.Span
	Value   *Node
}

// link places a node in its container: a member by its key, an element by its index.
type link struct {
	parent *Node
	key    string
	index  int
}

var pointerEscaper = strings.NewReplacer(tilde, escapedTilde, pointerSep, escapedSlash)

// Pointer is the node's RFC 6901 pointer, "" for the document, built from its containers.
func (n *Node) Pointer() string {
	var tokens []string
	for c := n; c != nil && c.at.parent != nil; c = c.at.parent {
		tokens = append(tokens, c.at.token())
	}
	var b strings.Builder
	for _, t := range slices.Backward(tokens) {
		b.WriteString(pointerSep)
		b.WriteString(t)
	}
	return b.String()
}

// token is the reference token of the linked node (RFC 6901 §3).
func (l link) token() string {
	if l.parent.Kind == Array {
		return strconv.Itoa(l.index)
	}
	if strings.ContainsAny(l.key, pointerSpecial) {
		return pointerEscaper.Replace(l.key)
	}
	return l.key
}
