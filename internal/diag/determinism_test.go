package diag_test

import (
	"bytes"
	"math/rand/v2"
	"reflect"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

var orderFiles = diag.MemFiles{
	{Path: "b.canon", Content: "one\ntwo\nthree\nfour\n"},
	{Path: "a.canon", Content: "one\ntwo\nthree\nfour\n"},
}

// report is one Report call; reports holds duplicates that differ only in the fields the F2
// key does not read, so only a total order makes the kept one fixed.
type report func(*diag.Bag)

func reports() []report {
	at := source.Span{File: 1, Start: 4, End: 7}
	other := source.Span{File: 2, Start: 8, End: 13}
	longer := source.Span{File: 1, Start: 4, End: 9}
	frames := func(n int, fn string) []diag.Frame {
		out := make([]diag.Frame, n)
		for i := range out {
			out[i] = diag.Frame{Fn: fn, Span: other}
		}
		return out
	}
	return []report{
		func(b *diag.Bag) { diag.E5001.At(at, "m").Path("y").Report(b) },
		func(b *diag.Bag) { diag.E5001.At(at, "m").Path("x").Report(b) },
		func(b *diag.Bag) { diag.E5001.At(at, "m").Path("x").Pointer("/b").Report(b) },
		func(b *diag.Bag) { diag.E5001.At(at, "m").Check("c2").Layer("l").Report(b) },
		func(b *diag.Bag) { diag.E5001.At(at, "m").Related(other, diag.NoteCheck("z")).Report(b) },
		func(b *diag.Bag) { diag.E5001.At(at, "m").Related(at, diag.NoteCheck("z")).Report(b) },
		func(b *diag.Bag) { diag.E5001.At(longer, "m").Report(b) },
		func(b *diag.Bag) { diag.E5001.At(at, "m").Stack(frames(2, "g")).Report(b) },
		func(b *diag.Bag) { diag.E5001.At(at, "m").Stack(frames(2, "f")).MoreFrames(3).Report(b) },
		func(b *diag.Bag) { diag.E5001.At(at, "m").Reads([]string{"b", "a"}).Report(b) },
		func(b *diag.Bag) { diag.E5001.At(at, "m").Reads([]string{"a", "b"}).Report(b) },
		func(b *diag.Bag) { diag.W5001.At(other, "w").Path("y").Report(b) },
		func(b *diag.Bag) { diag.W5001.At(other, "w").Path("x").Report(b) },
		func(b *diag.Bag) { diag.E5001.At(source.Span{}, "none").Report(b) },
		func(b *diag.Bag) { diag.E5001.At(source.Span{File: 2, Start: 1, End: 2}, "m").Report(b) },
	}
}

// fill reports rs into a fresh bag in the order perm gives, from workers goroutines.
func fill(rs []report, perm []int, workers int) *diag.Bag {
	bag := diag.NewBag(orderFiles, "p")
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for i := w; i < len(perm); i += workers {
				rs[perm[i]](bag)
			}
		})
	}
	wg.Wait()
	return bag
}

func rendered(t *testing.T, bag *diag.Bag) string {
	t.Helper()
	var buf bytes.Buffer
	for _, form := range []diag.Format{diag.FormatText, diag.FormatJSON} {
		opt := diag.RenderOptions{Format: form, Summary: bag.Summary(), Golden: true}
		if err := diag.Render(&buf, orderFiles, bag.Findings(), opt); err != nil {
			t.Fatal(err)
		}
	}
	return buf.String()
}

// EVALUATION.md §14, NFR-05: the kept findings and their forms ignore the Report order.
func TestBagIsOrderIndependent(t *testing.T) {
	rs := reports()
	rng := rand.New(rand.NewPCG(7, 11))
	identity := make([]int, len(rs))
	for i := range identity {
		identity[i] = i
	}
	first := fill(rs, identity, 1)
	want, wantText := first.Findings(), rendered(t, first)
	for run := range 64 {
		perm := rng.Perm(len(rs))
		bag := fill(rs, perm, 1+run%8)
		if got := bag.Findings(); !reflect.DeepEqual(got, want) {
			t.Fatalf("order %v: findings differ\n got %+v\nwant %+v", perm, got, want)
		}
		if got := rendered(t, bag); got != wantText {
			t.Fatalf("order %v: output differs\n got:\n%s\nwant:\n%s", perm, got, wantText)
		}
	}
}

// EVALUATION.md §14: of duplicates, the least in the total order is kept.
func TestBagKeepsTheLeastDuplicate(t *testing.T) {
	bag := diag.NewBag(orderFiles, "p")
	for _, r := range []report{reports()[0], reports()[1], reports()[12], reports()[11]} {
		r(bag)
	}
	fs := bag.Findings()
	if len(fs) != 2 || fs[0].Path != "x" || fs[1].Path != "x" {
		t.Fatalf("kept %+v", fs)
	}
}

// DOCTRINE §5: of duplicates in two versions of one path, the one kept never hangs on FileIDs.
func TestBagKeepsADuplicateWhateverItsFileID(t *testing.T) {
	older, newer := diag.MemFile{Path: "a.canon", Content: "one\n"}, diag.MemFile{Path: "a.canon", Content: "one two\n"}
	for _, files := range []diag.MemFiles{{older, newer}, {newer, older}} {
		bag := diag.NewBag(files, "p")
		for _, id := range []source.FileID{2, 1} {
			diag.E5001.At(source.Span{File: id, Start: 0, End: 3}, "m").Report(bag)
		}
		fs := bag.Findings()
		if len(fs) != 1 || string(files.Content(fs[0].Span.File)) != older.Content {
			t.Fatalf("files %v: kept %+v", files, fs)
		}
	}
}

// API.md F2, F16: Write sorts findings of several packages the same whatever their order.
func TestWriteIsOrderIndependent(t *testing.T) {
	a, b := fill(reports(), []int{0, 3, 5, 11}, 1), fill(reports(), []int{1, 4, 12, 13}, 1)
	located := diag.Locate(orderFiles, append(a.Findings(), b.Findings()...))
	var want bytes.Buffer
	if err := diag.Write(&want, located, diag.RenderOptions{Format: diag.FormatJSON}); err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(3, 5))
	for range 32 {
		rng.Shuffle(len(located), func(i, j int) { located[i], located[j] = located[j], located[i] })
		var got bytes.Buffer
		if err := diag.Write(&got, located, diag.RenderOptions{Format: diag.FormatJSON}); err != nil {
			t.Fatal(err)
		}
		if got.String() != want.String() {
			t.Fatalf("got:\n%s\nwant:\n%s", got.String(), want.String())
		}
	}
}

// API.md F13: Stack and MoreFrames set their counts, which the finding adds.
func TestStackCountsCutFramesOnce(t *testing.T) {
	frames := make([]diag.Frame, diag.MaxStackFrames+4)
	bag := diag.NewBag(orderFiles, "p")
	diag.E4001.At(source.Span{}, source.Span{}).Stack(frames).Stack(frames).MoreFrames(2).MoreFrames(3).Report(bag)
	diag.E4102.At(source.Span{}).Stack(frames).Stack(frames[:2]).Report(bag)
	fs := bag.Findings()
	if len(fs) != 2 || fs[0].MoreFrames != 7 || len(fs[0].Stack) != diag.MaxStackFrames || fs[1].MoreFrames != 0 {
		t.Fatalf("got %+v", fs)
	}
}
