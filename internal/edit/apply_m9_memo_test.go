package edit_test

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// spyVerdicts is a memo of fixed points that counts what it is asked and told.
type spyVerdicts struct {
	mu    sync.Mutex
	kept  map[edit.VerdictKey]bool
	asked int
	told  int
}

func (s *spyVerdicts) Fixed(k edit.VerdictKey) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked++
	return s.kept[k]
}

func (s *spyVerdicts) Keep(k edit.VerdictKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.told++
	if s.kept == nil {
		s.kept = map[edit.VerdictKey]bool{}
	}
	s.kept[k] = true
}

// A verdict kept by content gives what a fresh check gives, for the text of a snapshot and for a
// text that is not, on a miss and on a hit; only a fixed point is kept, and a hit comes from a
// snapshot whose tree is another parse of the same bytes, as each Edit's is.
func TestM9ContentMemo(t *testing.T) {
	// API.md M9
	for _, c := range []struct {
		name, text, raw string
		fixed           bool
	}{
		{"canonical", keptCanon, keptCanon, true},
		{"not canonical", keptLoose, keptLoose, false},
		{"carriage returns", keptCanon, strings.ReplaceAll(keptCanon, "\n", "\r\n"), false},
		{"other bytes", keptCanon, keptLoose, false},
		{"canonical other bytes", keptLoose, keptCanon, false}, // the source path says fixed; the tree path is not taken
		{"byte order mark", keptBOM, keptBOM, false},
		{"tab indentation", keptTab, keptTab, false},
		{"lexer error", keptLex, keptLex, false},
	} {
		var fs source.FileSet
		src, err := fs.Add("d/d.canon", "d/d.canon", []byte(c.raw))
		if err != nil {
			t.Fatal(err)
		}
		want, wantErr := format.Source(src, syntax.FileSource, diag.NewBag(&fs, ""))
		memo := &spyVerdicts{}
		for ask := range 3 { // a miss, then two hits each on a tree parsed again
			s := open(t, mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(c.text)}, nil, "", "d")
			got, err := edit.CanonicalSourceWith(s.snap, memo, "d/d.canon", []byte(c.raw))
			if !bytes.Equal(got, want) || !errors.Is(err, wantErr) {
				t.Errorf("%s, ask %d: %q, %v; a fresh check gives %q, %v", c.name, ask, got, err, want, wantErr)
			}
		}
		wantTold := 0
		if c.fixed {
			wantTold = 1
		}
		if memo.told != wantTold || memo.asked != 3 {
			t.Errorf("%s: kept %d verdicts over %d asks, want %d over 3", c.name, memo.told, memo.asked, wantTold)
		}
	}
}

// A text the formatter prints as itself, asked of a tree holding other bytes, is a fixed point
// by the source path alone: M9 answers so and keeps nothing, only a tree's own verdict being kept.
func TestM9MemoKeepsTreeVerdictsOnly(t *testing.T) {
	// API.md M9
	memo := &spyVerdicts{}
	s := open(t, mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(keptLoose)}, nil, "", "d")
	for range 2 {
		if got, err := edit.CanonicalSourceWith(s.snap, memo, "d/d.canon", []byte(keptCanon)); err != nil || string(got) != keptCanon {
			t.Fatalf("the source path gave %q, %v", got, err)
		}
	}
	if memo.told != 0 || len(memo.kept) != 0 {
		t.Errorf("%d verdicts kept from the source path", memo.told)
	}
}

// A kept verdict for bytes is not the layout of a tree holding other bytes: M9 adopts the verdict
// for a tree only when the tree holds exactly the raw bytes (API.md M9).
func TestM9MemoAdoptsOnlyItsBytes(t *testing.T) {
	memo := &spyVerdicts{}
	canon := open(t, mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(keptCanon)}, nil, "", "d")
	if _, err := edit.CanonicalSourceWith(canon.snap, memo, "d/d.canon", []byte(keptCanon)); err != nil {
		t.Fatal(err)
	}
	loose := open(t, mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(keptLoose)}, nil, "", "d")
	if got, err := edit.CanonicalSourceWith(loose.snap, memo, "d/d.canon", []byte(keptCanon)); err != nil || string(got) != keptCanon {
		t.Fatalf("a kept verdict gave %q, %v", got, err)
	}
	if memo.asked != 2 || memo.told != 1 {
		t.Fatalf("the second ask was not a hit: %d asks, %d kept", memo.asked, memo.told)
	}
	if fixed, err := format.Canonical(edit.TreeOf(loose.snap, "d/d.canon")); fixed || err != nil {
		t.Errorf("the tree of other bytes was given a fixed point's layout: %v, %v", fixed, err)
	}
}
