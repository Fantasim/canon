package syntax

import (
	"iter"
	"reflect"
)

// Visitor is called for each node Walk meets; the visitor it returns walks the node's children.
type Visitor interface {
	Visit(n Node) (w Visitor)
}

// Walk traverses the tree in source order: it calls v.Visit(n), walks n's children with the
// visitor it returns when that is not nil, then calls that visitor's Visit(nil).
func Walk(v Visitor, n Node) {
	if v = v.Visit(n); v == nil {
		return
	}
	n.children(func(c Node) bool {
		Walk(v, c)
		return true
	})
	v.Visit(nil)
}

// Inspect traverses the tree in source order, calling f for each node, and for nil after a
// node's children; the children of a node for which f returns false are skipped.
func Inspect(n Node, f func(Node) bool) {
	var yield func(Node) bool
	yield = func(c Node) bool {
		if f(c) {
			c.children(yield)
			f(nil)
		}
		return true
	}
	yield(n)
}

// Children yields the direct children of n in source order.
func Children(n Node) iter.Seq[Node] {
	return func(yield func(Node) bool) {
		n.children(yield)
	}
}

// visit yields each non-nil node of ns, a typed nil in an interface field included, and
// reports whether the walk goes on.
func visit[N Node](yield func(Node) bool, ns ...N) bool {
	for _, n := range ns {
		if !isNil(n) && !yield(n) {
			return false
		}
	}
	return true
}

// isNil reports a nil node: a nil interface, or a nil pointer held by one.
func isNil(n Node) bool {
	if n == nil {
		return true
	}
	v := reflect.ValueOf(n)
	return v.Kind() == reflect.Pointer && v.IsNil()
}
