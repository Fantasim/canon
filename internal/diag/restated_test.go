package diag_test

import (
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// reportedOne is the one finding b reports into a bag of package p.
func reportedOne(t *testing.T, b *diag.Builder) diag.Finding {
	t.Helper()
	bag := diag.NewBag(bagFiles, "p")
	b.Report(bag)
	fs := bag.Findings()
	if len(fs) != 1 {
		t.Fatalf("findings %+v", fs)
	}
	return fs[0]
}

// decorated is W5001 with message and every optional field set.
func decorated(message string) *diag.Builder {
	at := source.Span{File: 1, Start: 2, End: 3}
	return diag.W5001.At(at, message).Related(source.Span{File: 2, Start: 0, End: 1}, diag.NoteCheck("big")).
		Check("big").Path("uses.first").Pointer("/a").Layer("knights").
		Stack([]diag.Frame{{Fn: "mk", Span: at}}).MoreFrames(2).Reads([]string{"count"})
}

// DECISIONS 281, ERRORS.md §2.2: Restated re-renders f's own template and keeps every other field.
func TestRestated(t *testing.T) {
	got := reportedOne(t, decorated("count 20 is big")).Restated(bagFiles, "compte 20 trop grand")
	if want := reportedOne(t, decorated("compte 20 trop grand")); !reflect.DeepEqual(got, want) {
		t.Errorf("restated %+v, want %+v", got, want)
	}
	at := source.Span{File: 1, Start: 2}
	if got, want := reportedOne(t, diag.E1002.At(at, "a")).Restated(bagFiles, "b"), reportedOne(t, diag.E1002.At(at, "b")); !reflect.DeepEqual(got, want) {
		t.Errorf("E1002 restated %+v, want %+v", got, want)
	}
	var zero diag.Finding
	if got := zero.Restated(bagFiles, "x"); !reflect.DeepEqual(got, zero) {
		t.Errorf("zero finding restated %+v", got)
	}
}
