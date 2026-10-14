package syntax

import "github.com/fantasim/canonlang/internal/source"

// TokenKind is a token class, a punctuation or a reserved word; String is its name or text.
type TokenKind uint16

func (k TokenKind) String() string {
	if k >= TokenKindCount {
		return tokenNames[TokInvalid]
	}
	return tokenNames[k]
}

type TriviaKind uint8

// Trivia is one run of whitespace, one line break or one comment.
type Trivia struct {
	Kind       TriviaKind
	Start, End source.Pos
}

// Token is one token; its trivia is the bytes around it, which File.AppendLeading and
// File.AppendTrailing read. A string part spans its quote or brace, an interpolation's braces
// included; FORMAT_SPEC spans ":spec"; NL is empty, at the Trailing/Leading border.
type Token struct {
	Kind       TokenKind
	Start, End source.Pos
}

// Tok is the index of a token in File.Tokens; NoTok (0, the BOF sentinel) is none.
type Tok int32

func (t Tok) Valid() bool { return t > NoTok }

// FormatSpec is an interpolation's ":spec" (GRAMMAR.md §2.6); a zero Width or Hex: that part absent.
type FormatSpec struct {
	Tok      Tok
	Plus     bool
	Comma    bool
	Decimals int
	Zero     bool
	Width    int
	Hex      byte
}

// Delims are the opening and closing brackets of a list; both are NoTok when it is absent.
type Delims struct {
	Open, Close Tok
}
