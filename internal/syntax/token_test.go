package syntax_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

var keywords = []syntax.TokenKind{
	syntax.KwAnd, syntax.KwAmend, syntax.KwAs, syntax.KwAsset, syntax.KwBreak, syntax.KwCheck,
	syntax.KwConst, syntax.KwContinue, syntax.KwElse, syntax.KwEmit, syntax.KwEntry, syntax.KwEnum,
	syntax.KwExport, syntax.KwExpect, syntax.KwFalse, syntax.KwFn, syntax.KwFor, syntax.KwIf,
	syntax.KwImport, syntax.KwIn, syntax.KwInput, syntax.KwIs, syntax.KwLayer, syntax.KwLet,
	syntax.KwLoad, syntax.KwLocal, syntax.KwMatch, syntax.KwNone, syntax.KwNot, syntax.KwOr,
	syntax.KwPackage, syntax.KwProject, syntax.KwRecord, syntax.KwRef, syntax.KwRetired,
	syntax.KwReturn, syntax.KwSelf, syntax.KwStable, syntax.KwTable, syntax.KwTest,
	syntax.KwTranslation, syntax.KwTrue, syntax.KwType, syntax.KwVar, syntax.KwVariant,
	syntax.KwView, syntax.KwWarn, syntax.KwWhere, syntax.KwWhile, syntax.KwWidget,
}

var punctuation = []syntax.TokenKind{
	syntax.TokEllipsis, syntax.TokRangeIncl, syntax.TokRange, syntax.TokCoalesce, syntax.TokOptDot,
	syntax.TokFatArrow, syntax.TokArrow, syntax.TokEq, syntax.TokNe, syntax.TokLe, syntax.TokGe,
	syntax.TokAddAssign, syntax.TokSubAssign, syntax.TokMulAssign, syntax.TokDivAssign,
	syntax.TokPlus, syntax.TokMinus, syntax.TokStar, syntax.TokSlash, syntax.TokPercent,
	syntax.TokLt, syntax.TokGt, syntax.TokAssign, syntax.TokBang, syntax.TokQuestion, syntax.TokDot,
	syntax.TokComma, syntax.TokColon, syntax.TokLParen, syntax.TokRParen, syntax.TokLBrack,
	syntax.TokRBrack, syntax.TokLBrace, syntax.TokRBrace, syntax.TokAt, syntax.TokPipe,
	syntax.TokUnderscore, syntax.TokHash,
}

// The token classes of GRAMMAR.md §2.1 (DOC is trivia), with the lexer's own kinds.
var classes = map[syntax.TokenKind]string{
	syntax.TokInvalid: "INVALID", syntax.TokBOF: "BOF", syntax.TokEOF: "EOF", syntax.TokNL: "NL", syntax.TokIllegal: "ILLEGAL",
	syntax.TokIdent: "IDENT", syntax.TokInt: "INT", syntax.TokFloat: "FLOAT", syntax.TokDuration: "DURATION",
	syntax.TokString: "STRING", syntax.TokStringHead: "STRING_HEAD", syntax.TokStringMid: "STRING_MID",
	syntax.TokStringTail: "STRING_TAIL", syntax.TokMLString: "MLSTRING",
	syntax.TokMLStringHead: "MLSTRING_HEAD", syntax.TokMLStringMid: "MLSTRING_MID",
	syntax.TokMLStringTail: "MLSTRING_TAIL", syntax.TokRaw: "RAW", syntax.TokRawML: "RAWML",
	syntax.TokRegex: "REGEX", syntax.TokFormatSpec: "FORMAT_SPEC",
}

func spellings(kinds []syntax.TokenKind) []string {
	out := make([]string, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, k.String())
	}
	return out
}

// GRAMMAR.md §4.1: each reserved word is its own token kind, spelled as the word.
func TestKeywordsAreTheReservedWords(t *testing.T) {
	want := strings.Fields(strings.Join(grammarBlocks(t, false, "4.1"), " "))
	if got := spellings(keywords); !slices.Equal(got, want) {
		t.Errorf("keywords = %v, want %v", got, want)
	}
}

// GRAMMAR.md §2.8: the punctuation, longest first.
func TestPunctuation(t *testing.T) {
	want := strings.Fields(strings.Join(grammarBlocks(t, false, "2.8"), " "))
	if got := spellings(punctuation); !slices.Equal(got, want) {
		t.Errorf("punctuation = %v, want %v", got, want)
	}
}

// Every token kind is a class, a punctuation or a keyword, and prints a distinct name.
func TestTokenKindsPartition(t *testing.T) {
	seen := map[string]bool{}
	for k := syntax.TokInvalid; k < syntax.TokenKindCount; k++ {
		name := k.String()
		want, isClass := classes[k]
		fixed := slices.Contains(keywords, k) || slices.Contains(punctuation, k)
		if isClass == fixed || (isClass && name != want) || seen[name] {
			t.Errorf("token kind %d (%q) is not exactly one class, punctuation or keyword", k, name)
		}
		seen[name] = true
	}
	if got := syntax.TokenKindCount.String(); got != syntax.TokInvalid.String() {
		t.Errorf("out-of-range token kind prints %q", got)
	}
}
