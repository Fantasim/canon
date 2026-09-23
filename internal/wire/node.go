package wire

import (
	"math/big"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
)

// node is an encoded JSON value before layout: scalar bytes, an array or an ordered object.
type node struct {
	raw     []byte
	array   bool
	elems   []*node
	members []member
}

type member struct {
	key string
	val *node
}

func scalar(b []byte) *node { return &node{raw: b} }

func text(s string) *node { return scalar([]byte(s)) }

func stringNode(s string) *node { return scalar(diag.AppendJSONString(nil, s)) }

func arrayNode(n int) *node { return &node{array: true, elems: make([]*node, 0, n)} }

func objectNode() *node { return &node{members: []member{}} }

func (n *node) isObject() bool { return n.members != nil }

func (n *node) add(key string, val *node) { n.members = append(n.members, member{key, val}) }

// put adds val at a key path, creating each intermediate object at its first use (§5.5.3).
func (n *node) put(path []string, val *node) error {
	obj := n
	for _, key := range path[:len(path)-1] {
		next := obj.get(key)
		if next == nil {
			next = objectNode()
			obj.add(key, next)
		}
		if !next.isObject() {
			return ErrShape
		}
		obj = next
	}
	obj.add(path[len(path)-1], val)
	return nil
}

func (n *node) get(key string) *node {
	for _, m := range n.members {
		if m.key == key {
			return m.val
		}
	}
	return nil
}

// compact appends compact(v) of WIRE.md §7.4.
func (n *node) compact(b []byte) []byte {
	switch {
	case n.array:
		b = append(b, openArray...)
		for i, e := range n.elems {
			if i > 0 {
				b = append(b, sepCompact...)
			}
			b = e.compact(b)
		}
		return append(b, closeArray...)
	case n.isObject():
		b = append(b, openObject...)
		for i, m := range n.members {
			if i > 0 {
				b = append(b, sepCompact...)
			}
			b = append(diag.AppendJSONString(b, m.key), sepKey...)
			b = m.val.compact(b)
		}
		return append(b, closeObject...)
	}
	return append(b, n.raw...)
}

// pretty appends pretty(v, depth × 2) of WIRE.md §7.4, the layout of JSON.stringify(v, null, 2).
func (n *node) pretty(b []byte, depth int) []byte {
	if len(n.elems) == 0 && len(n.members) == 0 {
		return n.compact(b)
	}
	inner := newline + strings.Repeat(indentSpaces, depth+1)
	if n.array {
		b = append(b, openArray...)
		for i, e := range n.elems {
			b = append(b, separator(i, inner)...)
			b = e.pretty(b, depth+1)
		}
		return append(append(b, newline+strings.Repeat(indentSpaces, depth)...), closeArray...)
	}
	b = append(b, openObject...)
	for i, m := range n.members {
		b = append(b, separator(i, inner)...)
		b = append(diag.AppendJSONString(b, m.key), sepKey...)
		b = m.val.pretty(b, depth+1)
	}
	return append(append(b, newline+strings.Repeat(indentSpaces, depth)...), closeObject...)
}

// separator is what precedes the i-th element of a pretty container: its line break and indent.
func separator(i int, inner string) string {
	if i == 0 {
		return inner
	}
	return comma + inner
}

// sameJSON is the JSON equality of a none marker (WIRE.md §5.4).
func sameJSON(n *node, marker []byte) bool {
	m := string(marker)
	switch {
	case m == openObject+closeObject:
		return n.isObject() && len(n.members) == 0
	case m == openArray+closeArray:
		return n.array && len(n.elems) == 0
	case n.array || n.isObject():
		return false
	case strings.HasPrefix(m, quote):
		return sameString(string(n.raw), m)
	}
	return string(n.raw) == m || sameNumber(string(n.raw), m)
}

func sameString(a, b string) bool {
	x, _, errA := diag.UnquoteJSON(a)
	y, _, errB := diag.UnquoteJSON(b)
	return errA == nil && errB == nil && x == y
}

func sameNumber(a, b string) bool {
	x, okA := new(big.Rat).SetString(a)
	y, okB := new(big.Rat).SetString(b)
	return okA && okB && x.Cmp(y) == 0
}
