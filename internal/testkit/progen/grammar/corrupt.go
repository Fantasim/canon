package grammar

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// corruptions are the token mutations of DECISIONS 200: each rewrites one token of a lexed
// file, or bytes around it.
var corruptions = []func(r Rand, src []byte, toks []syntax.Token, i int) []byte{
	dropToken, doubleToken, swapTokens, replaceToken, insertByte, truncateAt,
}

// Corrupt is src with one to three token mutations: a token dropped, doubled, swapped with the
// next, replaced by a punctuation or keyword, a byte inserted (control, invalid UTF-8, a stray
// quote or brace), or the file cut short.
func Corrupt(r Rand, src []byte) []byte {
	for n := 1 + r.Intn(maxArgs); n > 0; n-- {
		toks := lexed(src)
		if len(toks) == 0 {
			return src
		}
		i := r.Intn(len(toks))
		src = corruptions[r.Intn(len(corruptions))](r, src, toks, i)
	}
	return src
}

// lexed are the tokens of src that have text; every kind lexes as a source file.
func lexed(src []byte) []syntax.Token {
	fs := &source.FileSet{}
	f, err := fs.Add(corruptFile, rootPath+corruptFile, src)
	if err != nil {
		return nil
	}
	var out []syntax.Token
	for _, t := range syntax.Parse(f, syntax.FileSource, diag.NewBag(fs, "")).Tokens {
		if t.End > t.Start && int(t.End) <= len(src) {
			out = append(out, t)
		}
	}
	return out
}

func splice(src []byte, start, end int, with []byte) []byte {
	out := make([]byte, 0, len(src)+len(with))
	out = append(out, src[:start]...)
	out = append(out, with...)
	return append(out, src[end:]...)
}

func dropToken(_ Rand, src []byte, toks []syntax.Token, i int) []byte {
	return splice(src, int(toks[i].Start), int(toks[i].End), nil)
}

func doubleToken(_ Rand, src []byte, toks []syntax.Token, i int) []byte {
	t := toks[i]
	return splice(src, int(t.End), int(t.End), append([]byte(space), src[t.Start:t.End]...))
}

func swapTokens(_ Rand, src []byte, toks []syntax.Token, i int) []byte {
	if i+1 >= len(toks) {
		return src
	}
	a, b := toks[i], toks[i+1]
	mid := append([]byte(nil), src[a.End:b.Start]...)
	swapped := append(append(append([]byte(nil), src[b.Start:b.End]...), mid...), src[a.Start:a.End]...)
	return splice(src, int(a.Start), int(b.End), swapped)
}

func replaceToken(r Rand, src []byte, toks []syntax.Token, i int) []byte {
	return splice(src, int(toks[i].Start), int(toks[i].End), []byte(pickOf(r, fixedTokens).String()))
}

func insertByte(r Rand, src []byte, toks []syntax.Token, i int) []byte {
	at := int(toks[i].Start) + r.Intn(int(toks[i].End-toks[i].Start)+1)
	return splice(src, at, at, []byte(pickOf(r, strayBytes)))
}

func truncateAt(r Rand, src []byte, toks []syntax.Token, i int) []byte {
	return src[:int(toks[i].Start)+r.Intn(int(toks[i].End-toks[i].Start)+1)]
}
