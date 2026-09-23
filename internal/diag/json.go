package diag

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/source"
)

// jsonWriter builds one line of JSON with the key order the caller writes (API.md F5).
type jsonWriter struct {
	buf []byte
}

// jsonForm is one object per finding, one per line, then the summary object (CLI.md §2.4).
func jsonForm(findings []Located, opt RenderOptions) string {
	var w jsonWriter
	for i := range findings {
		w.finding(&findings[i])
		w.buf = append(w.buf, lineBreak...)
	}
	w.summary(opt)
	w.buf = append(w.buf, lineBreak...)
	return string(w.buf)
}

// AppendJSON appends the finding's one-line JSON object (API.md F5), without a line break:
// the one writer of the form, which canon.Finding.MarshalJSON uses too.
func (l *Located) AppendJSON(buf []byte) []byte {
	w := jsonWriter{buf: buf}
	w.finding(l)
	return w.buf
}

// finding writes the keys of F5 in order, omitting the empty optional ones.
func (w *jsonWriter) finding(f *Located) {
	w.open(objOpen)
	w.str(keySeverity, f.Severity.String())
	w.str(keyCode, string(f.Code))
	w.loc(f.Loc)
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
			w.loc(r.Loc)
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
			w.loc(fr.Loc)
			w.close(objClose)
		}
		w.close(arrClose)
	}
	if f.MoreFrames > 0 {
		w.num(keyMoreFrames, f.MoreFrames)
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

// loc writes file, line, col, endLine and endCol, or nothing without a file (API.md F5).
func (w *jsonWriter) loc(l source.Location) {
	if l.Path == "" {
		return
	}
	w.str(keyFile, l.Path)
	w.num(keyLine, l.Line)
	w.num(keyCol, l.Col)
	w.num(keyEndLine, l.EndLine)
	w.num(keyEndCol, l.EndCol)
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
