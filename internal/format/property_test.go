package format_test

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

// sampleEvery keeps the property tests fast: one position in so many is tried, the offset
// turning with the file so that every offset is tried.
const sampleEvery = 13

// edit inserts text at a byte offset.
type edit struct {
	at   int
	text string
}

func apply(src []byte, edits []edit) []byte {
	slices.SortStableFunc(edits, func(x, y edit) int { return x.at - y.at })
	var out bytes.Buffer
	last := 0
	for _, e := range edits {
		out.Write(src[last:e.at])
		out.WriteString(e.text)
		last = e.at
	}
	out.Write(src[last:])
	return out.Bytes()
}

// inString reports, for each token, whether it lies inside a string's interpolation.
func inString(toks []syntax.Token) []bool {
	out := make([]bool, len(toks))
	depth := 0
	for i, t := range toks {
		switch t.Kind {
		case syntax.TokStringHead, syntax.TokMLStringHead:
			depth++
		case syntax.TokStringTail, syntax.TokMLStringTail:
			depth--
		default:
			out[i] = depth > 0
		}
	}
	return out
}

// injections are the comments a test may add to src without changing its meaning: an own-line
// comment before a line's first token, a trailing comment at a line's end, a block comment
// between two tokens of a line.
func injections(f *syntax.File) []edit {
	src, toks, inner := f.Src.Content, f.Tokens, inString(f.Tokens)
	var out []edit
	for i := 1; i+1 < len(toks); i++ {
		t, next := toks[i], toks[i+1]
		if t.Kind == syntax.TokNL || inner[i] {
			continue
		}
		gap := src[t.End:next.Start]
		if next.Kind == syntax.TokNL {
			gap = src[t.End:toks[i+2].Start]
		}
		switch {
		case bytes.HasPrefix(bytes.TrimLeft(gap, " \t"), []byte("\n")):
			out = append(out, edit{int(t.End), fmt.Sprintf(" // trailing %d", i)})
		case !bytes.Contains(gap, []byte("\n")) && blockable(toks, i):
			out = append(out, edit{int(t.End), fmt.Sprintf(" /* b%d */ ", i)})
		}
		if t.Start > 0 && bytes.LastIndexByte(src[:t.Start], '\n') == len(bytes.TrimRight(src[:t.Start], " \t"))-1 && t.Kind != syntax.TokEOF {
			out = append(out, edit{int(t.Start), fmt.Sprintf("// own %d\n", i)})
		}
	}
	return out
}

// blockable reports whether a block comment may stand after token i: not inside an annotation
// or before an amend position, where tokens must touch.
func blockable(toks []syntax.Token, i int) bool {
	k, next := toks[i].Kind, toks[i+1].Kind
	annName := i > 0 && toks[i-1].Kind == syntax.TokAt && next == syntax.TokLParen
	return k != syntax.TokAt && next != syntax.TokHash && !annName
}

// FORMATTER.md §8: a comment added anywhere it can stand keeps every guarantee.
func TestInjectedComments(t *testing.T) {
	for k, ex := range exampleFiles(t) {
		base := parse(t, ex.path, ex.data)
		for i, e := range injections(base.file) {
			if (i+k)%sampleEvery != 0 {
				continue
			}
			in := parse(t, ex.path, apply(ex.data, []edit{e}))
			if slices.Equal(codes(in.bag), codes(base.bag)) {
				checkFormatted(t, ex.path, in, mustFile(t, in.file))
			}
		}
	}
}

// relaid is src with more spaces between tokens, and line breaks after every "(", "[" and ","
// inside brackets but outside braces, where they neither matter nor change the single-line
// bit of a brace list.
func relaid(f *syntax.File) []byte {
	src, toks, inner := f.Src.Content, f.Tokens, inString(f.Tokens)
	var edits []edit
	var stack []syntax.TokenKind
	for i, t := range toks {
		if inner[i] {
			continue
		}
		for _, tr := range t.Trailing {
			if tr.Kind == syntax.TriviaSpace {
				edits = append(edits, edit{int(tr.Start), "  \t"})
			}
		}
		stack = track(stack, t.Kind)
		loose := len(stack) > 0 && !slices.Contains(stack, syntax.TokLBrace)
		opens := t.Kind == syntax.TokLParen || t.Kind == syntax.TokLBrack || t.Kind == syntax.TokComma
		if loose && opens && !hasComment(t.Trailing) {
			edits = append(edits, edit{int(t.End), "\n\t "})
		}
	}
	return apply(src, edits)
}

func hasComment(tr []syntax.Trivia) bool {
	return slices.ContainsFunc(tr, func(x syntax.Trivia) bool { return x.Kind != syntax.TriviaSpace && x.Kind != syntax.TriviaNewline })
}

func track(stack []syntax.TokenKind, k syntax.TokenKind) []syntax.TokenKind {
	switch k {
	case syntax.TokLParen, syntax.TokLBrack, syntax.TokLBrace:
		return append(stack, k)
	case syntax.TokRParen, syntax.TokRBrack, syntax.TokRBrace:
		if len(stack) > 0 {
			return stack[:len(stack)-1]
		}
	default:
	}
	return stack
}

// FORMATTER.md §12: spaces, line breaks and indentation where they do not matter change nothing.
func TestRelaidExamples(t *testing.T) {
	for _, ex := range exampleFiles(t) {
		base := parse(t, ex.path, ex.data)
		src := relaid(base.file)
		in := parse(t, ex.path, src)
		if !slices.Equal(codes(in.bag), codes(base.bag)) {
			t.Fatalf("%s: relaying changed the findings: %v\n%s", ex.path, codes(in.bag), src)
		}
		if got := mustFile(t, in.file); !bytes.Equal(got, ex.data) {
			t.Errorf("%s: relaid, it formats differently:\n%s", ex.path, lineDiff(ex.data, got))
		}
		reindented := reindent(ex.data, base.file)
		if got := mustFile(t, parse(t, ex.path, reindented).file); !bytes.Equal(got, ex.data) {
			t.Errorf("%s: reindented, it formats differently:\n%s", ex.path, lineDiff(ex.data, got))
		}
	}
}

// reindent replaces the indentation of every line that starts with a token by a tab.
func reindent(data []byte, f *syntax.File) []byte {
	starts := map[int]bool{}
	for _, t := range f.Tokens {
		starts[int(t.Start)] = true
	}
	var out strings.Builder
	at := 0
	for _, l := range strings.SplitAfter(string(data), "\n") {
		trimmed := strings.TrimLeft(l, " \t")
		first := at + len(l) - len(trimmed)
		at += len(l)
		if trimmed != "" && trimmed != "\n" && starts[first] && !strings.HasPrefix(trimmed, `"""`) {
			l = "\t" + trimmed
		}
		out.WriteString(l)
	}
	return []byte(out.String())
}
