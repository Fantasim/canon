package load_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"golang.org/x/tools/txtar"
)

// bareEncodingCase is the bare load whose file has \r\n line ends before a byte that is not UTF-8.
const bareEncodingCase = "testdata/findings/E7105_3.txtar"

// sumFS is memFS knowing each file's SHA-256 as a workspace snapshot does, counting its reads.
type sumFS struct {
	memFS
	reads *int
}

func (s sumFS) ReadFile(name string) ([]byte, error) {
	*s.reads++
	return s.memFS.ReadFile(name)
}

func (s sumFS) SumFile(name string) ([sha256.Size]byte, bool, error) {
	data, err := s.memFS.ReadFile(name)
	if err != nil {
		return [sha256.Size]byte{}, true, err
	}
	return sha256.Sum256(data), true, nil
}

// keptSource is a file a cache keeps, and the SHA-256 of its content.
type keptSource struct {
	sum [sha256.Size]byte
	src *source.File
}

// keepIn gives l a cache of every file its loads add, which Kept serves while the content is the same.
func keepIn(l *load.Loader) map[string]keptSource {
	kept := map[string]keptSource{}
	l.Add = func(display, abs string, data []byte, sum [sha256.Size]byte) (*source.File, error) {
		src, err := l.Set.Add(display, abs, data)
		kept[display+"\x00"+abs] = keptSource{sum: sum, src: src}
		return src, err
	}
	l.Kept = func(display, abs string, sum [sha256.Size]byte) (*source.File, bool) {
		k, ok := kept[display+"\x00"+abs]
		return k.src, ok && k.sum == sum
	}
	return kept
}

// IMPLEMENTATION-PLAN §7.6, API.md S1 (P18): loads and replays take kept files by sum, unread; a changed one is read.
func TestKeptTakesUnread(t *testing.T) {
	ctx := context.Background()
	l, req := loaderFor(t, memoTree)
	m, reads := l.FS.(memFS), 0
	l.FS = sumFS{memFS: m, reads: &reads}
	kept := keepIn(l)
	e := dirExpr("data/*.json")
	first, err := l.Recorded(ctx, req, e, rowType())
	if err != nil || first.Inputs == nil || reads != len(kept) {
		t.Fatalf("err=%v inputs=%t, %d reads of %d files", err, first.Inputs != nil, reads, len(kept))
	}
	for _, step := range []struct {
		name   string
		change func()
		reads  int
		replay bool
	}{
		{"unchanged", func() {}, 0, true},
		{"a edited", func() {
			m.m[strings.TrimPrefix(projectDir, "/")+"/data/a.json"] = &fstest.MapFile{Data: []byte(`{"label": "A"}`)}
		}, 1, false},
	} {
		step.change()
		reads = 0
		if got := l.Replay(first.Inputs); got != step.replay || reads != 0 {
			t.Errorf("%s: replay answered alike %t after %d reads, want %t after none", step.name, got, reads, step.replay)
		}
		v, ok, err := l.Load(ctx, req, e, rowType())
		if err != nil || !ok || reads != step.reads {
			t.Errorf("%s: ok=%t err=%v after %d reads, want %d", step.name, ok, err, reads, step.reads)
		}
		if want := strings.Contains(step.name, "edited"); strings.Contains(v.CanonText(), `"A"`) != want {
			t.Errorf("%s: value %s", step.name, v.CanonText())
		}
	}
}

// WIRE.md §3, DECISIONS 163 (P18): a bare load of a kept file that does not decode reports a cold load's raw offset.
func TestKeptBareEncodingOffset(t *testing.T) {
	a, err := txtar.ParseFile(bareEncodingCase)
	if err != nil {
		t.Fatal(err)
	}
	want, ok := archiveFile(a, findingsFile)
	if !ok {
		t.Fatal("no findings to compare with")
	}
	reads := 0
	l := &load.Loader{FS: sumFS{memFS: newMemFS(a), reads: &reads}, Set: &source.FileSet{}}
	keepIn(l)
	for _, run := range []string{"read, then kept", "taken by sum"} {
		reads = 0
		if got := bareFindings(t, l, a); !bytes.Equal(got, want) {
			t.Errorf("%s: findings\n%s\nwant, as cold\n%s", run, got, want)
		}
		if reads != 1 {
			t.Errorf("%s: %d reads, want 1: the bytes are read for the finding alone", run, reads)
		}
	}
}

// bareFindings is what l reports loading a's call, rendered as TestFindingsCall renders it.
func bareFindings(t *testing.T, l *load.Loader, a *txtar.Archive) []byte {
	t.Helper()
	data, _ := archiveFile(a, callFile)
	text := strings.TrimSuffix(string(data), "\n")
	set := l.Set
	src, err := set.Add(callFile, projectDir+"/"+callFile, data)
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(set, "p")
	var ok bool
	if l.Layout, ok = project.NewLayout(&project.Project{}, projectDir, nil, bag); !ok {
		t.Fatal("layout")
	}
	req := load.Request{Pkg: "p", Span: source.Span{File: src.ID, End: source.Pos(len(text))}, Bag: bag}
	if _, _, err := l.Load(context.Background(), req, loadCall(t, text), rowType()); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := diag.Render(&buf, set, bag.Findings(), diag.RenderOptions{Summary: bag.Summary(), Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
