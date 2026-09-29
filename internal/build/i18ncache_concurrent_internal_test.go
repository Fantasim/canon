package build

import (
	"context"
	"sync"
	"testing"
)

const (
	badReaders = 12
	badRounds  = 4
	badTwo     = "/law/b/b.canon"
	badFrB     = "package b\ntranslation fr\n\nItem.n \"quelque chose\"\n"
	badBSource = "package b\n\n/// Good.\nrecord Good {\n  /// Name.\n  name: String\n}\n"
)

// badFiles is two packages, each with a French file naming a key no declaration has.
func badFiles() roFS {
	return roFS{
		"law/project.canon": srcFile(badProject),
		"law/a/a.canon":     srcFile(badGood),
		"law/a/a.fr.canon":  srcFile(badTranslFr),
		"law/b/b.canon":     srcFile(badBSource),
		"law/b/b.fr.canon":  srcFile(badFrB),
	}
}

// analyzeSel is the dumps of a warm and a cold analysis of z's selection, "" for a failed one.
func analyzeSel(t *testing.T, z *analyzer, sel []string) [2]string {
	t.Helper()
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Error(err)
		return [2]string{}
	}
	var out [2]string
	for i, q := range []*Project{p.WithCache(z.cache), p} {
		a, err := q.Analyze(context.Background(), sel)
		if err != nil {
			t.Error(err)
			return [2]string{}
		}
		out[i] = dumpAnalysis(t, a)
	}
	return out
}

// runBadReaders runs badReaders goroutines badRounds times through one cache; pick gives the
// selection and whether b's source holds a Bad node for a goroutine in a round.
func runBadReaders(t *testing.T, pick func(i, round int) (sel []string, broken bool)) {
	t.Helper()
	base, cache := badFiles(), NewCache()
	for round := range badRounds {
		var wg sync.WaitGroup
		dumps := make([][2]string, badReaders)
		for i := range badReaders {
			z := &analyzer{fs: newEditFS(base), dir: badRoot, cache: cache}
			sel, broken := pick(i, round)
			if broken {
				z.fs.set(badTwo, []byte(badBSource+"\nrecord Item {\n  n: Int\n"))
			}
			wg.Go(func() { dumps[i] = analyzeSel(t, z, sel) })
		}
		wg.Wait()
		for i, d := range dumps {
			if d[0] != d[1] {
				t.Errorf("round %d, reader %d: warm differs from cold", round, i)
			}
		}
	}
}

// I18N.md F4, IMPLEMENTATION-PLAN §7.6: snapshots of one cache, some with a Bad node, re-check as cold (run with -race).
func TestI18NBadNodeCacheConcurrentSnapshots(t *testing.T) {
	runBadReaders(t, func(i, round int) ([]string, bool) { return nil, (i+round)%2 == 0 })
}

// I18N.md F4, IMPLEMENTATION-PLAN §7.6: selections a and b alternate through one cache, evicting each other's entries mid-read.
func TestI18NBadNodeCacheAlternatingSelections(t *testing.T) {
	runBadReaders(t, func(i, round int) ([]string, bool) {
		if i%2 == 0 {
			return []string{"a"}, false
		}
		return []string{"b"}, (i/2+round)%2 == 0
	})
}
