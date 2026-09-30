package lock_test

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	phaseEntries = 7000 // the NFR-01 benchmark's stable table (IMPLEMENTATION-PLAN §7.6)
	phasePkg     = "bench"
	phaseTable   = "bench.items"
	phaseLock    = "bench/canon.lock"
	phaseSeed    = 3
	phaseSpan    = 16 // the bytes between two entries' spans
)

// phaseSetup is a stable table of phaseEntries entries whose keys are out of canonical order, as
// entry files placed by kind give them, and the canon.lock that locks them all.
func phaseSetup(b *testing.B) (*value.Table, *source.File) {
	b.Helper()
	elem := &types.RecordType{Pkg: phasePkg, Name: "Item"}
	t := &value.Table{T: &types.TableType{Elem: elem, Stable: true}}
	for i, k := range rand.New(rand.NewPCG(phaseSeed, phaseSeed)).Perm(phaseEntries) {
		p := &value.Prov{Span: source.Span{File: 1, Start: source.Pos(i * phaseSpan)}}
		t.Entries = append(t.Entries, &value.Record{T: elem, Ident: &value.Identity{Key: value.Key{S: fmt.Sprintf("item_%05d", k)}}, P: p})
	}
	s := lock.NewSources(phasePkg)
	if _, err := s.AddTable(phaseTable, t, nil); err != nil {
		b.Fatal(err)
	}
	f := lock.New(phasePkg)
	if _, err := f.Update(s); err != nil {
		b.Fatal(err)
	}
	src, err := (&source.FileSet{}).Add(phaseLock, "/"+phaseLock, f.Format())
	if err != nil {
		b.Fatal(err)
	}
	return t, src
}

// IMPLEMENTATION-PLAN §7.6 NFR-01, LOCK.md §3, §4: one analysis's lock phase, cold and warm.
func BenchmarkLockPhase(b *testing.B) {
	for _, warm := range []bool{false, true} {
		b.Run(fmt.Sprintf("warm=%t", warm), func(b *testing.B) {
			t, src := phaseSetup(b)
			bag := diag.NewBag(&source.FileSet{}, phasePkg)
			kept, _ := lock.Parse(src.ID, src.Content, phasePkg, bag)
			var prev *lock.Order
			for b.Loop() {
				s := lock.NewSources(phasePkg)
				o, err := s.AddTable(phaseTable, t, prev)
				if err != nil {
					b.Fatal(err)
				}
				f := kept
				if warm {
					prev = o
				} else {
					f, _ = lock.Parse(src.ID, src.Content, phasePkg, bag)
				}
				f.Verify(s, bag)
			}
			if len(bag.Findings()) != 0 {
				b.Fatal(bag.Findings())
			}
		})
	}
}
