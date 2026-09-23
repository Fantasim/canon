package syntax

import (
	"math"
	"math/big"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
)

// maxMillis is the largest duration magnitude: 2^63 ms, the negated minimum of an int64.
var maxMillis = new(big.Int).Lsh(big.NewInt(1), maxMillisBits)

// numProblem is what is wrong with a numeric literal (GRAMMAR.md §2.4, §2.5).
type numProblem uint8

// numResult is a classified numeric literal: its kind, its problem and, for a duration, the
// unit repeated and the magnitude in milliseconds.
type numResult struct {
	kind    TokenKind
	problem numProblem
	unit    string
	millis  uint64
}

// number reads a numeric literal and reports E1110 or E1111 (GRAMMAR.md §2.4, §2.5).
func (l *lexer) number() {
	start := l.pos
	l.pos = numberEnd(l.src, start)
	text := l.text(start, l.pos)
	r := classifyNumber(text)
	l.emit(r.kind, start)
	sp := l.span(start, l.pos)
	switch r.problem {
	case numBad:
		diag.E1110.At(sp, text).Report(l.bag)
	case durOrder:
		diag.E1111.AtOrder(sp, text).Report(l.bag)
	case durRepeated:
		diag.E1111.AtRepeated(sp, text, r.unit).Report(l.bag)
	case durFraction:
		diag.E1111.AtFraction(sp, text).Report(l.bag)
	case durTrailing:
		diag.E1111.AtTrailing(sp, text).Report(l.bag)
	case durOverflow:
		diag.E1111.AtOverflow(sp, text).Report(l.bag)
	case numOK:
	}
}

// numberEnd is the end of the literal at i: letters, digits and "_", a fraction when "." is
// followed by a digit, and a signed exponent.
func numberEnd(src []byte, i int) int {
	j := wordEnd(src, i)
	if isDigits(src[i:j]) && j+1 < len(src) && src[j] == '.' && isDigit(src[j+1]) {
		j = wordEnd(src, j+1)
	}
	if expOpen(src[i:j]) && j+1 < len(src) && (src[j] == '+' || src[j] == '-') && isDigit(src[j+1]) {
		j = wordEnd(src, j+1)
	}
	return j
}

func wordEnd(src []byte, i int) int {
	for i < len(src) && isWordByte(src[i]) {
		i++
	}
	return i
}

func isDigits(b []byte) bool {
	for _, c := range b {
		if !isDigit(c) && c != '_' {
			return false
		}
	}
	return true
}

// expOpen reports a decimal or a fraction ending with the "e" of an exponent.
func expOpen(b []byte) bool {
	n := len(b)
	if n < minExpLen || b[n-1]|asciiLowerBit != 'e' {
		return false
	}
	whole, frac, _ := strings.Cut(string(b[:n-1]), fractionDot)
	return isDigits([]byte(whole)) && isDigits([]byte(frac))
}

// classifyNumber checks text against INT, FLOAT and DURATION (GRAMMAR.md §2.4, §2.5).
func classifyNumber(text string) numResult {
	if base, digits, ok := prefixed(text); ok {
		return numResult{kind: TokInt, problem: checkIf(validDigits(digits, base))}
	}
	whole := decimalEnd(text, 0)
	if !validDecimal(text[:whole]) {
		return numResult{kind: TokInt, problem: numBad}
	}
	if whole == len(text) {
		return numResult{kind: TokInt}
	}
	switch c := text[whole]; {
	case c == '.' || c|asciiLowerBit == 'e':
		return classifyFloat(text, whole)
	case unitStart(c):
		return classifyDuration(text)
	}
	return numResult{kind: TokInt, problem: numBad}
}

// classifyFloat checks "decimal [. digits] [exponent]" from the end of the whole part; units
// after it make a duration with a fraction (E1111).
func classifyFloat(text string, whole int) numResult {
	i := whole
	if text[i] == '.' {
		i = decimalEnd(text, i+1)
		if !validDigits(text[whole+1:i], decimalBase) {
			return numResult{kind: TokFloat, problem: numBad}
		}
	}
	if i < len(text) && unitStart(text[i]) && text[i]|asciiLowerBit != 'e' {
		return numResult{kind: TokDuration, problem: durFraction}
	}
	if i < len(text) {
		if _, ok := exponent(text[i:]); !ok {
			return numResult{kind: TokFloat, problem: numBad}
		}
	}
	if _, _, ok := floatParts(text); !ok {
		return numResult{kind: TokFloat, problem: numBad}
	}
	return numResult{kind: TokFloat}
}

// classifyDuration checks "decimal unit { decimal unit }", units strictly decreasing.
func classifyDuration(text string) numResult {
	r := numResult{kind: TokDuration}
	last := -1
	var total big.Int
	for i := 0; i < len(text); {
		end := decimalEnd(text, i)
		if end == len(text) {
			return numResult{kind: TokDuration, problem: durTrailing}
		}
		unit, rank := unitAt(text, end)
		if !validDecimal(text[i:end]) || rank < 0 {
			return numResult{kind: TokDuration, problem: durationProblem(text[end])}
		}
		if rank <= last {
			return numResult{kind: TokDuration, problem: orderProblem(rank == last), unit: unit}
		}
		last = rank
		addUnits(&total, text[i:end], rank)
		i = end + len(unit)
	}
	if total.Cmp(maxMillis) > 0 {
		return numResult{kind: TokDuration, problem: durOverflow}
	}
	r.millis = total.Uint64()
	return r
}

func durationProblem(c byte) numProblem {
	if c == '.' {
		return durFraction
	}
	return numBad
}

func orderProblem(repeated bool) numProblem {
	if repeated {
		return durRepeated
	}
	return durOrder
}

func addUnits(total *big.Int, digits string, rank int) {
	var n big.Int
	n.SetString(strings.ReplaceAll(digits, digitSep, ""), decimalBase)
	n.Mul(&n, big.NewInt(unitMillis[rank]))
	total.Add(total, &n)
}

// unitAt is the unit at i, longest first, and its rank (d h m s ms), or rank -1.
func unitAt(text string, i int) (string, int) {
	for rank, u := range unitsLongestFirst {
		if strings.HasPrefix(text[i:], u) {
			return u, unitRank[rank]
		}
	}
	return "", -1
}

func unitStart(c byte) bool { return strings.IndexByte(unitLetters, c) >= 0 }

// decimalEnd is the end of the run of digits and "_" starting at i.
func decimalEnd(text string, i int) int {
	for i < len(text) && (isDigit(text[i]) || text[i] == '_') {
		i++
	}
	return i
}

// validDecimal is a decimal with "_" only between digits and no leading zero (GRAMMAR.md §2.4).
func validDecimal(s string) bool {
	return validDigits(s, decimalBase) && (len(s) == 1 || s[0] != '0')
}

// validDigits is a non-empty run of digits of base with "_" only between two of them.
func validDigits(s string, base int) bool {
	if s == "" || s[0] == '_' || s[len(s)-1] == '_' || strings.Contains(s, doubleSep) {
		return false
	}
	for i := range len(s) {
		if s[i] != '_' && digitValue(s[i]) >= base {
			return false
		}
	}
	return true
}

func digitValue(c byte) int {
	switch {
	case isDigit(c):
		return int(c - '0')
	case c|asciiLowerBit >= 'a' && c|asciiLowerBit <= 'z':
		return int(c|asciiLowerBit-'a') + decimalBase
	}
	return math.MaxInt
}

// prefixed splits "0x…" and "0b…" into their base and digits.
func prefixed(text string) (int, string, bool) {
	switch {
	case strings.HasPrefix(text, hexPrefix):
		return hexBase, text[len(hexPrefix):], true
	case strings.HasPrefix(text, binPrefix):
		return binBase, text[len(binPrefix):], true
	}
	return 0, "", false
}

func checkIf(ok bool) numProblem {
	if ok {
		return numOK
	}
	return numBad
}

// exponent reads "e[+-]digits", the digits without a leading-zero rule, as an exact integer.
func exponent(s string) (*big.Int, bool) {
	if len(s) < minExpLen || s[0]|asciiLowerBit != 'e' {
		return nil, false
	}
	digits := s[1:]
	if digits[0] == '+' || digits[0] == '-' {
		digits = digits[1:]
	}
	if !validDigits(digits, decimalBase) {
		return nil, false
	}
	var e big.Int
	e.SetString(strings.ReplaceAll(s[1:], digitSep, ""), decimalBase)
	return &e, true
}
