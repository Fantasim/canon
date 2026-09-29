package jsonsrc

import (
	"slices"
	"strconv"
	"strings"
)

var pointerUnescaper = strings.NewReplacer(escapedSlash, pointerSep, escapedTilde, tilde)

// Find is the node at the RFC 6901 pointer ptr from n, "" being n itself; nil when there is none.
func (n *Node) Find(ptr string) *Node {
	if ptr == "" {
		return n
	}
	rest, ok := strings.CutPrefix(ptr, pointerSep)
	if !ok {
		return nil
	}
	token, rest, more := strings.Cut(rest, pointerSep)
	child := n.child(pointerUnescaper.Replace(token))
	switch {
	case child == nil:
		return nil
	case more:
		return child.Find(pointerSep + rest)
	default:
		return child
	}
}

// child is an object's member value with key token, or an array's element at the decimal index
// token, without sign or leading zero.
func (n *Node) child(token string) *Node {
	if n.Kind == Object {
		if i := n.member(token); i >= 0 {
			return n.Members[i].Value
		}
		return nil
	}
	i, err := strconv.Atoi(token)
	if n.Kind != Array || err != nil || i < 0 || i >= len(n.Elems) || strconv.Itoa(i) != token {
		return nil
	}
	return n.Elems[i]
}

// depth is the number of containers holding n.
func (n *Node) depth() int {
	d := 0
	for c := n.at.parent; c != nil; c = c.at.parent {
		d++
	}
	return d
}

// Place is the position of a new member key in obj, declared being the wire names of obj's
// type in declaration order (a @json(path:) prefix where its first field stands): after the
// nearest earlier one present, else before the first later one present, else last.
func Place(obj *Node, declared []string, key string) int {
	// FORMATTER.md §14.2, LOD-09
	at := slices.Index(declared, key)
	if at < 0 {
		return len(obj.Members)
	}
	for i := at - 1; i >= 0; i-- {
		if m := obj.member(declared[i]); m >= 0 {
			return m + 1
		}
	}
	for _, d := range declared[at+1:] {
		if m := obj.member(d); m >= 0 {
			return m
		}
	}
	return len(obj.Members)
}

// member is the index of the member with key, -1 when there is none.
func (n *Node) member(key string) int {
	for i, m := range n.Members {
		if m.Key == key {
			return i
		}
	}
	return -1
}
