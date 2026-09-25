package ir

import (
	"encoding/hex"
	"slices"
	"strings"
	"unicode/utf8"
)

// byteRange is the bytes lo..hi, both ASCII or both not: std::regex compares a plain char
// signed, so a bracket range never crosses 0x7F.
type byteRange struct{ lo, hi byte }

// classText is the code points of ranges (rune pairs, sorted) as UTF-8 byte sequences: one
// bracket for the ASCII ones, then per encoded length its compact form when the class holds all
// of it, else one alternative per sequence. Nothing is negated: that would match part of a code point.
func classText(ranges []rune) reText {
	valid := validRanges(ranges)
	var ascii []byteRange
	in := clipRanges(valid, 0, utf8.RuneSelf-1)
	for i := 0; i+1 < len(in); i += pairWidth {
		ascii = append(ascii, byteRange{string(in[i])[0], string(in[i+1])[0]}) // one byte each: ASCII
	}
	var alts []string
	if len(ascii) > 0 {
		alts = append(alts, bracket(ascii))
	}
	all := validRanges(anyRune)
	for _, sp := range utf8Spans {
		alts = append(alts, spanAlts(valid, all, sp.lo, sp.hi, sp.all)...)
	}
	switch {
	case len(alts) == 0:
		var b strings.Builder
		writeReByte(&b, reNoMatchByte)
		return reText{b.String(), reAtom}
	case len(alts) > 1:
		return reText{strings.Join(alts, reBar), reAlt}
	case len(ascii) > 0:
		return reText{alts[0], reAtom}
	}
	return reText{alts[0], reConcat}
}

// spanAlts are the alternatives of valid's code points lo..hi, one encoded length: compact when
// valid holds every one of them (as all does), one per byte-range sequence otherwise.
func spanAlts(valid, all []rune, lo, hi rune, compact string) []string {
	in := clipRanges(valid, lo, hi)
	if len(in) > 0 && slices.Equal(in, clipRanges(all, lo, hi)) {
		return []string{compact}
	}
	var out []string
	for i := 0; i+1 < len(in); i += pairWidth {
		for _, seq := range appendSeqs(nil, in[i], in[i+1]) {
			out = append(out, seqText(seq))
		}
	}
	return out
}

// validRanges are ranges without the surrogates, which valid UTF-8 never encodes.
func validRanges(ranges []rune) []rune {
	var out []rune
	for i := 0; i+1 < len(ranges); i += pairWidth {
		lo, hi := ranges[i], min(ranges[i+1], utf8.MaxRune)
		if lo <= surrogateMax && hi >= surrogateMin {
			if lo < surrogateMin {
				out = append(out, lo, surrogateMin-1)
			}
			lo = surrogateMax + 1
		}
		if lo <= hi {
			out = append(out, lo, hi)
		}
	}
	return out
}

// clipRanges is the part of ranges within lo..hi.
func clipRanges(ranges []rune, lo, hi rune) []rune {
	var out []rune
	for i := 0; i+1 < len(ranges); i += pairWidth {
		if a, b := max(ranges[i], lo), min(ranges[i+1], hi); a <= b {
			out = append(out, a, b)
		}
	}
	return out
}

// appendSeqs appends the byte-range sequences of lo..hi, one encoded length, split until each
// position is one range.
func appendSeqs(out [][]byteRange, lo, hi rune) [][]byteRange {
	if lo > hi {
		return out
	}
	if mid, ok := seqSplit(lo, hi); ok {
		return appendSeqs(appendSeqs(out, lo, mid), mid+1, hi)
	}
	a, b := utf8.AppendRune(nil, lo), utf8.AppendRune(nil, hi)
	seq := make([]byteRange, len(a))
	for i := range a {
		seq[i] = byteRange{a[i], b[i]}
	}
	return append(out, seq)
}

// seqSplit is where lo..hi (one encoded length) splits so a continuation byte never wraps; ok is
// false when every byte position is already one range.
func seqSplit(lo, hi rune) (mid rune, ok bool) {
	for i := 1; i < utf8.UTFMax; i++ {
		m := rune(1)<<(utf8ContBits*i) - 1
		switch {
		case lo&^m == hi&^m:
		case lo&m != 0:
			return lo | m, true
		case hi&m != m:
			return (hi &^ m) - 1, true
		}
	}
	return 0, false
}

// bracket is ASCII ranges as one class, or one byte alone.
func bracket(rs []byteRange) string {
	var b strings.Builder
	if len(rs) == 1 && rs[0].lo == rs[0].hi {
		writeReByte(&b, rs[0].lo)
		return b.String()
	}
	b.WriteString(reClassOpen)
	for _, r := range rs {
		writeRange(&b, r)
	}
	b.WriteString(reClassClose)
	return b.String()
}

// seqText is one multi-byte sequence: a byte, or a one-range class, per position.
func seqText(seq []byteRange) string {
	var b strings.Builder
	for _, r := range seq {
		if r.lo == r.hi {
			writeReByte(&b, r.lo)
			continue
		}
		b.WriteString(reClassOpen)
		writeRange(&b, r)
		b.WriteString(reClassClose)
	}
	return b.String()
}

func writeRange(b *strings.Builder, r byteRange) {
	writeReByte(b, r.lo)
	if r.hi != r.lo {
		b.WriteString(reRangeDash)
		writeReByte(b, r.hi)
	}
}

// writeReByte writes c raw when it is a letter, a digit or `_`, as `\xHH` otherwise: literal
// in and out of brackets under every grammar and locale.
func writeReByte(b *strings.Builder, c byte) {
	if c < utf8.RuneSelf && reRawBytes[c] {
		b.WriteByte(c)
		return
	}
	b.WriteString(reHexEscape + hex.EncodeToString([]byte{c}))
}
