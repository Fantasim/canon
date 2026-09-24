package load

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// dir runs `load.dir(pattern)` of JSON files against t (WIRE.md §6.5).
func (l *Loader) dir(ctx context.Context, req Request, e *syntax.LoadExpr, t types.Type) (value.Value, bool, error) {
	pattern, ok := dirPattern(e)
	if !ok {
		return nil, false, unsupported("a load.dir option, this milestone reads only its one path")
	}
	if !supported(t) {
		return nil, false, unsupported("an element type this milestone cannot decode without the evaluator")
	}
	matches, ok := l.match(pattern, req)
	if !ok {
		return nil, false, nil
	}
	if len(matches) == 0 {
		diag.W7107.At(req.Span, pattern).Report(req.Bag)
	}
	if ok, err := l.checkFormats(matches, req); err != nil || !ok {
		return nil, false, err
	}
	files, readOK := l.readFiles(matches, req)
	host := wireHost{span: req.Span}
	v, decOK, err := (&wire.Decoder{Bag: req.Bag, Pkg: req.Pkg, Host: host}).Dir(ctx, files, t)
	if err != nil {
		return nil, false, err
	}
	return v, readOK && decOK, nil
}

// match resolves pattern to its matched files, in path order; ok is false after a finding (WIRE.md §6.5).
func (l *Loader) match(pattern string, req Request) ([]matchFile, bool) {
	base, rest, ok := l.globBase(pattern, req.From, req.Span, req.Bag)
	if !ok {
		return nil, false
	}
	chk := validateGlob(rest)
	switch {
	case chk.e7001:
		diag.E7001.AtEmpty(req.Span, pattern).Report(req.Bag)
		return nil, false
	case chk.e7005Cause != "":
		diag.E7005.At(req.Span, pattern, chk.e7005Cause).Report(req.Bag)
		return nil, false
	case chk.dirOnly:
		return nil, true
	}
	matches, err := l.globMatches(base, rest, req)
	if err != nil {
		diag.E7004.At(req.Span, base.Display, causeOf(err)).Report(req.Bag)
		return nil, false
	}
	return matches, true
}

// causeOf is E7004's fixed cause for err, chosen by errors.Is, never the OS message or a path.
func causeOf(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return causeMissing
	case errors.Is(err, fs.ErrPermission):
		return causePermission
	case errors.Is(err, errNotDir):
		return causeNotDir
	case errors.Is(err, source.ErrFileTooLarge):
		return causeTooLarge
	default:
		return causeUnreadable
	}
}

// checkFormats refuses a csv or text match first (ErrUnsupported, no finding before it), then
// reports E7007 for every unrecognized extension; the extension is the match's own, not its
// link target's.
func (l *Loader) checkFormats(matches []matchFile, req Request) (bool, error) {
	for _, m := range matches {
		if f := formatOf(m.Display); f == fmtCSV || f == fmtText {
			return false, unsupported("a load.dir file whose format is not json")
		}
	}
	ok := true
	for _, m := range matches {
		if formatOf(m.Display) == fmtUnknown {
			diag.E7007.At(req.Span, m.Display).Report(req.Bag)
			ok = false
		}
	}
	return ok, nil
}

// formatOf is name's format from its extension, compared ASCII case-insensitively (WIRE.md §6.2).
func formatOf(name string) wireFormat {
	switch ext := path.Ext(name); {
	case equalFoldASCII(ext, ".json"):
		return fmtJSON
	case equalFoldASCII(ext, ".csv"):
		return fmtCSV
	case equalFoldASCII(ext, ".txt"):
		return fmtText
	default:
		return fmtUnknown
	}
}

// equalFoldASCII is whether a and b are equal, ASCII letter case ignored only (WIRE.md §6.2).
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if lowerASCII(a[i]) != lowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// readFiles reads and parses every match into wire's per-file selection; ok is false after any read or JSON error (WIRE.md §6.5).
func (l *Loader) readFiles(matches []matchFile, req Request) ([]wire.File, bool) {
	ok := true
	files := make([]wire.File, 0, len(matches))
	for _, m := range matches {
		f, fOK := l.readFile(m, req)
		if f != nil {
			files = append(files, *f)
		}
		ok = ok && fOK
	}
	return files, ok
}

// readFile reads, then parses one matched file into wire's per-file selection.
func (l *Loader) readFile(m matchFile, req Request) (*wire.File, bool) {
	data, err := l.FS.ReadFile(m.Abs)
	if err != nil {
		diag.E7004.At(req.Span, m.Display, causeOf(err)).Report(req.Bag)
		return nil, false
	}
	src, err := l.Set.Add(m.Display, m.Abs, data)
	if err != nil {
		diag.E7004.At(req.Span, m.Display, causeOf(err)).Report(req.Bag)
		return nil, false
	}
	root, err := jsonsrc.Parse(src, req.Bag)
	if err != nil {
		return nil, reportEncoding(req.Bag, m.Display, data, err)
	}
	return &wire.File{Sel: wire.Selection{Node: root}, Stem: stem(m.Display), At: root.Span}, true
}

// reportEncoding is E7105 for a jsonsrc encoding error, data's raw bytes giving the file
// variant's {offset} a byte of the file as written, not of the folded content its span is in.
func reportEncoding(bag *diag.Bag, path string, data []byte, err error) bool {
	var enc *jsonsrc.EncodingError
	if errors.As(err, &enc) {
		if enc.Surrogate != 0 {
			diag.E7105.AtSurrogate(enc.Span, enc.Surrogate).Report(bag)
		} else {
			diag.E7105.AtFile(enc.Span, path, rawOffset(data, int(enc.Span.Start))).Report(bag)
		}
	}
	return false
}

// rawOffset is data's raw byte position matching normalized, a position in its CRLF-folded content.
func rawOffset(data []byte, normalized int) int64 {
	raw, n := 0, 0
	for n < normalized && raw < len(data) {
		if data[raw] == '\r' && raw+1 < len(data) && data[raw+1] == '\n' {
			raw += crlfLen
		} else {
			raw++
		}
		n++
	}
	return int64(raw)
}

// stem is a load.dir file's table key: its name with no extension (WIRE.md §6.5).
func stem(display string) string {
	base := path.Base(display)
	return strings.TrimSuffix(base, path.Ext(base))
}

// dirPattern is load.dir's one positional argument, a plain literal path; its options wait for M3.
func dirPattern(e *syntax.LoadExpr) (string, bool) {
	if len(e.Args) != 1 || e.Args[0].Name != nil {
		return "", false
	}
	s, ok := e.Args[0].Value.(*syntax.StringLit)
	if !ok {
		return "", false
	}
	return plainString(s)
}
