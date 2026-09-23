package syntax

import "iter"

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
	Walk(inspector(f), n)
}

// Children yields the direct children of n in source order.
func Children(n Node) iter.Seq[Node] {
	return func(yield func(Node) bool) {
		n.children(yield)
	}
}

type inspector func(Node) bool

func (f inspector) Visit(n Node) Visitor {
	if f(n) {
		return f
	}
	return nil
}

// visit yields each non-nil node of ns and reports whether the walk goes on.
func visit[N interface {
	Node
	comparable
}](yield func(Node) bool, ns ...N,
) bool {
	var none N
	for _, n := range ns {
		if n != none && !yield(n) {
			return false
		}
	}
	return true
}
