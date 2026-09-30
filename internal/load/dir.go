package load

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// dir runs `load.dir(pattern[, at:][, partial:][, format:])` of JSON files against t (WIRE.md §6.5).
func (l *Loader) dir(ctx context.Context, req Request, e *syntax.LoadExpr, t types.Type) (value.Value, bool, error) {
	c, ok := parseCall(e)
	if !ok {
		return nil, false, unsupported(causeDirOption)
	}
	forcedJSON, err := dirForcedJSON(c)
	if err != nil {
		return nil, false, err
	}
	if !checkOptions(methodDir, c, types.FormatJSON, req) {
		return nil, false, nil
	}
	matches, ok := l.match(c.path, req)
	if !ok {
		return nil, false, nil
	}
	if len(matches) == 0 {
		req.report(diag.W7107.At(req.Span, c.path))
	}
	if ok, err := l.checkFormats(matches, forcedJSON, req); err != nil || !ok {
		return nil, false, err
	}
	files, readOK := l.readFiles(matches, c.at, req)
	dec := req.decoder(c.boolOpt(c.partial))
	dec.Elems = dirElems(req, t)
	v, decOK, err := dec.Dir(ctx, files, t)
	if err != nil {
		return nil, false, err
	}
	return v, readOK && decOK, nil
}

// dirForcedJSON is whether c.format: names json; any other symbol is ErrUnsupported (WIRE.md §6.5).
func dirForcedJSON(c parsedCall) (bool, error) {
	if c.format == nil {
		return false, nil
	}
	if types.FormatNamed(*c.format) == types.FormatJSON {
		return true, nil
	}
	return false, unsupported(causeDirFormat)
}

// match resolves pattern to its matched files, in path order; ok is false after a finding (WIRE.md §6.5).
func (l *Loader) match(pattern string, req Request) ([]matchFile, bool) {
	base, rest, ok := l.globBase(pattern, req.From, req.Span, req.Bag)
	if !ok {
		return nil, false
	}
	chk := validateGlob(rest)
	switch chk.verdict {
	case globWellFormed:
	case globEmptySeg:
		diag.E7001.AtEmpty(req.Span, pattern).Report(req.Bag)
		return nil, false
	case globDotSeg:
		diag.E7001.AtDot(req.Span, pattern, chk.seg).Report(req.Bag)
		return nil, false
	case globInvalid:
		diag.E7005.At(req.Span, pattern, chk.kind).Report(req.Bag)
		return nil, false
	}
	var matches []matchFile
	if !chk.dirOnly {
		var err error
		if matches, err = l.globMatches(base, rest, req); err != nil {
			diag.E7004.At(req.Span, base.Display, causeOf(err)).Report(req.Bag)
			return nil, false
		}
	}
	l.globbed(path.Join(base.Display, rest), matches)
	return matches, true
}

// causeOf is E7004's fixed Kind for err, chosen by errors.Is, never the OS message or a path (ERRORS.md §1.4, WIRE.md §6.1, §6.5).
func causeOf(err error) diag.Kind {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return diag.KindReadMissing
	case errors.Is(err, project.ErrSymlinkLoop):
		return diag.KindReadLinkLoop
	case errors.Is(err, fs.ErrPermission):
		return diag.KindReadPermission
	case errors.Is(err, errNotDir):
		return diag.KindReadNotDir
	case errors.Is(err, source.ErrFileTooLarge):
		return diag.KindReadTooLarge
	default:
		return diag.KindReadUnreadable
	}
}

// checkFormats refuses a csv or text match first (ErrUnsupported, no finding before it), then
// reports E7007 for every unrecognized extension; the extension is the match's own, not its
// link target's. forcedJSON (an explicit `format: json`) skips both: every match reads as json.
func (l *Loader) checkFormats(matches []matchFile, forcedJSON bool, req Request) (bool, error) {
	if forcedJSON {
		return true, nil
	}
	for _, m := range matches {
		if f := types.FormatOfPath(m.Display); f == types.FormatCSV || f == types.FormatText {
			return false, unsupported(causeDirFileFormat)
		}
	}
	ok := true
	for _, m := range matches {
		if types.FormatOfPath(m.Display) == types.FormatUnknown {
			diag.E7007.At(req.Span, m.Display).Report(req.Bag)
			ok = false
		}
	}
	return ok, nil
}

// readFiles parses every match, at applied to each when given (WIRE.md §6.5, §6.3).
func (l *Loader) readFiles(matches []matchFile, at *string, req Request) ([]wire.File, bool) {
	ok := true
	files := make([]wire.File, 0, len(matches))
	for _, m := range matches {
		f, fOK := l.readFile(m, at, req)
		if f != nil {
			files = append(files, *f)
		}
		ok = ok && fOK
	}
	return files, ok
}

// readFile reads, then parses one matched file into wire's per-file selection.
func (l *Loader) readFile(m matchFile, at *string, req Request) (*wire.File, bool) {
	src, took, ok := l.takeSource(m.Display, m.Abs, req)
	if !ok {
		return nil, false
	}
	root, err := l.parse(src, req.Bag)
	if err != nil {
		return nil, ReportEncoding(req.Bag, m.Display, took.raw(), err)
	}
	sel := wire.Selection{Node: root}
	if at != nil {
		if sel, ok = applyAt(root, *at, req); !ok {
			return nil, false
		}
	}
	return &wire.File{Sel: sel, Stem: stem(m.Display), At: root.Span}, true
}

// ReportEncoding is E7105 for a jsonsrc encoding error, data's raw bytes giving the file
// variant's {offset} a byte of the file as written, not of the folded content its span is in.
func ReportEncoding(bag *diag.Bag, path string, data []byte, err error) bool {
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
			raw += twoBytes
		} else {
			raw++
		}
		n++
	}
	return int64(raw)
}

// foldCursor is rawOffset's inverse: data's raw offsets in the FileSet's CRLF-folded content.
// It resumes from its last answer, so a scanner asking in order walks the file once, not once per cell.
type foldCursor struct {
	data  []byte
	next  int // every "\r\n" starting before next is counted in pairs
	last  int // the last offset asked, raw
	pairs int
}

// at is raw's folded offset: raw less the "\r\n" pairs that end at or before it.
func (c *foldCursor) at(raw int) int {
	raw = min(max(raw, 0), len(c.data))
	if raw < c.last {
		c.next, c.pairs = 0, 0
	}
	c.last = raw
	for ; c.next+1 < raw; c.next++ {
		if c.data[c.next] == '\r' && c.data[c.next+1] == '\n' {
			c.pairs++
		}
	}
	return raw - c.pairs
}

// stem is a load.dir file's table key: its name with no extension (WIRE.md §6.5).
func stem(display string) string {
	base := path.Base(display)
	return strings.TrimSuffix(base, path.Ext(base))
}
