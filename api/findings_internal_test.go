package canon

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// publicOf is the canon.Finding of a resolved finding, as Check will build it.
func publicOf(l *diag.Located) Finding {
	f := Finding{
		Severity: Severity(l.Severity.String()), Code: string(l.Code), Span: spanOf(l.Loc),
		Pointer: l.Pointer, Package: l.Package, Path: l.Path, Message: l.Message,
		Check: l.Check, Layer: l.Layer, MoreFrames: l.MoreFrames, Reads: l.Reads,
	}
	for _, r := range l.Related {
		f.Related = append(f.Related, Related{Span: spanOf(r.Loc), Note: r.Note})
	}
	for _, fr := range l.Stack {
		f.Stack = append(f.Stack, Frame{Fn: fr.Fn, Span: spanOf(fr.Loc)})
	}
	return f
}

func spanOf(l source.Location) Span {
	return Span{File: l.Path, Line: l.Line, Col: l.Col, EndLine: l.EndLine, EndCol: l.EndCol}
}

// reported is a bag of findings using every field of API.md §4.1, over a real file set.
func reported(t *testing.T) (*source.FileSet, *diag.Bag) {
	t.Helper()
	var files source.FileSet
	lock, err := files.Add("teamboard/canon.lock", "/p/teamboard/canon.lock", []byte("# canon.lock v1\nkind  x\n"))
	if err != nil {
		t.Fatal(err)
	}
	src, err := files.Add("teamboard/taxonomy.canon", "/p/teamboard/taxonomy.canon", []byte("let a = 1\nlet b = a.c\n"))
	if err != nil {
		t.Fatal(err)
	}
	at := func(f *source.File, from, to int) source.Span {
		return source.Span{File: f.ID, Start: source.Pos(from), End: source.Pos(to)}
	}
	frames := make([]diag.Frame, diag.MaxStackFrames+2)
	for i := range frames {
		frames[i] = diag.Frame{Fn: "teamboard.f", Span: at(src, 0, 3)}
	}
	bag := diag.NewBag(&files, "teamboard")
	diag.E6005.AtKind(at(lock, 16, 23), "kind").Report(bag)
	diag.E4001.At(at(src, 18, 21), at(src, 18, 21)).Path("b").Layer("louis").Stack(frames).MoreFrames(2).Report(bag)
	diag.W5001.At(at(src, 0, 9), "one").Check("one").Related(at(src, 0, 9), diag.NoteCheck("one")).
		Reads([]string{"a"}).Pointer("/a").Report(bag)
	diag.E1003.At(source.Span{}, "/p").Report(bag)
	return &files, bag
}

// API.md F16: WriteFindings writes what diag.Render writes, whatever the order.
func TestWriteFindingsIsRender(t *testing.T) {
	files, bag := reported(t)
	located := diag.Locate(files, bag.Findings())
	var public []Finding
	for i := range located {
		public = append(public, publicOf(&located[i]))
	}
	slices.Reverse(public)
	sum := Summary{Errors: 3, Warnings: 1, Packages: 1, Truncated: []Truncation{{Package: "teamboard", Errors: 1}}}
	for _, form := range []diag.Format{diag.FormatText, diag.FormatJSON} {
		var want, got bytes.Buffer
		opt := diag.RenderOptions{Format: form, Summary: sum.diag(), Duration: 1400 * time.Millisecond}
		if err := diag.Render(&want, files, bag.Findings(), opt); err != nil {
			t.Fatal(err)
		}
		o := WriteOptions{JSON: form == diag.FormatJSON, Summary: sum, Duration: opt.Duration}
		if err := WriteFindings(&got, public, o); err != nil {
			t.Fatal(err)
		}
		if got.String() != want.String() {
			t.Errorf("form %d:\n got:\n%s\nwant:\n%s", form, got.String(), want.String())
		}
	}
}

// API.md F5, F6: MarshalJSON writes diag's form, and UnmarshalJSON reads it back whole.
func TestFindingJSONRoundTrip(t *testing.T) {
	files, bag := reported(t)
	for _, l := range diag.Locate(files, bag.Findings()) {
		f := publicOf(&l)
		data, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		if want := l.AppendJSON(nil); !bytes.Equal(data, want) {
			t.Errorf("Marshal:\n got %s\nwant %s", data, want)
		}
		var back Finding
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(back, f) {
			t.Errorf("round trip:\n got %+v\nwant %+v", back, f)
		}
	}
}

// API.md F5: a key F5 does not list, or a severity other than error and warning, is refused.
func TestFindingJSONRefuses(t *testing.T) {
	for _, in := range []string{
		`{"severity":"error","code":"X","package":"p","message":"m","extra":1}`,
		`{"severity":"runtime","code":"X","package":"p","message":"m"}`,
	} {
		var f Finding
		if err := json.Unmarshal([]byte(in), &f); err == nil {
			t.Errorf("%s: read as %+v", in, f)
		}
	}
	if _, err := json.Marshal(Finding{Severity: "fatal"}); !errors.Is(err, errSeverity) {
		t.Errorf("Marshal of an unknown severity: %v", err)
	}
	if err := WriteFindings(&bytes.Buffer{}, []Finding{{}}, WriteOptions{}); !errors.Is(err, errSeverity) {
		t.Errorf("WriteFindings of an unknown severity: %v", err)
	}
}
