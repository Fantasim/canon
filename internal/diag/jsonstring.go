package diag

import (
	"fmt"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// AppendJSONString appends s as WIRE.md §7.3 writes a string, invalid UTF-8 as U+FFFD.
func AppendJSONString(dst []byte, s string) []byte {
	dst = append(dst, quote)
	for i := 0; i < len(s); {
		c := s[i]
		if esc, ok := jsonShortEscapes[c]; ok {
			dst = append(dst, esc...)
			i++
			continue
		}
		if c < controlLimit {
			dst = append(dst, unicodeEscape...)
			dst = append(dst, hexLower[c>>hexShift], hexLower[c&hexLowMask])
			i++
			continue
		}
		r, width := utf8.DecodeRuneInString(s[i:])
		dst = utf8.AppendRune(dst, r)
		i += width
	}
	return append(dst, quote)
}

// UnquoteJSON reads the JSON string s starts with: its value and byte length (WIRE.md §3.2).
func UnquoteJSON(s string) (string, int, error) {
	if s == "" || s[0] != quote {
		return "", 0, fmt.Errorf("%w: %w", ErrJSONString, errNoQuote)
	}
	var out []byte
	for i := 1; i < len(s); {
		c := s[i]
		switch {
		case c == quote:
			return string(out), i + 1, nil
		case c == backslash:
			r, width, err := unescape(s[i:])
			if err != nil {
				return "", 0, fmt.Errorf("%w: byte %d: %w", ErrJSONString, i, err)
			}
			out = utf8.AppendRune(out, r)
			i += width
		case c < controlLimit:
			return "", 0, fmt.Errorf("%w: byte %d: %w", ErrJSONString, i, errControl)
		default:
			r, width := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && width == 1 {
				return "", 0, fmt.Errorf("%w: byte %d: %w", ErrJSONString, i, errUTF8)
			}
			out = append(out, s[i:i+width]...)
			i += width
		}
	}
	return "", 0, fmt.Errorf("%w: %w", ErrJSONString, errUnterminated)
}

// unescape reads the escape s starts with (its `\` included): its rune and its width.
func unescape(s string) (rune, int, error) {
	if len(s) < shortEscapeWidth {
		return 0, 0, errEscape
	}
	if c, ok := jsonDecodedEscapes[s[1]]; ok {
		return rune(c), shortEscapeWidth, nil
	}
	if s[1] != unicodeLetter {
		return 0, 0, errEscape
	}
	hi, err := hex4(s[shortEscapeWidth:])
	if err != nil {
		return 0, 0, err
	}
	if !utf16.IsSurrogate(hi) {
		return hi, uEscapeWidth, nil
	}
	if hi >= surrogateLow || len(s) < pairWidth || s[uEscapeWidth] != backslash || s[uEscapeWidth+1] != unicodeLetter {
		return 0, 0, errSurrogate
	}
	lo, err := hex4(s[uEscapeWidth+shortEscapeWidth:])
	if err != nil {
		return 0, 0, err
	}
	r := utf16.DecodeRune(hi, lo)
	if r == utf8.RuneError {
		return 0, 0, errSurrogate
	}
	return r, pairWidth, nil
}

// hex4 reads the four hexadecimal digits s starts with.
func hex4(s string) (rune, error) {
	if len(s) < hexRuneDigits {
		return 0, errEscape
	}
	n, err := strconv.ParseUint(s[:hexRuneDigits], hexBase, hexRuneBits)
	if err != nil {
		return 0, errEscape
	}
	return rune(n), nil
}
