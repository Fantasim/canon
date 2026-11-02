package lint

import (
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// markerWords reports W0005 (API.md L6): a string literal outside the tests and the broken declarations that holds a marker's words.
func markerWords(p *pass, pkg *check.Package, bag *diag.Bag) {
	for _, f := range sources(pkg) {
		syntax.Inspect(f, func(n syntax.Node) bool {
			if _, isTest := n.(*syntax.TestDecl); isTest || p.broken.nodes[n] {
				return false
			}
			if holdsMarker(n) {
				diag.W0005.At(f.Span(n)).Report(bag)
			}
			return true
		})
	}
}

// holdsMarker reports a string literal, plain or raw, with a marker's words in a text part.
func holdsMarker(n syntax.Node) bool {
	switch n := n.(type) {
	case *syntax.StringLit:
		for _, part := range n.Parts {
			if hasMarker(part.Text) {
				return true
			}
		}
	case *syntax.RawStringLit:
		return hasMarker(n.Value)
	}
	return false
}

// hasMarker reports text holding the words of a marker.
func hasMarker(text string) bool {
	for _, m := range markers {
		if strings.Contains(text, m) {
			return true
		}
	}
	return false
}
