package load_test

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	sharedHeader = "codes.h"
	sharedPrefix = "C_"
	sharedName   = "C_ONE"
	sharedBefore = "#define C_ONE 1\n#define C_TWO (C_ONE + 1)\n#define C_F(x) (x)\n"
	sharedAfter  = "#define C_ONE 5\n#define C_TWO (C_ONE + 1)\n#define C_F(x) (x)\n"
	sharedEdited = 5
)

// sameFiles is a Loader.Add that keeps one source file per name and content, as a build's cache does.
type sameFiles struct {
	set  *source.FileSet
	kept map[[sha256.Size]byte]*source.File
}

func (s *sameFiles) add(display, abs string, data []byte, sum [sha256.Size]byte) (*source.File, error) {
	if src, ok := s.kept[sum]; ok && src.Path == display && src.Abs == abs {
		return src, nil
	}
	src, err := s.set.Add(display, abs, data)
	if err == nil {
		s.kept[sum] = src
	}
	return src, err
}

// definesRun is one run's load.defines of the shared header over a tree holding text, sharing h
// and same when same is set, and its findings once every load is forced.
func definesRun(t *testing.T, text string, h *load.Headers, same *sameFiles) (*value.Table, []diag.Finding) {
	t.Helper()
	l, req := loaderFor(t, map[string]string{sharedHeader: text})
	if same != nil {
		l.Set, req.Bag = same.set, diag.NewBag(same.set, req.Pkg)
		l.Add, l.Headers = same.add, h
	}
	v, ok, err := l.Load(context.Background(), req, definesExpr(sharedHeader, sharedPrefix), definesTableType())
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v findings=%+v", ok, err, req.Bag.Findings())
	}
	l.FinishDefines()
	return v.(*value.Table), req.Bag.Findings()
}

// IMPLEMENTATION-PLAN §7.6, WIRE.md §6.8: a kept classification gives what a cold run gives.
func TestDefinesKeptAcrossRuns(t *testing.T) {
	h := &load.Headers{}
	same := &sameFiles{set: &source.FileSet{}, kept: map[[sha256.Size]byte]*source.File{}}
	for _, step := range []struct {
		name, text string
		one        int64
	}{
		{"first", sharedBefore, 1},
		{"unchanged", sharedBefore, 1},
		{"edited", sharedAfter, sharedEdited},
		{"put back", sharedBefore, 1},
	} {
		warm, warmFound := definesRun(t, step.text, h, same)
		cold, coldFound := definesRun(t, step.text, nil, nil)
		if got, ok := defineOf(warm, sharedName); !ok || got != step.one {
			t.Errorf("%s: %s = %d (%t), want %d", step.name, sharedName, got, ok, step.one)
		}
		if len(warm.Entries) != len(cold.Entries) || len(warmFound) != 1 || len(coldFound) != 1 ||
			warmFound[0].Code != diag.W7101.Def().Code || warmFound[0].Span.Start != coldFound[0].Span.Start {
			t.Errorf("%s: %d defines, findings %v; cold %d defines, findings %v", step.name, len(warm.Entries), warmFound, len(cold.Entries), coldFound)
			continue
		}
		for i, w := range warm.Entries {
			c := cold.Entries[i]
			if w.Ident.Key != c.Ident.Key || w.P.Span.Start != c.P.Span.Start || w.P.Span.End != c.P.Span.End {
				t.Errorf("%s: define %d is %v at %v, cold %v at %v", step.name, i, w.Ident.Key, w.P.Span, c.Ident.Key, c.P.Span)
			}
		}
	}
}
