package diag_test

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

var bagFiles = diag.MemFiles{
	{Path: "b.canon", Content: "x\ny\nz\n"},
	{Path: "a.canon", Content: "x\ny\nz\n"},
}

// API.md F2, EVALUATION.md §14: no file first, then file bytes, line, column, code, message.
func TestBagSortsFindings(t *testing.T) {
	bag := diag.NewBag(bagFiles, "p")
	diag.E1004.At(source.Span{File: 1, Start: 2}).Report(bag)
	diag.E1002.At(source.Span{File: 1, Start: 2}, "b").Report(bag)
	diag.E1002.At(source.Span{File: 1, Start: 2}, "a").Report(bag)
	diag.E1004.At(source.Span{File: 2, Start: 4}).Report(bag)
	diag.E1004.At(source.Span{File: 2, Start: 2}).Report(bag)
	diag.E1004.At(source.Span{}).Report(bag)
	diag.E1002.At(source.Span{}, "z").Report(bag)
	var got []string
	for _, f := range bag.Findings() {
		got = append(got, bagFiles.Path(f.Span.File)+" "+string(f.Code)+" "+f.Message)
	}
	want := []string{
		" E1002 unknown project key \"z\"",
		" E1004 project.canon must declare \"canon\"",
		"a.canon E1004 project.canon must declare \"canon\"",
		"a.canon E1004 project.canon must declare \"canon\"",
		"b.canon E1002 unknown project key \"a\"",
		"b.canon E1002 unknown project key \"b\"",
		"b.canon E1004 project.canon must declare \"canon\"",
	}
	if !slices.Equal(got, want) {
		t.Errorf("order:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// EVALUATION.md §14: a duplicate is reported once, the least in the total order kept.
func TestBagDropsDuplicates(t *testing.T) {
	bag := diag.NewBag(bagFiles, "p")
	at := source.Span{File: 1, Start: 2, End: 3}
	diag.E5001.At(at, "m").Related(at, diag.NoteCheck("first")).Report(bag)
	diag.E5001.At(at, "m").Related(at, diag.NoteCheck("second")).Report(bag)
	diag.E5001.At(source.Span{File: 1, Start: 3, End: 4}, "m").Report(bag)
	fs := bag.Findings()
	if len(fs) != 2 || fs[0].Related[0].Note != "check first" {
		t.Fatalf("got %+v", fs)
	}
	if s := bag.Summary(); s.Errors != 2 || s.Warnings != 0 || s.Packages != 1 || s.Truncated != nil {
		t.Errorf("summary %+v", s)
	}
}

// API.md F7: a bag keeps DefaultMaxFindings findings, the first in F2 order, and counts
// the dropped ones; findings reported from many goroutines are all collected.
func TestBagCapsFindingsPerPackage(t *testing.T) {
	var sb strings.Builder
	for range diag.DefaultMaxFindings + 5 {
		sb.WriteString("x\n")
	}
	files := diag.MemFiles{{Path: "p.canon", Content: sb.String()}}
	bag := diag.NewBag(files, "p")
	var wg sync.WaitGroup
	for i := range diag.DefaultMaxFindings + 5 {
		wg.Go(func() {
			span := source.Span{File: 1, Start: source.Pos(2 * i)}
			if i%2 == 0 {
				diag.W1001.At(span).Report(bag)
			} else {
				diag.W1001.At(span).Report(bag)
				diag.E1004.At(span).Report(bag)
			}
		})
	}
	wg.Wait()
	fs := bag.Findings()
	if len(fs) != diag.DefaultMaxFindings {
		t.Fatalf("kept %d findings", len(fs))
	}
	if line, _ := files.Position(1, fs[len(fs)-1].Span.Start); line != 667 {
		t.Errorf("last kept finding is on line %d", line)
	}
	s := bag.Summary()
	want := diag.Truncation{Package: "p", Errors: 169, Warnings: 338}
	if s.Errors != 502 || s.Warnings != 1005 || len(s.Truncated) != 1 || s.Truncated[0] != want {
		t.Errorf("summary %+v", s)
	}
	bag.Truncate(-1)
	if n := len(bag.Findings()); n != 0 {
		t.Errorf("Truncate(-1) keeps %d", n)
	}
}

// API.md F7, F8: merged summaries add their counts and list truncations by package name.
func TestSummaryMerge(t *testing.T) {
	a := diag.Summary{Errors: 1, Warnings: 2, Packages: 1, Truncated: []diag.Truncation{{Package: "z", Errors: 1}}}
	b := diag.Summary{Errors: 3, Packages: 2, Truncated: []diag.Truncation{{Package: "a", Warnings: 4}}}
	got := a.Merge(b)
	if got.Errors != 4 || got.Warnings != 2 || got.Packages != 3 || got.Truncated[0].Package != "a" || got.Truncated[1].Package != "z" {
		t.Errorf("merge %+v", got)
	}
}

// API.md F15: `<n> ms` below one second, `<s.d> s` rounded down from one second, `(…)` in
// goldens; singular nouns only for exactly 1.
func TestSummaryLine(t *testing.T) {
	cases := []struct {
		s    diag.Summary
		d    time.Duration
		want string
	}{
		{diag.Summary{Packages: 1}, 0, "0 errors, 0 warnings in 1 package (0 ms)\n"},
		{diag.Summary{Errors: 1, Warnings: 1, Packages: 2}, 999*time.Millisecond + 999*time.Microsecond, "1 error, 1 warning in 2 packages (999 ms)\n"},
		{diag.Summary{Errors: 2, Warnings: 5, Packages: 3}, time.Second, "2 errors, 5 warnings in 3 packages (1.0 s)\n"},
		{diag.Summary{Packages: 1}, 61*time.Second + 999*time.Millisecond, "0 errors, 0 warnings in 1 package (61.9 s)\n"},
		{diag.Summary{Packages: 1}, -time.Second, "0 errors, 0 warnings in 1 package (0 ms)\n"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if err := diag.Render(&buf, bagFiles, nil, diag.RenderOptions{Summary: c.s, Duration: c.d}); err != nil {
			t.Fatal(err)
		}
		if buf.String() != c.want {
			t.Errorf("got %q, want %q", buf.String(), c.want)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

// Render reports a write failure wrapped in ErrWrite.
func TestRenderWriteError(t *testing.T) {
	err := diag.Render(failingWriter{}, bagFiles, nil, diag.RenderOptions{})
	if !errors.Is(err, diag.ErrWrite) || !errors.Is(err, os.ErrClosed) {
		t.Errorf("got %v", err)
	}
}

// API.md F9, F12, F13: no line ends with a space, a related location or frame without a
// file writes no location, and a multi-line message indents every line.
func TestTextFormEdges(t *testing.T) {
	bag := diag.NewBag(bagFiles, "p")
	diag.E5001.At(source.Span{File: 2}, "trailing  \n\nnext").Related(source.Span{}, diag.NoteCheck("")).
		Stack([]diag.Frame{{Fn: "f"}}).Report(bag)
	var buf bytes.Buffer
	if err := diag.Render(&buf, bagFiles, bag.Findings(), diag.RenderOptions{Summary: bag.Summary(), Golden: true}); err != nil {
		t.Fatal(err)
	}
	want := "error[E5001]  a.canon:1:1\n  trailing\n\n  next\n  expected by (check)\n  in f\n\n1 error, 0 warnings in 1 package (…)\n"
	if buf.String() != want {
		t.Errorf("got %q\nwant %q", buf.String(), want)
	}
}
