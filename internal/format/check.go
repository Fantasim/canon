package format

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// usable is ErrSyntax for a tree with a syntax error, the lexer's included, and ErrLayout for an
// indentation tab or a lone carriage return; source.FileSet folds CRLF before format sees it,
// so the raw-bytes check of API.md M9 is the edit layer's. kind is the role (DECISIONS 258).
func usable(f *syntax.File, kind syntax.FileKind) error {
	if !sound(f) {
		return ErrSyntax
	}
	if bytes.Contains(f.Src.Content, []byte(carriageReturn)) || tabbed(f) {
		return ErrLayout
	}
	if _, err := reparse(f, kind, f.Src.Content); err != nil {
		return ErrSyntax // an error only the lexer reports leaves a whole tree (DECISIONS 166)
	}
	return nil
}

// tabbed reports a token or a comment that a tab indents.
func tabbed(f *syntax.File) bool {
	src := f.Src.Content
	indented := func(at source.Pos) bool {
		i := int(at)
		for i > 0 && strings.IndexByte(trailingBlanks, src[i-1]) >= 0 {
			i--
		}
		return (i == 0 || src[i-1] == newlineText[0]) && bytes.Contains(src[i:at], []byte(tab))
	}
	return slices.ContainsFunc(f.Tokens, func(t syntax.Token) bool {
		return indented(t.Start) || slices.ContainsFunc(t.Leading, func(tr syntax.Trivia) bool {
			return isComment(tr) && indented(tr.Start)
		})
	})
}

// reparse parses content as f's file in the role of kind, not the tree's own kind: a source
// opening with `project` holds E1011 (DECISIONS 258); ErrText when it holds a syntax error.
func reparse(f *syntax.File, kind syntax.FileKind, content []byte) (*syntax.File, error) {
	var fs source.FileSet
	src, err := fs.Add(f.Src.Path, f.Src.Abs, content)
	if err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}
	bag := diag.NewBag(&fs, "")
	g := syntax.Parse(src, kind, bag)
	if failed(bag, src) || !sound(g) {
		return nil, ErrText
	}
	return g, nil
}

// stands reports that the new text of m parses as exactly one node like the one it replaces,
// or as one item of its list (log-2026-09-29 M4 U1r).
func (b *builder) stands(m mark) bool {
	found := false
	syntax.Inspect(b.f, func(n syntax.Node) bool {
		if n == nil || found || n.Kind() == syntax.KindDocComment || n.Kind() == syntax.KindFile {
			return !found
		}
		lo, hi := b.span(n)
		if lo == m.lo && hi == m.hi {
			found = b.meets(n, m.want)
		}
		return lo <= m.lo && m.hi <= hi
	})
	return found
}

// meets reports a node that is what want asks for.
func (b *builder) meets(n syntax.Node, want expect) bool {
	owner, isItem := b.idx.owner[n]
	switch {
	case want.item && (!isItem || want.top != (owner < 0)):
		return false
	case want.like == nil:
		return true
	}
	for _, is := range categories {
		if is(want.like) {
			return is(n)
		}
	}
	return want.like.Kind() == n.Kind()
}

// holds reports a node whose tokens are tokens of f.
func holds(f *syntax.File, n syntax.Node) bool {
	return !absent(n) && n.First() > syntax.NoTok && n.First() <= n.Last() && int(n.Last()) < len(f.Tokens)
}

// absent reports a nil node, typed or not; syntax keeps its own isNil unexported.
func absent(n syntax.Node) bool {
	v := reflect.ValueOf(n)
	return n == nil || v.Kind() == reflect.Pointer && v.IsNil()
}

// trimmed is new text without the blanks around it.
func trimmed(t string) string { return strings.TrimSpace(t) }
