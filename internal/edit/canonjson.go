package edit

import (
	"bytes"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
)

// typedNumbers sets in root, parsed from content (the JSON source at display), each number token a
// load of the snapshot read from those bytes to its canonical text, as `canon fmt --json-sources`
// does (API.md M9; FORMATTER.md 14.1; DECISIONS 165).
func (s *Snapshot) typedNumbers(display string, content []byte, root *jsonsrc.Node) {
	read, texts := s.a.NumberTexts(display)
	if len(texts) == 0 || !bytes.Equal(read, content) {
		return
	}
	setNumbers(root, texts)
}

// setNumbers sets each number token of root at a span of texts to that text.
func setNumbers(root *jsonsrc.Node, texts map[source.Span]string) {
	stack := []*jsonsrc.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = append(stack[:len(stack)-1], n.Elems...)
		for _, m := range n.Members {
			stack = append(stack, m.Value)
		}
		if text, ok := texts[source.Span{Start: n.Span.Start, End: n.Span.End}]; ok && n.Kind == jsonsrc.Number {
			n.Text = text
		}
	}
}
