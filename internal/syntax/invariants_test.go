package syntax_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	dirPerm    = 0o755
	filePerm   = 0o644
	opTokField = "OpTok"
)

// checkTree checks what the File contract promises of a tree: every node names tokens of the
// file, lies within its parent, after its previous sibling (a doc block shares its host's first
// token), and a file parsed without findings holds no Bad or empty node.
func checkTree(t testing.TB, name string, f *syntax.File, clean bool) {
	t.Helper()
	var stack []syntax.Node
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if msg := badNode(f, n, stack, clean); msg != "" {
			t.Errorf("%s: %s %s at %+v: %s", name, n.Kind(), msg, f.Span(n), stack[len(stack)-1].Kind())
		}
		stack = append(stack, n)
		return true
	})
	checkSiblings(t, name, f)
}

func badNode(f *syntax.File, n syntax.Node, stack []syntax.Node, clean bool) string {
	if _, ok := n.(*syntax.File); ok {
		return ""
	}
	first, last := n.First(), n.Last()
	switch {
	case !first.Valid() || int(last) >= len(f.Tokens) || last < first-1:
		return "has bounds outside the tokens"
	case last < first && clean:
		return "is empty in a file without findings"
	case isBad(n) && clean:
		return "is a Bad node in a file without findings"
	}
	parent := stack[len(stack)-1]
	if _, isFile := parent.(*syntax.File); !isFile && (first < parent.First() || last > parent.Last()) {
		return "lies outside its parent"
	}
	return ""
}

func isBad(n syntax.Node) bool {
	switch n.(type) {
	case *syntax.BadExpr, *syntax.BadType, *syntax.BadStmt, *syntax.BadDecl:
		return true
	}
	return false
}

// checkSiblings checks that the children of every node come in source order.
func checkSiblings(t testing.TB, name string, f *syntax.File) {
	t.Helper()
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n == nil {
			return false
		}
		var prev syntax.Node
		for c := range syntax.Children(n) {
			if prev != nil && !ordered(prev, c) {
				t.Errorf("%s: %s: child %s before %s", name, n.Kind(), c.Kind(), prev.Kind())
			}
			prev = c
		}
		return true
	})
}

func ordered(prev, c syntax.Node) bool {
	if _, doc := prev.(*syntax.DocComment); doc || isBad(prev) || isBad(c) {
		return c.First() >= prev.Last()
	}
	return c.First() > prev.Last()
}
