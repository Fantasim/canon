package ir

import (
	"fmt"
	"regexp"
	resyntax "regexp/syntax"
	"strconv"
	"strings"
	"unicode/utf8"
)

// CppPattern is re for C++ std::regex (ECMAScript grammar, no other flag) searching UTF-8 bytes: std::regex_search accepts a text exactly when re.MatchString does, for valid UTF-8 text only, which EVALUATION.md §11.3 guarantees (log 2026-09-24 "Pattern semantics"); the result is printable ASCII without `"` or `??` and reads no locale.
func CppPattern(re *regexp.Regexp) (string, error) {
	tree, err := resyntax.Parse(re.String(), resyntax.Perl)
	if err != nil {
		return "", fmt.Errorf(fmtPatternParse, ErrPattern, err)
	}
	var x reTranslator
	t := x.node(tree)
	if x.err != nil {
		return "", x.err
	}
	return t.s, nil
}

// reTranslator holds the first construct found without a translation.
type reTranslator struct{ err error }

// reText is a translated subexpression and how tightly it binds (reAlt < reConcat < reAtom).
type reText struct {
	s    string
	prec int
}

// at is t as an operand binding at least as tightly as prec.
func (t reText) at(prec int) string {
	if t.prec < prec {
		return reGroupOpen + t.s + reGroupClose
	}
	return t.s
}

// reOps translates each operator the translation keeps; any other has no translation.
var reOps map[resyntax.Op]func(*reTranslator, *resyntax.Regexp) reText

func init() {
	reOps = map[resyntax.Op]func(*reTranslator, *resyntax.Regexp) reText{
		resyntax.OpNoMatch: (*reTranslator).noMatch, resyntax.OpEmptyMatch: (*reTranslator).empty,
		resyntax.OpLiteral: (*reTranslator).literal, resyntax.OpCharClass: (*reTranslator).class,
		resyntax.OpAnyCharNotNL: (*reTranslator).anyNotNL, resyntax.OpAnyChar: (*reTranslator).any,
		resyntax.OpBeginText: (*reTranslator).begin, resyntax.OpEndText: (*reTranslator).end,
		resyntax.OpCapture: (*reTranslator).capture, resyntax.OpStar: (*reTranslator).quantify,
		resyntax.OpPlus: (*reTranslator).quantify, resyntax.OpQuest: (*reTranslator).quantify,
		resyntax.OpRepeat: (*reTranslator).quantify, resyntax.OpConcat: (*reTranslator).concat,
		resyntax.OpAlternate: (*reTranslator).alternate,
	}
}

// node translates one node; multi-line anchors and word boundaries have none (E1904 refuses `(?m`, `\b`, `\B`).
func (x *reTranslator) node(re *resyntax.Regexp) reText {
	op, ok := reOps[re.Op]
	if !ok {
		return x.refuse(re)
	}
	return op(x, re)
}

func (x *reTranslator) refuse(re *resyntax.Regexp) reText {
	if x.err == nil {
		x.err = fmt.Errorf(fmtPatternOp, ErrPattern, re)
	}
	return reText{}
}

// noMatch is byte 0xFF, which valid UTF-8 never holds.
func (*reTranslator) noMatch(*resyntax.Regexp) reText { return classText(nil) }

func (*reTranslator) empty(*resyntax.Regexp) reText { return reText{reEmptyGroup, reAtom} }

// literal is the UTF-8 bytes of the runes; a surrogate, which valid UTF-8 never encodes and
// RE2 never matches, makes it a no-match; case folding needs `(?i`, which E1904 refuses.
func (x *reTranslator) literal(re *resyntax.Regexp) reText {
	if re.Flags&resyntax.FoldCase != 0 {
		return x.refuse(re)
	}
	var b strings.Builder
	for _, r := range re.Rune {
		if !utf8.ValidRune(r) {
			return x.noMatch(re)
		}
		for _, c := range utf8.AppendRune(nil, r) {
			writeReByte(&b, c)
		}
	}
	if len(re.Rune) == 1 && re.Rune[0] < utf8.RuneSelf {
		return reText{b.String(), reAtom}
	}
	return reText{b.String(), reConcat}
}

func (*reTranslator) class(re *resyntax.Regexp) reText { return classText(re.Rune) }

// anyNotNL is `.`: one code point but `\n`, never a byte of one.
func (*reTranslator) anyNotNL(*resyntax.Regexp) reText { return classText(anyRuneNotNL) }

// any is any code point: the parser's form of a class such as `[\s\S]`.
func (*reTranslator) any(*resyntax.Regexp) reText { return classText(anyRune) }

// begin and end are `^` and `$` of a pattern without `(?m`: the text's two ends in both grammars.
func (*reTranslator) begin(*resyntax.Regexp) reText { return reText{reCaret, reConcat} }

func (*reTranslator) end(*resyntax.Regexp) reText { return reText{reDollar, reConcat} }

// capture drops the group: only acceptance is observable.
func (x *reTranslator) capture(re *resyntax.Regexp) reText { return x.node(re.Sub[0]) }

// quantify drops laziness, which never changes whether a search matches: the result then never
// holds `??`, a trigraph a C++ string literal would warn about.
func (x *reTranslator) quantify(re *resyntax.Regexp) reText {
	q := reQuantifiers[re.Op]
	if re.Op == resyntax.OpRepeat {
		q = repeatBounds(re.Min, re.Max)
	}
	return reText{x.node(re.Sub[0]).at(reAtom) + q, reConcat}
}

// repeatBounds is `{n}`, `{n,}` or `{n,m}`; a negative max is unbounded.
func repeatBounds(lo, hi int) string {
	n := strconv.Itoa(lo)
	switch {
	case hi == lo:
		return reBraceOpen + n + reBraceClose
	case hi < 0:
		return reBraceOpen + n + reComma + reBraceClose
	}
	return reBraceOpen + n + reComma + strconv.Itoa(hi) + reBraceClose
}

func (x *reTranslator) concat(re *resyntax.Regexp) reText {
	if len(re.Sub) == 0 {
		return x.empty(re)
	}
	var b strings.Builder
	for _, s := range re.Sub {
		b.WriteString(x.node(s).at(reConcat))
	}
	return reText{b.String(), reConcat}
}

func (x *reTranslator) alternate(re *resyntax.Regexp) reText {
	parts := make([]string, 0, len(re.Sub))
	for _, s := range re.Sub {
		parts = append(parts, x.node(s).s)
	}
	return reText{strings.Join(parts, reBar), reAlt}
}
