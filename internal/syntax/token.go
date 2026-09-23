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

// Token is one token with its trivia: Trailing runs to the first line break after it, Leading
// holds the rest up to the token. A string part spans its quote or brace, an interpolation's
// braces included; FORMAT_SPEC spans ":spec"; NL is empty, at the Trailing/Leading border.
type Token struct {
	Kind       TokenKind
	Start, End source.Pos
	Leading    []Trivia
	Trailing   []Trivia
}

// Tok is the index of a token in File.Tokens; NoTok (0, the BOF sentinel) is none.
type Tok int32

func (t Tok) Valid() bool { return t > NoTok }

// FormatSpec is the ":spec" of an interpolation (GRAMMAR.md §2.6).
type FormatSpec struct {
	Tok      Tok
	Plus     bool
	Comma    bool
	Decimals int
}

// Delims are the opening and closing brackets of a list; both are NoTok when it is absent.
type Delims struct {
	Open, Close Tok
}
