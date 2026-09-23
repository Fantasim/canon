package source_test

import (
	"errors"
	"math"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/source"
)

// API.md §1.3: 1-based lines, 1-based columns counted in UTF-8 bytes, end exclusive.
func TestPosition(t *testing.T) {
	var fs source.FileSet
	f, err := fs.Add("a.canon", "/a.canon", []byte("ab\né\n\nx"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		pos       source.Pos
		line, col int
	}{
		{0, 1, 1}, {2, 1, 3}, {3, 2, 1}, {5, 2, 3}, {6, 3, 1}, {7, 4, 1}, {8, 4, 2},
		{-4, 1, 1}, {99, 4, 2},
	}
	for _, tt := range tests {
		line, col := f.Position(tt.pos)
		if line != tt.line || col != tt.col {
			t.Errorf("Position(%d) = %d:%d, want %d:%d", tt.pos, line, col, tt.line, tt.col)
		}
	}
}

// GRAMMAR.md §1: "\r\n" is normalized to "\n" before lexing; a lone "\r" is kept for the lexer.
func TestAddNormalizes(t *testing.T) {
	var fs source.FileSet
	f, err := fs.Add(`dir\a.canon`, "/p/dir/a.canon", []byte("a\r\nb\rc\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(f.Content); got != "a\nb\rc\n" {
		t.Errorf("Content = %q", got)
	}
	if line, _ := f.Position(4); line != 2 {
		t.Errorf("line of c = %d, want 2", line)
	}
	if fs.File(f.ID) != f || f.ID != 1 {
		t.Errorf("File(%d) is not the added file", f.ID)
	}
}

func TestFileUnknown(t *testing.T) {
	var fs source.FileSet
	if fs.File(source.NoFile) != nil || fs.File(1) != nil {
		t.Error("an empty set returned a file")
	}
	if loc := fs.Locate(source.Span{File: 3, Start: 1, End: 2}); loc != (source.Location{}) {
		t.Errorf("Locate of an unknown file = %+v", loc)
	}
}

// DECISIONS 81: the file set answers by id what diag renders; an unknown id is no location.
func TestFileSetByID(t *testing.T) {
	var fs source.FileSet
	f, err := fs.Add("a.canon", "/a.canon", []byte("ab\r\ncd"))
	if err != nil {
		t.Fatal(err)
	}
	line, col := fs.Position(f.ID, 4)
	if fs.Path(f.ID) != "a.canon" || string(fs.Content(f.ID)) != "ab\ncd" || line != 2 || col != 2 {
		t.Errorf("by id: %q %q %d:%d", fs.Path(f.ID), fs.Content(f.ID), line, col)
	}
	for _, id := range []source.FileID{source.NoFile, f.ID + 1} {
		line, col := fs.Position(id, 1)
		if fs.Path(id) != "" || fs.Content(id) != nil || line != 0 || col != 0 {
			t.Errorf("id %d: %q %q %d:%d, want no file", id, fs.Path(id), fs.Content(id), line, col)
		}
	}
}

func TestAddTooLarge(t *testing.T) {
	var fs source.FileSet
	_, err := fs.Add("big.canon", "/big.canon", make([]byte, math.MaxInt32+1))
	if !errors.Is(err, source.ErrFileTooLarge) {
		t.Errorf("Add = %v, want ErrFileTooLarge", err)
	}
}

func TestSpanHelpers(t *testing.T) {
	a := source.Span{File: 2, Start: 4, End: 9}
	b := source.Span{File: 2, Start: 7, End: 12}
	if a.Len() != 5 || !a.Contains(4) || a.Contains(9) || a.Contains(3) {
		t.Errorf("Len/Contains wrong for %+v", a)
	}
	if got := a.Cover(b); got != (source.Span{File: 2, Start: 4, End: 12}) {
		t.Errorf("Cover = %+v", got)
	}
}

// The file set is safe for concurrent use: every Add gets a distinct id (run under -race).
func TestConcurrentAdd(t *testing.T) {
	const workers = 16
	var fs source.FileSet
	var wg sync.WaitGroup
	ids := make([]source.FileID, workers)
	for i := range workers {
		wg.Go(func() {
			f, err := fs.Add("f.canon", "/f.canon", []byte("x"))
			if err != nil {
				t.Error(err)
				return
			}
			ids[i] = f.ID
			_ = fs.File(f.ID)
		})
	}
	wg.Wait()
	seen := map[source.FileID]bool{}
	for _, id := range ids {
		if seen[id] || id == source.NoFile || int(id) > workers {
			t.Fatalf("ids = %v", ids)
		}
		seen[id] = true
	}
}
