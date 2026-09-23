package diag

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/source"
)

// jsonWriter builds one line of JSON with the key order the caller writes (API.md F5).
type jsonWriter struct {
	buf   []byte
	files Files
}

// jsonForm is one object per finding, one per line, then the summary object (CLI.md §2.4).
func jsonForm(files Files, keyed []keyedFinding, opt RenderOptions) string {
	w := jsonWriter{files: files}
	for _, k := range keyed {
		w.finding(k.f)
		w.buf = append(w.buf, lineBreak...)
	}
	w.summary(opt)
	w.buf = append(w.buf, lineBreak...)
	return string(w.buf)
}

// finding writes the keys of F5 in order, omitting the empty optional ones.
func (w *jsonWriter) finding(f Finding) {
	w.open(objOpen)
	w.str(keySeverity, f.Severity.String())
	w.str(keyCode, string(f.Code))
	w.span(f.Span)
	w.optStr(keyPointer, f.Pointer)
	w.str(keyPackage, f.Package)
	w.optStr(keyPath, f.Path)
	w.str(keyMessage, f.Message)
	w.optStr(checkWord, f.Check)
	w.optStr(keyLayer, f.Layer)
	if len(f.Related) > 0 {
		w.key(keyRelated)
		w.open(arrOpen)
		for _, r := range f.Related {
			w.open(objOpen)
			w.span(r.Span)
			w.str(keyNote, r.Note)
			w.close(objClose)
		}
		w.close(arrClose)
	}
	if len(f.Stack) > 0 {
		w.key(keyStack)
		w.open(arrOpen)
		for _, fr := range f.Stack {
			w.open(objOpen)
			w.str(keyFn, fr.Fn)
			w.span(fr.Span)
			w.close(objClose)
		}
		w.close(arrClose)
	}
	if len(f.Reads) > 0 {
		w.key(keyReads)
		w.open(arrOpen)
		for _, r := range f.Reads {
			w.elem()
			w.buf = AppendJSONString(w.buf, r)
		}
		w.close(arrClose)
	}
	w.close(objClose)
}

// span writes file, line, col, endLine and endCol, or nothing without a file (API.md F5).
func (w *jsonWriter) span(s source.Span) {
	path := w.files.Path(s.File)
	if path == "" {
		return
	}
	line, col := w.files.Position(s.File, s.Start)
	endLine, endCol := w.files.Position(s.File, s.End)
	w.str(keyFile, path)
	w.num(keyLine, line)
	w.num(keyCol, col)
	w.num(keyEndLine, endLine)
	w.num(keyEndCol, endCol)
}

// summary writes {"summary":{…}}, with truncated only when findings were dropped.
func (w *jsonWriter) summary(opt RenderOptions) {
	s := opt.Summary
	w.open(objOpen)
	w.key(keySummary)
	w.open(objOpen)
	w.num(errorsWord, s.Errors)
	w.num(warningsWord, s.Warnings)
	w.num(packagesWord, s.Packages)
	w.num(keyMillis, int(max(opt.Duration, 0).Milliseconds()))
	if len(s.Truncated) > 0 {
		w.key(keyTrunc)
		w.open(arrOpen)
		for _, t := range s.Truncated {
			w.open(objOpen)
			w.str(keyPackage, t.Package)
			w.num(errorsWord, t.Errors)
			w.num(warningsWord, t.Warnings)
			w.close(objClose)
		}
		w.close(arrClose)
	}
	w.close(objClose)
	w.close(objClose)
}

// open starts an object or array, as an element of the enclosing array when there is one.
func (w *jsonWriter) open(c byte) {
	w.elem()
	w.buf = append(w.buf, c)
}

func (w *jsonWriter) close(c byte) {
	w.buf = append(w.buf, c)
}

// elem writes the comma that separates an element or member from the previous one.
func (w *jsonWriter) elem() {
	if n := len(w.buf); n > 0 && w.buf[n-1] != objOpen && w.buf[n-1] != arrOpen && w.buf[n-1] != colon && w.buf[n-1] != lineBreak[0] {
		w.buf = append(w.buf, comma)
	}
}

func (w *jsonWriter) key(k string) {
	w.elem()
	w.buf = AppendJSONString(w.buf, k)
	w.buf = append(w.buf, colon)
}

func (w *jsonWriter) str(k, v string) {
	w.key(k)
	w.buf = AppendJSONString(w.buf, v)
}

func (w *jsonWriter) optStr(k, v string) {
	if v != "" {
		w.str(k, v)
	}
}

func (w *jsonWriter) num(k string, n int) {
	w.key(k)
	w.buf = strconv.AppendInt(w.buf, int64(n), decimalBase)
}
