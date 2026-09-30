package load

import (
	"bytes"
	"context"
	"math"
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

// posOf is i as a source.Pos: files are already size-checked by FileSet.Add, but a reader
// built from an int offset still clamps rather than wrap.
func posOf(i int) source.Pos {
	if i < 0 {
		return 0
	}
	if i > math.MaxInt32 {
		return math.MaxInt32
	}
	return source.Pos(i)
}

// resolvePath resolves path relative to req.From, with no filesystem access (WIRE.md §2.3, §6.1).
func (l *Loader) resolvePath(path string, req Request) (project.Path, bool) {
	return l.Layout.Resolve(path, req.From, req.Span, req.Bag)
}

// statFile is p's existence check: a directory or non-regular file (a FIFO could hang) refuses too (WIRE.md §6.1).
func (l *Loader) statFile(p project.Path, req Request) bool {
	info, err := l.stat(p.Abs)
	switch {
	case err != nil:
		diag.E7004.At(req.Span, p.Display, causeOf(err)).Report(req.Bag)
		return false
	case info.IsDir():
		diag.E7004.At(req.Span, p.Display, diag.KindReadIsDir).Report(req.Bag)
		return false
	case !info.Mode().IsRegular():
		diag.E7004.At(req.Span, p.Display, diag.KindReadNotRegular).Report(req.Bag)
		return false
	}
	return true
}

// resolveFile resolves and stats path, the one file a single-file form reads, not a glob (WIRE.md §6.1).
func (l *Loader) resolveFile(path string, req Request) (project.Path, bool) {
	p, ok := l.resolvePath(path, req)
	if !ok || !l.statFile(p, req) {
		return project.Path{}, false
	}
	return p, true
}

// bare runs the plain `load(path)` form, decoded per its own format, path resolved first so E7007 names it (WIRE.md §6.1).
func (l *Loader) bare(ctx context.Context, req Request, e *syntax.LoadExpr, t types.Type) (value.Value, bool, error) {
	c, ok := parseCall(e)
	if !ok {
		return nil, false, notLiteral(loadForm)
	}
	p, ok := l.resolvePath(c.path, req)
	if !ok {
		return nil, false, nil
	}
	format, ok := resolveCallFormat(c, p.Display, req)
	if !ok {
		return nil, false, nil
	}
	if !checkOptions(loadForm, c, format, req) {
		return nil, false, nil
	}
	if !types.FormatFits(format, c.boolOpt(c.header), t) {
		diag.E7116.At(req.Span, formLabel(loadForm), t).Report(req.Bag)
		return nil, false, nil
	}
	if !l.statFile(p, req) {
		return nil, false, nil
	}
	switch format {
	case types.FormatJSON:
		return l.bareJSON(ctx, req, c, p, t)
	case types.FormatCSV:
		return l.bareCSV(ctx, req, c, p, t)
	default:
		return l.textValue(req, p, t)
	}
}

// bareJSON decodes p as JSON against t, applying `at:` first when given (WIRE.md §6.3).
func (l *Loader) bareJSON(ctx context.Context, req Request, c parsedCall, p project.Path, t types.Type) (value.Value, bool, error) {
	src, took, ok := l.takeSource(p.Display, p.Abs, req)
	if !ok {
		return nil, false, nil
	}
	root, err := l.parse(src, req.Bag)
	if err != nil {
		return nil, ReportEncoding(req.Bag, p.Display, took.raw(), err), nil
	}
	sel := wire.Selection{Node: root}
	if c.at != nil {
		if sel, ok = applyAt(root, *c.at, req); !ok {
			return nil, false, nil
		}
	}
	dec := req.decoder(c.boolOpt(c.partial))
	return dec.Decode(ctx, sel, t)
}

// bareCSV decodes p as CSV against t, `header:` choosing records or plain cells (WIRE.md §6.6).
func (l *Loader) bareCSV(ctx context.Context, req Request, c parsedCall, p project.Path, t types.Type) (value.Value, bool, error) {
	res, ok := l.readCSV(p, req)
	if !ok {
		return nil, false, nil
	}
	return decodeCSV(ctx, req, c, res, t)
}

// boolOpt is an option's value, false when it was not given.
func (c parsedCall) boolOpt(o *bool) bool { return o != nil && *o }

// textValue reads p as a String, or an alias or refinement of it (WIRE.md §6.7).
func (l *Loader) textValue(req Request, p project.Path, t types.Type) (value.Value, bool, error) {
	content, span, ok := l.readText(p, req)
	if !ok {
		return nil, false, nil
	}
	return &value.Str{V: content, T: t, P: &value.Prov{Kind: value.ProvText, Span: span}}, true, nil
}

// text runs `load.text(path)` (WIRE.md §6.7, LOD-07).
func (l *Loader) text(_ context.Context, req Request, e *syntax.LoadExpr, t types.Type) (value.Value, bool, error) {
	c, ok := parseCall(e)
	if !ok {
		return nil, false, notLiteral(methodText)
	}
	if !checkOptions(methodText, c, types.FormatText, req) {
		return nil, false, nil
	}
	p, ok := l.resolveFile(c.path, req)
	if !ok {
		return nil, false, nil
	}
	return l.textValue(req, p, t)
}

// readText reads p's bytes as a String, a BOM removed, nothing else trimmed (WIRE.md §6.7).
func (l *Loader) readText(p project.Path, req Request) (string, source.Span, bool) {
	src, data, ok := l.readSource(p.Display, p.Abs, req)
	if !ok {
		return "", source.Span{}, false
	}
	start, ok := checkUTF8(src, data, p.Display, req)
	if !ok {
		return "", source.Span{}, false
	}
	span := source.Span{File: src.ID, Start: posOf(start), End: posOf(len(src.Content))}
	return string(src.Content[start:]), span, true
}

// csv runs `load.csv(path[, header:][, partial:])` (WIRE.md §6.6, LOD-06).
func (l *Loader) csv(ctx context.Context, req Request, e *syntax.LoadExpr, t types.Type) (value.Value, bool, error) {
	c, ok := parseCall(e)
	if !ok {
		return nil, false, notLiteral(methodCSV)
	}
	if !checkOptions(methodCSV, c, types.FormatCSV, req) {
		return nil, false, nil
	}
	if !types.CSVFits(c.boolOpt(c.header), t) {
		diag.E7116.At(req.Span, formLabel(methodCSV), t).Report(req.Bag)
		return nil, false, nil
	}
	p, ok := l.resolveFile(c.path, req)
	if !ok {
		return nil, false, nil
	}
	res, ok := l.readCSV(p, req)
	if !ok {
		return nil, false, nil
	}
	return decodeCSV(ctx, req, c, res, t)
}

// csvRows is readCSV's output: a file's records, plus the FileSet id and folded size of the file
// its cells' spans are in.
type csvRows struct {
	rows [][]wire.Cell
	file source.FileID
	size int
}

// decodeCSV is header (c's first row) and rows as t; no record under header: true is noHeader at 1:1.
func decodeCSV(ctx context.Context, req Request, c parsedCall, res csvRows, t types.Type) (value.Value, bool, error) {
	hasHeader, rows := c.boolOpt(c.header), res.rows
	if hasHeader && len(rows) == 0 {
		diag.E7113.AtNoHeader(source.Span{File: res.file, End: posOf(min(1, res.size))}).Report(req.Bag)
		return nil, false, nil
	}
	var header []wire.Cell
	if hasHeader {
		header, rows = rows[0], rows[1:]
	}
	dec := req.decoder(c.boolOpt(c.partial))
	return dec.CSV(ctx, header, rows, t)
}

// readCSV reads p's raw bytes as RFC 4180 records, every cell located in the file (WIRE.md §6.6).
func (l *Loader) readCSV(p project.Path, req Request) (csvRows, bool) {
	src, data, ok := l.readSource(p.Display, p.Abs, req)
	if !ok {
		return csvRows{}, false
	}
	start, ok := checkUTF8(src, data, p.Display, req)
	if !ok {
		return csvRows{}, false
	}
	rows, ok := parseCSV(src, data, start, req)
	return csvRows{rows: rows, file: src.ID, size: len(src.Content)}, ok
}

// utf8Source is the file checkUTF8 and reportUTF8 share: its parsed content, its raw bytes
// (for E7105's offset) and its display path.
type utf8Source struct {
	src  *source.File
	data []byte
	path string
}

// checkUTF8 is src's UTF-8 validity, a leading UTF-8 BOM skipped, else E7105 (WIRE.md §3.1).
func checkUTF8(src *source.File, data []byte, path string, req Request) (int, bool) {
	fs := utf8Source{src: src, data: data, path: path}
	for _, bom := range jsonsrc.ForeignBOMs {
		if bytes.HasPrefix(src.Content, []byte(bom)) {
			return reportUTF8(fs, req, 0, len(bom))
		}
	}
	if i := jsonsrc.InvalidUTF8At(src.Content); i >= 0 {
		return reportUTF8(fs, req, i, i+1)
	}
	if bytes.HasPrefix(src.Content, []byte(jsonsrc.UTF8BOM)) {
		return len(jsonsrc.UTF8BOM), true
	}
	return 0, true
}

// reportUTF8 is E7105 at fs's raw file offset for [start,end) of its (folded) content.
func reportUTF8(fs utf8Source, req Request, start, end int) (int, bool) {
	sp := source.Span{File: fs.src.ID, Start: posOf(start), End: posOf(end)}
	diag.E7105.AtFile(sp, fs.path, rawOffset(fs.data, start)).Report(req.Bag)
	return 0, false
}

// csvScanner reads one file's raw RFC 4180 bytes into cells, a quoted cell's CR LF kept verbatim (WIRE.md §6.6).
type csvScanner struct {
	data   []byte
	pos    int
	file   source.FileID
	req    Request
	record int64 // 1-based
	fold   foldCursor
}

// parseCSV reads data from start as RFC 4180 records: LF or CR LF ends a record (WIRE.md §6.6).
func parseCSV(src *source.File, data []byte, start int, req Request) ([][]wire.Cell, bool) {
	sc := &csvScanner{data: data, pos: start, file: src.ID, req: req, fold: foldCursor{data: data}}
	var rows [][]wire.Cell
	width := -1
	for sc.pos < len(sc.data) {
		recordStart := sc.pos
		sc.record++
		row, ok := sc.readRecord()
		switch {
		case !ok:
			return nil, false
		case width < 0:
			width = len(row)
		case len(row) != width:
			sc.failFieldCount(recordStart, recordStart+1, len(row), width)
			return nil, false
		}
		rows = append(rows, row)
	}
	return rows, true
}

// readRecord reads one record: fields separated by ",", ending at "\n", "\r\n" or EOF, each cell's Row and Col set for its provenance pointer (WIRE.md §6.6).
func (sc *csvScanner) readRecord() ([]wire.Cell, bool) {
	var row []wire.Cell
	for {
		cell, ok := sc.readField()
		if !ok {
			return nil, false
		}
		cell.Row, cell.Col = sc.record, int64(len(row)+1)
		row = append(row, cell)
		switch {
		case sc.pos >= len(sc.data):
			return row, true
		case sc.data[sc.pos] == '\n':
			sc.pos++
			return row, true
		case sc.data[sc.pos] == '\r':
			sc.pos += twoBytes // a record only ever stops at a validated "\r\n" (readUnquoted, readQuoted)
			return row, true
		default:
			sc.pos++ // the "," between fields
		}
	}
}

func (sc *csvScanner) readField() (wire.Cell, bool) {
	if sc.pos < len(sc.data) && sc.data[sc.pos] == '"' {
		return sc.readQuoted()
	}
	return sc.readUnquoted()
}

// fieldEnds is whether a field stops at data[i]: ",", "\n" or "\r\n", never a lone CR (WIRE.md §6.6).
func (sc *csvScanner) fieldEnds(i int) bool {
	switch sc.data[i] {
	case ',', '\n':
		return true
	case '\r':
		return i+1 < len(sc.data) && sc.data[i+1] == '\n'
	default:
		return false
	}
}

// readUnquoted reads a field with no quote and no CR: E7113 bareQuote or bareCR otherwise (DECISIONS 218).
func (sc *csvScanner) readUnquoted() (wire.Cell, bool) {
	start, i := sc.pos, sc.pos
	for ; i < len(sc.data) && !sc.fieldEnds(i); i++ {
		switch sc.data[i] {
		case '"':
			sc.failBareQuote(i, i+1)
			return wire.Cell{}, false
		case '\r':
			sc.failBareCR(i, i+1)
			return wire.Cell{}, false
		}
	}
	return sc.unquotedCell(start, i)
}

func (sc *csvScanner) unquotedCell(start, end int) (wire.Cell, bool) {
	sc.pos = end
	return wire.Cell{Text: string(sc.data[start:end]), Span: sc.span(start, end)}, true
}

// readQuoted reads a `"`-quoted field, `""` an escaped quote, CR and LF kept verbatim (WIRE.md §6.6).
func (sc *csvScanner) readQuoted() (wire.Cell, bool) {
	start := sc.pos
	var b strings.Builder
	i, closing := start+1, false
	for !closing {
		next, atClose, ok := sc.quotedByte(&b, i)
		if !ok {
			sc.failUnclosed(start, start+1)
			return wire.Cell{}, false
		}
		i, closing = next, atClose
	}
	if i < len(sc.data) && !sc.fieldEnds(i) { // a lone CR too: the record would lose its next byte
		sc.failAfterQuote(i, i+1)
		return wire.Cell{}, false
	}
	sc.pos = i
	return wire.Cell{Text: b.String(), Span: sc.span(start, i)}, true
}

// quotedByte handles data[i] inside a quoted field: the next index to read from, whether it
// closed the field, false past EOF with no closing quote.
func (sc *csvScanner) quotedByte(b *strings.Builder, i int) (int, bool, bool) {
	if i >= len(sc.data) {
		return 0, false, false
	}
	if sc.data[i] != '"' {
		b.WriteByte(sc.data[i])
		return i + 1, false, true
	}
	if i+1 < len(sc.data) && sc.data[i+1] == '"' {
		b.WriteByte('"')
		return i + twoBytes, false, true
	}
	return i + 1, true, true
}

// span is [a,b) of the raw bytes, translated to the FileSet's CRLF-folded content coordinates.
func (sc *csvScanner) span(a, b int) source.Span {
	start := sc.fold.at(a)
	return source.Span{File: sc.file, Start: posOf(start), End: posOf(sc.fold.at(b))}
}

func (sc *csvScanner) failBareQuote(a, b int) {
	diag.E7113.AtBareQuote(sc.span(a, b), sc.record).Report(sc.req.Bag)
}

func (sc *csvScanner) failUnclosed(a, b int) {
	diag.E7113.AtUnclosed(sc.span(a, b), sc.record).Report(sc.req.Bag)
}

func (sc *csvScanner) failAfterQuote(a, b int) {
	diag.E7113.AtAfterQuote(sc.span(a, b), sc.record).Report(sc.req.Bag)
}

func (sc *csvScanner) failFieldCount(a, b, got, want int) {
	diag.E7113.AtFieldCount(sc.span(a, b), sc.record, int64(got), int64(want)).Report(sc.req.Bag)
}

func (sc *csvScanner) failBareCR(a, b int) {
	diag.E7113.AtBareCR(sc.span(a, b), sc.record).Report(sc.req.Bag)
}
