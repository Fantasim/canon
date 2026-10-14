package syntax

import (
	"bytes"

	"github.com/fantasim/canonlang/internal/source"
)

// AppendLeading appends token t's leading trivia to dst, read back from the bytes before t: from
// the first line break after the previous token, all of them after BOF; none for BOF or an NL.
func (f *File) AppendLeading(dst []Trivia, t Tok) []Trivia {
	return appendLeading(dst, f.Src.Content, f.Tokens, t)
}

// AppendTrailing appends token t's trailing trivia to dst, read back from the bytes after t up
// to the first line break; none for BOF, EOF or an NL token.
func (f *File) AppendTrailing(dst []Trivia, t Tok) []Trivia {
	return appendTrailing(dst, f.Src.Content, f.Tokens, t)
}

// appendLeading is AppendLeading on tokens being parsed; the split is the lexer's (DECISIONS 73).
func appendLeading(dst []Trivia, src []byte, toks []Token, t Tok) []Trivia {
	if t <= NoTok || int(t) >= len(toks) {
		return dst
	}
	prev := toks[t-1]
	s := gapScan(src, prev, toks[t])
	lead := prev.Kind == TokBOF
	for tr, ok := s.next(); ok; tr, ok = s.next() {
		lead = lead || tr.Kind == TriviaNewline
		if lead {
			dst = append(dst, tr)
		}
	}
	return dst
}

// appendTrailing is AppendTrailing on tokens being parsed: the gap after t up to its line break.
func appendTrailing(dst []Trivia, src []byte, toks []Token, t Tok) []Trivia {
	if t <= NoTok || int(t)+1 >= len(toks) {
		return dst
	}
	s := gapScan(src, toks[t], toks[t+1])
	for tr, ok := s.next(); ok && tr.Kind != TriviaNewline; tr, ok = s.next() {
		dst = append(dst, tr)
	}
	return dst
}

// trailEnd is where p's trailing trivia ends before next: at the first line break between
// them, else at next.
func trailEnd(src []byte, p, next Token) source.Pos {
	s := gapScan(src, p, next)
	for tr, ok := s.next(); ok; tr, ok = s.next() {
		if tr.Kind == TriviaNewline {
			return tr.Start
		}
	}
	return next.Start
}

// triviaScan reads the trivia of src[pos:end], the bytes between two tokens, one at a time;
// lineText is set while a token or a block comment precedes on the line: "///" is no doc there.
type triviaScan struct {
	src      []byte
	pos, end int
	lineText bool
}

// gapScan reads the trivia between prev and next; every token but BOF is text on its line.
func gapScan(src []byte, prev, next Token) triviaScan {
	return triviaScan{src: src, pos: int(prev.End), end: int(next.Start), lineText: prev.Kind != TokBOF}
}

func (s *triviaScan) next() (Trivia, bool) {
	if s.pos >= s.end {
		return Trivia{}, false
	}
	start := s.pos
	k := s.read()
	return Trivia{Kind: k, Start: pos(start), End: pos(s.pos)}, true
}

// read moves past the trivia at pos and returns its kind, as the lexer reads it.
func (s *triviaScan) read() TriviaKind {
	start := s.pos
	switch c := s.src[start]; {
	case start == 0 && hasPrefixAt(s.src, 0, bomText):
		s.pos = len(bomText)
		return TriviaBOM
	case c == '\n':
		s.pos++
		s.lineText = false
		return TriviaNewline
	case c == ' ' || c == '\t':
		s.pos = spaceEnd(s.src, start)
		return TriviaSpace
	case hasPrefixAt(s.src, start, lineCommentText):
		s.pos = lineCommentEnd(s.src, start)
		if isDocLine(s.src, start) && !s.lineText {
			return TriviaDocComment
		}
		return TriviaLineComment
	case hasPrefixAt(s.src, start, blockOpen):
		s.pos, _ = blockCommentEnd(s.src, start)
		s.lineText = true
		return TriviaBlockComment
	default:
	}
	s.pos++ // a "\r" the "\r\n" normalization left, whitespace on its own (E1124)
	return TriviaSpace
}

// spaceEnd is the end of the run of spaces and tabs at i.
func spaceEnd(src []byte, i int) int {
	for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
		i++
	}
	return i
}

// lineCommentEnd is the end of the line comment at start: its line break, or the file's end.
func lineCommentEnd(src []byte, start int) int {
	end := bytes.IndexByte(src[start:], '\n')
	if end < 0 {
		return len(src)
	}
	return start + end
}

// blockCommentEnd is the end of the block comment at start, past its "*/"; the file's end and
// false when it is unterminated.
func blockCommentEnd(src []byte, start int) (int, bool) {
	end := bytes.Index(src[start+len(blockOpen):], []byte(blockClose))
	if end < 0 {
		return len(src), false
	}
	return start + len(blockOpen) + end + len(blockClose), true
}

// isDocLine reports a "///" comment at start that is not "////" (GRAMMAR.md §2.2).
func isDocLine(src []byte, start int) bool {
	return hasPrefixAt(src, start, docPrefix) && !hasPrefixAt(src, start, ordinaryDoc)
}
