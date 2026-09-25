package load_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// A large CR LF file with quoted CR LF cells; linearTimeBound is far above a linear read, even
// under -race, and far below a quadratic one (minutes).
const (
	csvBigRecords   = 100_000
	csvBigRecord    = "abc,\"d\r\ne\"\r\n"
	linearTimeBound = 10 * time.Second
)

// WIRE.md §6.1: a single-file form's path naming a directory is E7004 ReadIsDir.
func TestBareLoadOnDirectoryIsE7004ReadIsDir(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"data.json/a.json": "{}"})
	_, ok, err := l.Load(context.Background(), req, bareExpr("data.json"), types.StringType)
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a refused decode", ok, err)
	}
	fd := req.Bag.Findings()
	if len(fd) != 1 || fd[0].Code != diag.E7004.Def().Code || !strings.Contains(fd[0].Message, "is a directory") {
		t.Errorf(`findings = %+v, want one %s naming "is a directory"`, fd, diag.E7004.Def().Code)
	}
}

func textExpr(path string) *syntax.LoadExpr {
	return &syntax.LoadExpr{
		Method: &syntax.Ident{Name: "text"},
		Args:   []*syntax.Arg{{Value: &syntax.StringLit{Parts: []syntax.StringPart{{Text: path}}}}},
	}
}

func headerOpt(v bool) *syntax.Arg {
	return &syntax.Arg{Name: &syntax.Ident{Name: "header"}, Value: &syntax.BoolLit{Value: v}}
}

// WIRE.md §6.6: a CRLF inside a quoted cell stays verbatim, and "\r\n" still ends a record.
func TestCSVQuotedCellKeepsCRLFVerbatim(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.csv": "a,\"x\r\ny\"\r\nc,d\r\n"})
	v, ok, err := l.Load(context.Background(), req, csvExpr("a.csv"), rowsType())
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v findings=%+v", ok, err, req.Bag.Findings())
	}
	rows := v.(*value.List)
	row0 := rows.Elems[0].(*value.List).Elems
	if row0[0].(*value.Str).V != "a" || row0[1].(*value.Str).V != "x\r\ny" {
		t.Errorf("row0 = %q %q", row0[0].(*value.Str).V, row0[1].(*value.Str).V)
	}
	row1 := rows.Elems[1].(*value.List).Elems
	if row1[0].(*value.Str).V != "c" || row1[1].(*value.Str).V != "d" {
		t.Errorf("row1 = %q %q", row1[0].(*value.Str).V, row1[1].(*value.Str).V)
	}
}

// WIRE.md §6.6: a lone CR inside an unquoted field is E7113 bareCR, wherever it sits.
func TestCSVUnquotedLoneCRIsBareCR(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.csv": "a,b\r\nb\rc,d\r\n"})
	_, ok, err := l.Load(context.Background(), req, csvExpr("a.csv"), rowsType())
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a refused decode", ok, err)
	}
	if len(req.Bag.Findings()) != 1 || req.Bag.Findings()[0].Code != diag.E7113.Def().Code {
		t.Errorf("findings = %+v, want one %s", req.Bag.Findings(), diag.E7113.Def().Code)
	}
}

// WIRE.md §6.6: a lone CR as the file's very last byte is E7113 bareCR too, an incomplete field.
func TestCSVBareCRAtEOF(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.csv": "a,b\r"})
	_, ok, err := l.Load(context.Background(), req, csvExpr("a.csv"), rowsType())
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a refused decode", ok, err)
	}
	if len(req.Bag.Findings()) != 1 || req.Bag.Findings()[0].Code != diag.E7113.Def().Code {
		t.Errorf("findings = %+v, want one %s", req.Bag.Findings(), diag.E7113.Def().Code)
	}
}

// WIRE.md §6.6: `header: true` on a file with no record at all is E7113 noHeader, at 1:1.
func TestCSVNoHeaderOnEmptyFile(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.csv": ""})
	e := csvExpr("a.csv")
	e.Args = append(e.Args, headerOpt(true))
	_, ok, err := l.Load(context.Background(), req, e, headerRowsType())
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a refused decode", ok, err)
	}
	fd := req.Bag.Findings()
	if len(fd) != 1 || fd[0].Code != diag.E7113.Def().Code || fd[0].Span.Start != 0 {
		t.Errorf("findings = %+v, want one %s at the file's first byte", fd, diag.E7113.Def().Code)
	}
}

// WIRE.md §6.1, §6.6: internal/check cannot see a bare load's runtime-detected format.
func TestBareLoadFormatMismatchIsE7116(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.txt": "hello"})
	_, ok, err := l.Load(context.Background(), req, bareExpr("a.txt"), rowType())
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a refused decode", ok, err)
	}
	if len(req.Bag.Findings()) != 1 || req.Bag.Findings()[0].Code != diag.E7116.Def().Code {
		t.Errorf("findings = %+v, want one %s", req.Bag.Findings(), diag.E7116.Def().Code)
	}
}

// WIRE.md §6.6: the hole internal/check's collectionTarget leaves open (internal/check/load.go:76-85).
func TestCSVHeaderNonRecordIsE7116(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.csv": "1\n2\n"})
	e := csvExpr("a.csv")
	e.Args = append(e.Args, headerOpt(true))
	_, ok, err := l.Load(context.Background(), req, e, &types.ListType{Elem: types.IntType})
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a refused decode", ok, err)
	}
	if len(req.Bag.Findings()) != 1 || req.Bag.Findings()[0].Code != diag.E7116.Def().Code {
		t.Errorf("findings = %+v, want one %s", req.Bag.Findings(), diag.E7116.Def().Code)
	}
}

// WIRE.md §6.2: an unrecognized `format:` symbol is a real finding, not a silent refusal.
func TestFormatSymbolUnrecognizedIsE7006(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.dat": "x"})
	e := bareExpr("a.dat")
	e.Args = append(e.Args, &syntax.Arg{Name: &syntax.Ident{Name: "format"}, Value: &syntax.IdentExpr{Name: "xml"}})
	_, ok, err := l.Load(context.Background(), req, e, types.StringType)
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a refused decode", ok, err)
	}
	if len(req.Bag.Findings()) != 1 || req.Bag.Findings()[0].Code != diag.E7006.Def().Code {
		t.Errorf("findings = %+v, want one %s", req.Bag.Findings(), diag.E7006.Def().Code)
	}
}

// WIRE.md §6.1, §2.3: an unrecognized-extension bare load names the display path, resolved first.
func TestBareLoadE7007NamesDisplayPath(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"sub/a.xyz": "x"})
	_, ok, err := l.Load(context.Background(), req, bareExpr("sub/a.xyz"), types.StringType)
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a refused decode", ok, err)
	}
	fd := req.Bag.Findings()
	if len(fd) != 1 || fd[0].Code != diag.E7007.Def().Code {
		t.Fatalf("findings = %+v, want one %s", fd, diag.E7007.Def().Code)
	}
}

// enumDefaultFixture is a record with a field whose default is an enum member, which the
// evaluator runs (mirrors resource/vocab/vocab.canon's rollMode default).
func enumDefaultFixture() *types.RecordType {
	kind := &types.EnumType{Pkg: "p", Name: "Kind"}
	f := &types.Field{
		Name: "mode", Type: kind, Wire: "mode", WirePath: []string{"mode"},
		Default: &syntax.IdentExpr{Name: "local_budget"},
	}
	return &types.RecordType{Pkg: "p", Name: "R", Fields: []*types.Field{f}}
}

// rowsType is `[[String]]`: a headerless load.csv's own shape (WIRE.md §6.6).
func rowsType() *types.ListType {
	return &types.ListType{Elem: &types.ListType{Elem: types.StringType}}
}

// headerRowsType is a List of one-field records: a headered load.csv's own shape (WIRE.md §6.6).
func headerRowsType() *types.ListType {
	f := &types.Field{Name: "n", Type: types.StringType, Wire: "n", WirePath: []string{"n"}}
	return &types.ListType{Elem: &types.RecordType{Pkg: "p", Name: "N", Fields: []*types.Field{f}}}
}

// sameFinding fails unless got is exactly the finding want builds: code, message and span.
func sameFinding(t *testing.T, set *source.FileSet, got diag.Finding, want *diag.Builder) {
	t.Helper()
	bag := diag.NewBag(set, "p")
	want.Report(bag)
	w := bag.Findings()[0]
	if got.Code != w.Code || got.Message != w.Message || got.Span != w.Span {
		t.Errorf("finding = %s %q %+v, want %s %q %+v", got.Code, got.Message, got.Span, w.Code, w.Message, w.Span)
	}
}

// DECISIONS 218: a CR after a closing quote and before no LF is afterQuote, at the CR.
func TestCSVCRAfterClosingQuoteIsAfterQuote(t *testing.T) {
	for _, data := range []string{"\"a\"\rb\n", "\"a\"\r", "x,\"a\"\r,b\n"} {
		l, req := loaderFor(t, map[string]string{"a.csv": data})
		_, ok, err := l.Load(context.Background(), req, csvExpr("a.csv"), rowsType())
		fd := req.Bag.Findings()
		if err != nil || ok || len(fd) != 1 {
			t.Fatalf("%q: ok=%v err=%v findings=%+v, want one", data, ok, err, fd)
		}
		cr := source.Pos(strings.IndexByte(data, '\r'))
		sameFinding(t, l.Set, fd[0], diag.E7113.AtAfterQuote(source.Span{File: fd[0].Span.File, Start: cr, End: cr + 1}, 1))
	}
}

// WIRE.md §6.6: a closing quote followed by CR LF still ends the record, the cell verbatim.
func TestCSVQuotedCellThenCRLF(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.csv": "\"a\"\r\nb\r\n"})
	v, ok, err := l.Load(context.Background(), req, csvExpr("a.csv"), rowsType())
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v findings=%+v", ok, err, req.Bag.Findings())
	}
	if rows := v.(*value.List).Elems; len(rows) != 2 || rows[1].(*value.List).Elems[0].(*value.Str).V != "b" {
		t.Errorf("rows = %s, want [[a], [b]]", v.CanonText())
	}
}

// DECISIONS 218: noHeader is at 1:1 inside the file, zero-width when the file is empty.
func TestCSVNoHeaderSpanInsideFile(t *testing.T) {
	for data, end := range map[string]source.Pos{"": 0, "\xef\xbb\xbf": 1} {
		l, req := loaderFor(t, map[string]string{"a.csv": data})
		e := csvExpr("a.csv")
		e.Args = append(e.Args, headerOpt(true))
		if _, ok, err := l.Load(context.Background(), req, e, headerRowsType()); err != nil || ok {
			t.Fatalf("%q: ok=%v err=%v, want a refused decode", data, ok, err)
		}
		fd := req.Bag.Findings()
		if len(fd) != 1 {
			t.Fatalf("%q: findings = %+v, want one", data, fd)
		}
		sameFinding(t, l.Set, fd[0], diag.E7113.AtNoHeader(source.Span{File: fd[0].Span.File, End: end}))
	}
}

// WIRE.md §6.6: every cell's span is folded in one pass, so a 100k-record file loads in linear time.
func TestCSVLargeFileReadsInLinearTime(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.csv": strings.Repeat(csvBigRecord, csvBigRecords)})
	start := time.Now()
	v, ok, err := l.Load(context.Background(), req, csvExpr("a.csv"), rowsType())
	if elapsed := time.Since(start); elapsed > linearTimeBound {
		t.Errorf("read in %v, want well under %v", elapsed, linearTimeBound)
	}
	if err != nil || !ok || len(v.(*value.List).Elems) != csvBigRecords {
		t.Fatalf("ok=%v err=%v findings=%+v", ok, err, req.Bag.Findings())
	}
	last := v.(*value.List).Elems[csvBigRecords-1].(*value.List).Elems[1]
	if s := last.(*value.Str); s.V != "d\r\ne" || s.P == nil {
		t.Errorf("last cell = %+v", s)
	}
}

// BenchmarkLoadCSV100k is TestCSVLargeFileReadsInLinearTime's read, for its numbers.
func BenchmarkLoadCSV100k(b *testing.B) {
	data := strings.Repeat(csvBigRecord, csvBigRecords)
	for b.Loop() {
		b.StopTimer()
		l, req := loaderFor(b, map[string]string{"a.csv": data})
		b.StartTimer()
		if _, ok, err := l.Load(context.Background(), req, csvExpr("a.csv"), rowsType()); err != nil || !ok {
			b.Fatalf("ok=%v err=%v", ok, err)
		}
	}
}
