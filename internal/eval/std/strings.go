package std

import (
	"strings"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// stringMethods are STDLIB.md §7: bytes of the UTF-8 encoding, ASCII case mapping, one step each.
func stringMethods() map[string]builtin {
	return map[string]builtin{
		bLen: strLen, bIsEmpty: strIsEmpty, bContains: strPred(strings.Contains),
		bStartsWith: strPred(strings.HasPrefix), bEndsWith: strPred(strings.HasSuffix),
		bFind: strFind, bSplit: strSplit, bTrim: strTrim, bLower: strMap(lowerASCII),
		bUpper: strMap(upperASCII), bReplace: strReplace, bMatches: strMatches,
	}
}

// strOf is the string a String value holds; "" for any other value.
func strOf(v value.Value) string {
	if s, ok := v.(*value.Str); ok {
		return s.V
	}
	return ""
}

func strLen(h Host, c *Call) (value.Value, bool) {
	return c.intv(int64(len(strOf(c.Recv)))), h.Charge(1)
}

func strIsEmpty(h Host, c *Call) (value.Value, bool) {
	return c.boolv(strOf(c.Recv) == ""), h.Charge(1)
}

func strPred(f func(s, t string) bool) builtin {
	return func(h Host, c *Call) (value.Value, bool) {
		return c.boolv(f(strOf(c.Recv), strOf(c.arg(0)))), h.Charge(1)
	}
}

// strMap is a case mapping: 1, then a step per byte of its result before it is made (DECISIONS 197).
func strMap(f func(s string) string) builtin {
	return func(h Host, c *Call) (value.Value, bool) {
		s := strOf(c.Recv)
		if !h.Charge(1 + len(s)) {
			return nil, false
		}
		return c.strv(f(s)), true
	}
}

// strTrim is 1, then a step per byte of the trimmed string (DECISIONS 197).
func strTrim(h Host, c *Call) (value.Value, bool) {
	t := trimASCII(strOf(c.Recv))
	if !h.Charge(1 + len(t)) {
		return nil, false
	}
	return c.strv(t), true
}

// strFind is the byte index of the first occurrence; "x".find("") is 0.
func strFind(h Host, c *Call) (value.Value, bool) {
	i := strings.Index(strOf(c.Recv), strOf(c.arg(0)))
	return c.orNone(c.intv(int64(i)), i >= 0), h.Charge(1)
}

// strSplit: "a,,b" gives three parts, "" gives [""]; E4106 on an empty separator. It costs 1,
// then a step per part and per byte of the parts, before they are made (DECISIONS 197).
func strSplit(h Host, c *Call) (value.Value, bool) {
	sep := strOf(c.arg(0))
	if sep == "" {
		h.Fail(diag.E4106.AtSeparator(h.Site(), bSplit))
		return nil, false
	}
	s := strOf(c.Recv)
	n := strings.Count(s, sep) + 1
	if !h.Charge(1 + n + len(s) - (n-1)*len(sep)) {
		return nil, false
	}
	parts := strings.Split(s, sep)
	out := make([]value.Value, len(parts))
	for i, p := range parts {
		out[i] = c.strv(p)
	}
	return c.list(out), true
}

// strReplace replaces every non-overlapping occurrence, left to right; E4106 on an empty
// pattern. It costs 1, then a step per byte of its result before it is made (DECISIONS 197).
func strReplace(h Host, c *Call) (value.Value, bool) {
	a := strOf(c.arg(0))
	if a == "" {
		h.Fail(diag.E4106.AtPattern(h.Site(), bReplace))
		return nil, false
	}
	s, b := strOf(c.Recv), strOf(c.arg(1))
	if !h.Charge(1 + len(s) + strings.Count(s, a)*(len(b)-len(a))) {
		return nil, false
	}
	return c.strv(strings.ReplaceAll(s, a, b)), true
}

// strMatches is an RE2 search (STDLIB.md §8).
func strMatches(h Host, c *Call) (value.Value, bool) {
	re := h.Regexp(strOf(c.arg(0)))
	return c.boolv(re != nil && re.MatchString(strOf(c.Recv))), h.Charge(1)
}

// trimASCII removes leading and trailing space, \t, \n, \v, \f and \r.
func trimASCII(s string) string {
	return strings.Trim(s, asciiSpace)
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= upperA && c <= upperZ {
			b[i] = c | caseBit
		}
	}
	return string(b)
}

func upperASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= lowerA && c <= lowerZ {
			b[i] = c &^ caseBit
		}
	}
	return string(b)
}

// SliceBounds resolves a slice's bounds against length n (STDLIB.md §4.1).
func SliceBounds(h Host, n, a, b int64) (int64, int64, bool) {
	lo, hi := a, b
	if lo < 0 {
		lo += n
	}
	if hi < 0 {
		hi += n
	}
	if lo < 0 || lo > hi || hi > n {
		h.Fail(diag.E4002.AtSlice(h.Site(), a, b))
		return 0, 0, false
	}
	return lo, hi, true
}

// edge is a slice bound: the byte offset it resolves to, and the bound as written.
type edge struct{ at, written int64 }

// SliceString is s[a..b] on bytes: E4107 when a bound falls inside a UTF-8 character.
func SliceString(h Host, s string, a, b int64, p *value.Prov) (value.Value, bool) {
	lo, hi, ok := SliceBounds(h, int64(len(s)), a, b)
	if !ok {
		return nil, false
	}
	for _, e := range []edge{{lo, a}, {hi, b}} {
		if e.at < int64(len(s)) && !utf8.RuneStart(s[e.at]) {
			h.Fail(diag.E4107.At(h.Site(), e.written))
			return nil, false
		}
	}
	return &value.Str{V: s[lo:hi], T: types.StringType, P: p}, h.Charge(1)
}
