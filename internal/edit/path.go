package edit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
)

// SegKind is the form of a path segment (API.md §6.1).
type SegKind uint8

// KeyLitKind is the form of a key between brackets (API.md §6.1).
type KeyLitKind uint8

// KeyLit is a key as written: Text holds a word or a decoded JSON string, Int an integer.
type KeyLit struct {
	Kind KeyLitKind
	Text string
	Int  int64
	Raw  string // the key's source text, which String writes back
}

// Seg is one segment: Name for SegField, Key for SegKey, Pos for SegPos.
type Seg struct {
	Kind SegKind
	Name string
	Key  KeyLit
	Pos  int
}

// Path is a parsed value path; Package is "" when the path has no `package:` prefix.
type Path struct {
	Package string
	Root    string
	Segs    []Seg
}

// segmentParsers dispatches a segment on its first byte.
var segmentParsers = map[byte]func(*parser) (Seg, error){
	fieldMark:   (*parser).field,
	bracketOpen: (*parser).bracket,
}

// parser reads one path left to right; i is the next byte.
type parser struct {
	s string
	i int
}

// Parse reads a value path (API.md §6.1); a syntax error wraps ErrBadPath.
func Parse(s string) (Path, error) {
	p := parser{s: s}
	var path Path
	if pkg, ok := packagePrefix(s); ok {
		path.Package = pkg
		p.i = len(pkg) + len(packageMark)
	}
	root, err := p.word()
	if err != nil {
		return Path{}, err
	}
	path.Root = root
	for p.i < len(s) {
		parse, ok := segmentParsers[s[p.i]]
		if !ok {
			return Path{}, p.fail(errUnexpected)
		}
		seg, err := parse(&p)
		if err != nil {
			return Path{}, err
		}
		path.Segs = append(path.Segs, seg)
	}
	return path, nil
}

// packagePrefix is the `word { "." word }` before a ":" that s starts with, if any.
func packagePrefix(s string) (string, bool) {
	for i := 0; ; {
		n := wordLen(s[i:])
		if n == 0 {
			return "", false
		}
		i += n
		switch {
		case strings.HasPrefix(s[i:], packageMark):
			return s[:i], true
		case i < len(s) && s[i] == fieldMark:
			i++
		default:
			return "", false
		}
	}
}

// wordLen is the length of the word s starts with, 0 for none; "_" alone is no word.
func wordLen(s string) int {
	n := 0
	for n < len(s) && (isLetter(s[n]) || n > 0 && isDigit(s[n])) {
		n++
	}
	if s[:n] == underscore {
		return 0
	}
	return n
}

func isLetter(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_' }

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

func (p *parser) fail(reason error) error {
	return fmt.Errorf("%w: %w at byte %d", ErrBadPath, reason, p.i)
}

func (p *parser) word() (string, error) {
	n := wordLen(p.s[p.i:])
	if n == 0 {
		return "", p.fail(errName)
	}
	p.i += n
	return p.s[p.i-n : p.i], nil
}

// field reads `.name`.
func (p *parser) field() (Seg, error) {
	p.i++
	name, err := p.word()
	return Seg{Kind: SegField, Name: name}, err
}

// bracket reads `[#n]` or `[key]`.
func (p *parser) bracket() (Seg, error) {
	p.i++
	var seg Seg
	var err error
	if strings.HasPrefix(p.s[p.i:], positionMark) {
		p.i += len(positionMark)
		seg.Kind = SegPos
		seg.Pos, err = p.position()
	} else {
		seg.Kind = SegKey
		seg.Key, err = p.key()
	}
	if err != nil {
		return Seg{}, err
	}
	if !strings.HasPrefix(p.s[p.i:], bracketClose) {
		return Seg{}, p.fail(errUnclosed)
	}
	p.i += len(bracketClose)
	return seg, nil
}

// position reads the digits of `[#n]`: no sign, no leading zero (API.md P4).
func (p *parser) position() (int, error) {
	digits, err := p.digits()
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, p.fail(errRange)
	}
	return n, nil
}

// key reads a word, an optionally negative integer or a JSON string.
func (p *parser) key() (KeyLit, error) {
	start := p.i
	rest := p.s[p.i:]
	switch {
	case strings.HasPrefix(rest, jsonQuote):
		text, n, err := diag.UnquoteJSON(rest)
		if err != nil {
			return KeyLit{}, p.fail(err)
		}
		p.i += n
		return KeyLit{Kind: KeyString, Text: text, Raw: rest[:n]}, nil
	case rest != "" && (rest[0] == minus || isDigit(rest[0])):
		if rest[0] == minus {
			p.i++
		}
		if _, err := p.digits(); err != nil {
			return KeyLit{}, err
		}
		n, err := strconv.ParseInt(p.s[start:p.i], decimalBase, int64Bits)
		if err != nil {
			return KeyLit{}, p.fail(errRange)
		}
		return KeyLit{Kind: KeyInt, Int: n, Raw: p.s[start:p.i]}, nil
	}
	w, err := p.word()
	return KeyLit{Kind: KeyWord, Text: w, Raw: w}, err
}

// digits reads `digit { digit }` with no leading zero but "0" itself.
func (p *parser) digits() (string, error) {
	n := 0
	for p.i+n < len(p.s) && isDigit(p.s[p.i+n]) {
		n++
	}
	d := p.s[p.i : p.i+n]
	if n == 0 || n > 1 && d[0] == zeroDigit {
		return "", p.fail(errDigits)
	}
	p.i += n
	return d, nil
}

// String writes the path as parsed; a key built without Raw is written in its canonical form.
func (p Path) String() string {
	var sb strings.Builder
	if p.Package != "" {
		sb.WriteString(p.Package + packageMark)
	}
	sb.WriteString(p.Root)
	for _, s := range p.Segs {
		switch s.Kind {
		case SegField:
			sb.WriteString(string(fieldMark) + s.Name)
		case SegKey:
			sb.WriteString(string(bracketOpen) + s.Key.text() + bracketClose)
		case SegPos:
			sb.WriteString(string(bracketOpen) + positionMark + strconv.Itoa(s.Pos) + bracketClose)
		}
	}
	return sb.String()
}

// text is the key's Raw or, for a key built without one, the text its Kind writes.
func (k KeyLit) text() string {
	switch {
	case k.Raw != "":
		return k.Raw
	case k.Kind == KeyInt:
		return strconv.FormatInt(k.Int, decimalBase)
	case k.Kind == KeyString:
		return string(diag.AppendJSONString(nil, k.Text))
	}
	return k.Text
}
