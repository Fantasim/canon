package lock

import (
	"bytes"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// reader reads one canon.lock line by line.
type reader struct {
	id        source.FileID
	bag       *diag.Bag
	file      *File
	batch     []Fact
	pkgSegs   []string
	sawHeader bool
	stop      bool
	ok        bool
}

// token is one field of a line; a JSON string keeps its decoded value.
type token struct {
	text   string
	quoted bool
	value  string
}

// shape is what follows the kind on a line of that kind.
type shape struct {
	kind     Kind
	fields   int
	value    bool
	retiring bool
}

// problem is why a line of a known kind is refused.
type problem uint8

// Parse reads pkg's canon.lock held by file id (LOCK.md §2.4); ok is false once it reported E6005.
func Parse(id source.FileID, data []byte, pkg string, bag *diag.Bag) (*File, bool) {
	r := reader{id: id, bag: bag, file: New(pkg), pkgSegs: strings.Split(pkg, nameSep), ok: true}
	for start := 0; start < len(data) && !r.stop; {
		end := bytes.IndexByte(data[start:], lineBreak[0])
		if end < 0 {
			end = len(data) - start
		}
		r.line(start, strings.TrimSuffix(string(data[start:start+end]), carriageReturn))
		start += end + 1
	}
	if !r.sawHeader {
		r.report(diag.E6005.AtHeader(source.Span{File: id}))
	}
	r.file.mergeAll(r.batch)
	return r.file, r.ok
}

func (r *reader) report(b *diag.Builder) {
	b.Report(r.bag)
	r.ok = false
}

// line reads one line, CR LF already read as LF; a line of spaces is blank and ignored.
func (r *reader) line(start int, text string) {
	span := source.Span{File: r.id, Start: pos(start), End: pos(start + len(text))}
	switch {
	case strings.Trim(text, space) == "":
	case isMergeMarker(text):
		r.report(diag.E6005.AtMerge(span))
	case !r.sawHeader:
		r.header(span, text)
	default:
		r.fact(span, text)
	}
}

// pos is a byte offset as a source.Pos; a file set holds no file past MaxInt32 bytes.
func pos(n int) source.Pos {
	if n < 0 || n > math.MaxInt32 {
		return math.MaxInt32
	}
	return source.Pos(n)
}

func isMergeMarker(text string) bool {
	return slices.ContainsFunc(mergeMarkers[:], func(m string) bool { return strings.HasPrefix(text, m) })
}

// header reads the first non-blank line; a newer version stops the reading (LOCK.md §10).
func (r *reader) header(span source.Span, text string) {
	r.sawHeader = true
	if text == header {
		return
	}
	version, found := strings.CutPrefix(text, versionPrefix)
	if found && isNumeral(version) && version != zeroDigit && version != supportedVersion {
		r.report(diag.E6005.AtVersion(span, version))
		r.stop = true
		return
	}
	r.report(diag.E6005.AtHeader(span))
}

// fact reads one fact line and adds it to the set.
func (r *reader) fact(span source.Span, text string) {
	fields, ok := split(text)
	if !ok {
		r.report(diag.E6005.AtSyntax(span))
		return
	}
	sh, known := shapes[fields[0].text]
	if !known || fields[0].quoted {
		r.report(diag.E6005.AtKind(span, fields[0].text))
		return
	}
	fact, pr := r.parseFact(sh, fields[1:])
	switch pr {
	case problemSyntax:
		r.report(diag.E6005.AtSyntax(span))
	case problemPackage:
		r.report(diag.E6005.AtPackage(span, fields[1].text, r.file.Package))
	case problemNone:
		fact.Span = span
		r.batch = append(r.batch, fact)
	}
}

// parseFact reads name, value when the kind has one, holder, then `retired` if it retires.
func (r *reader) parseFact(sh shape, fields []token) (Fact, problem) {
	n := len(fields)
	retired := sh.retiring && n == sh.fields+1 && !fields[n-1].quoted && fields[n-1].text == retiredWord
	if retired {
		n--
	}
	if n != sh.fields || fields[0].quoted || fields[n-1].quoted || !isIdentifier(fields[n-1].text) {
		return Fact{}, problemSyntax
	}
	fact := Fact{Kind: sh.kind, Holder: fields[n-1].text, Retired: retired}
	if sh.value {
		v, ok := parseValue(fields[1], sh.kind == KindField)
		if !ok {
			return Fact{}, problemSyntax
		}
		fact.Value = v
	}
	var pr problem
	fact.Name, fact.Field, pr = r.name(fields[0].text, sh.kind)
	return fact, pr
}

// name splits a qualified name into the table's or enum's name and a field fact's field; a
// name of another package, or with a segment too many or too few, is not in this package.
func (r *reader) name(text string, kind Kind) (string, string, problem) {
	segs := strings.Split(text, nameSep)
	if slices.ContainsFunc(segs, func(s string) bool { return !isIdentifier(s) }) {
		return "", "", problemSyntax
	}
	own := len(r.pkgSegs)
	want := own + 1
	if kind == KindField {
		want++
	}
	if len(segs) != want || !slices.Equal(segs[:own], r.pkgSegs) {
		return "", "", problemPackage
	}
	field := ""
	if kind == KindField {
		field = segs[want-1]
	}
	return strings.Join(segs[:own+1], nameSep), field, problemNone
}

// parseValue reads an integer, or a JSON string where strings are allowed.
func parseValue(t token, allowString bool) (Value, bool) {
	if t.quoted {
		return Value{IsString: true, Str: t.value}, allowString
	}
	if !isNumeral(strings.TrimPrefix(t.text, minusSign)) {
		return Value{}, false
	}
	n, err := strconv.ParseInt(t.text, decimalBase, int64Bits)
	return Value{Int: n}, err == nil
}

// split cuts a line into fields at runs of spaces, a JSON string being one field; a leading
// or trailing space, or a string glued to what follows, fails.
func split(text string) ([]token, bool) {
	var out []token
	for i := 0; ; {
		t, n, ok := readToken(text[i:])
		if !ok {
			return nil, false
		}
		out = append(out, t)
		i += n
		if i == len(text) {
			return out, true
		}
		gap := len(text[i:]) - len(strings.TrimLeft(text[i:], space))
		if gap == 0 || i+gap == len(text) {
			return nil, false
		}
		i += gap
	}
}

// readToken reads the field s starts with and its width.
func readToken(s string) (token, int, bool) {
	if strings.HasPrefix(s, jsonQuote) {
		v, n, err := diag.UnquoteJSON(s)
		return token{text: s[:n], quoted: true, value: v}, n, err == nil
	}
	n := strings.Index(s, space)
	if n < 0 {
		n = len(s)
	}
	return token{text: s[:n]}, n, n > 0
}

// isIdentifier is an identifier of SPEC §2.4.
func isIdentifier(s string) bool {
	if s == "" || s == underscore || strings.IndexByte(digitChars, s[0]) >= 0 {
		return false
	}
	return strings.Trim(s, identChars) == ""
}

// isNumeral is decimal digits with no leading zero but "0" itself.
func isNumeral(s string) bool {
	if s == "" || len(s) > 1 && s[0] == zeroDigit[0] {
		return false
	}
	return strings.Trim(s, digitChars) == ""
}
