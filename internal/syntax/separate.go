package syntax

import (
	"bytes"
	"slices"
)

// separate inserts the NL tokens (GRM-02, GRM-03) into the lexer's tokens. Brackets are tracked
// as written; an interpolation opens a bracket of its own, closed at the latest by the next
// line break (an interpolation never spans lines).
func separate(src []byte, toks []Token) []Token {
	s := &separator{toks: toks}
	out := make([]Token, 0, len(toks)+len(toks)/nlShare)
	for i, t := range toks {
		if i > 0 && toks[i-1].Kind != TokBOF && lineBreak(src, toks[i-1], t) {
			s.stack = dropInterps(s.stack)
			if s.separates(i) {
				out = append(out, nlToken(toks[i-1]))
			}
		}
		out = append(out, t)
		s.stack = trackBracket(s.stack, t.Kind)
	}
	return out
}

// separator is the state of the separator pass: the open brackets, and the end of the last run
// of annotations looked at, which every "@" of the run shares.
type separator struct {
	toks   []Token
	stack  []TokenKind
	runEnd int
}

// lineBreak reports a line break between p's end and t's start, comments included.
func lineBreak(src []byte, p, t Token) bool {
	return bytes.IndexByte(src[p.End:t.Start], '\n') >= 0
}

// separates applies rules 1 to 4 of GRAMMAR.md §3.1 to the break before toks[i].
func (s *separator) separates(i int) bool {
	if len(s.stack) > 0 && s.stack[len(s.stack)-1] != TokLBrace {
		return false
	}
	if cannotEnd[s.toks[i-1].Kind] {
		return false
	}
	if s.toks[i].Kind == TokAt {
		if i >= s.runEnd {
			s.runEnd = skipAnnotations(s.toks, i)
		}
		return startsDecl(s.toks, s.runEnd)
	}
	return !continuesLine[s.toks[i].Kind]
}

// skipAnnotations is the index after the run of annotations at i: "@" WORD ["(" … ")"].
func skipAnnotations(toks []Token, i int) int {
	for i+1 < len(toks) && toks[i].Kind == TokAt && isWord(toks[i+1].Kind) {
		i += pairWidth
		if i < len(toks) && toks[i].Kind == TokLParen {
			i = matchingParen(toks, i) + 1
		}
	}
	return i
}

// matchingParen is the index of the ")" closing the "(" at i, or the last index.
func matchingParen(toks []Token, i int) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		switch toks[j].Kind {
		case TokLParen:
			depth++
		case TokRParen:
			depth--
			if depth == 0 {
				return j
			}
		default:
		}
	}
	return len(toks) - 1
}

// startsDecl reports a declaration keyword at i, or "retired" before "entry" (§3.1 rule 4).
func startsDecl(toks []Token, i int) bool {
	if i >= len(toks) {
		return false
	}
	k := toks[i].Kind
	return declKeyword[k] || (k == KwRetired && i+1 < len(toks) && toks[i+1].Kind == KwEntry)
}

// nlToken is the empty NL token after p, at the border of p's trailing trivia.
func nlToken(p Token) Token {
	at := p.End
	if n := len(p.Trailing); n > 0 {
		at = p.Trailing[n-1].End
	}
	return Token{Kind: TokNL, Start: at, End: at}
}

func trackBracket(stack []TokenKind, k TokenKind) []TokenKind {
	switch k {
	case TokLParen, TokLBrack, TokLBrace, TokStringHead, TokMLStringHead:
		return append(stack, k)
	case TokStringTail, TokMLStringTail:
		return closeInterps(stack)
	case TokRParen, TokRBrack, TokRBrace:
		return popTo(stack, func(o TokenKind) bool { return o == openerOf[k] })
	default:
	}
	return stack
}

// closeInterps drops the innermost open interpolation and what it holds.
func closeInterps(stack []TokenKind) []TokenKind { return popTo(stack, isInterpOpen) }

// popTo drops the innermost bracket that is and what it holds; the stack is kept when none is.
func popTo(stack []TokenKind, is func(TokenKind) bool) []TokenKind {
	for i, k := range slices.Backward(stack) {
		if is(k) {
			return stack[:i]
		}
	}
	return stack
}

// dropInterps drops every open interpolation at a line break, and what they hold.
func dropInterps(stack []TokenKind) []TokenKind {
	if i := slices.IndexFunc(stack, isInterpOpen); i >= 0 {
		return stack[:i]
	}
	return stack
}

func isInterpOpen(k TokenKind) bool { return k == TokStringHead || k == TokMLStringHead }

// isWord reports a WORD: an identifier or a reserved word (GRAMMAR.md §2.3).
func isWord(k TokenKind) bool { return k == TokIdent || k >= KwAnd && k < TokenKindCount }
