package progen_test

import (
	"bytes"
	"flag"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

var flagRekeep = flag.Bool("progen.rekeep", false, "keep every open mismatch archive again, rebuilt from its operator and seed and shrunk without leftovers")

// TestRekeep rebuilds each open mutation archive born of a mismatch from its operator and seed
// and, when the seed still picks its site and the case still fails so, keeps it again: shrunk
// with no finding its unshrunk case does not report, and born anew. Its open line stays.
func TestRekeep(t *testing.T) {
	if !*flagRekeep {
		t.Skip("run with -progen.rekeep only")
	}
	c := examples(t)
	paths, err := filepath.Glob(keptGlob)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		arch, err := progen.ReadCounterexample(p)
		if err != nil {
			t.Fatal(err)
		}
		if arch.Suite == suiteMutation && arch.Open != "" {
			t.Run(filepath.Base(p), func(t *testing.T) { rekeep(t, c, p, arch) })
		}
	}
}

// rekeep keeps arch again at p, if its case can be rebuilt and still fails with a mismatch.
func rekeep(t *testing.T, c *corpus, p string, arch *progen.Counterexample) {
	ops := catalogue()
	i := slices.IndexFunc(ops, func(o operator) bool { return o.name() == arch.Name })
	if i < 0 {
		t.Skipf("no operator is named %q", arch.Name)
	}
	o := ops[i]
	pl := progen.Pick(progen.NewRand(arch.Seed), o.placements(c))
	m, err := pl.site.Mutate(c.project)
	if err != nil {
		t.Fatal(err)
	}
	if !sameSite(arch, m) {
		t.Skip("the seed no longer picks the archived site")
	}
	base := c.baselineOf(pl.run.pkgs)
	v := judge(o, pl.run, m.At, m.Project, base)
	if v.Kind == "" || slices.Contains(crashKinds, v.Kind) || !strings.HasPrefix(arch.Born, classMismatch) {
		t.Skipf("born %q; the unshrunk case now: %q", arch.Born, v.Sig)
	}
	small := shrinkMutation(failure{o: o, run: pl.run, m: m, v: v, k: arch.Case, seed: arch.Seed, base: base})
	small.Open, small.Born = arch.Open, bornOf(suiteMutation, small.Sig)
	t.Logf("born %q, was %q", small.Born, arch.Born)
	writeArchive(t, p, small)
}

// sameSite tells that a rebuilt mutation wrote, where the archive expects its finding, the text
// the archive holds there, on a line of which the archive's is what shrinking left.
func sameSite(arch *progen.Counterexample, m progen.Mutated) bool {
	f := strings.Fields(arch.Want)
	if len(f) != wantFields || f[1] != m.At.Path {
		return false
	}
	start, err1 := strconv.Atoi(f[2])
	end, err2 := strconv.Atoi(f[3])
	kept, _ := arch.Files.Get(f[1])
	built, _ := m.Project.Get(m.At.Path)
	if err1 != nil || err2 != nil || end > len(kept) || start > end {
		return false
	}
	return string(kept[start:end]) == string(built[m.At.Start:m.At.End]) &&
		cutFrom(lineOf(kept, start), lineOf(built, m.At.Start))
}

// lineOf is the text of the line of src that holds off, without its blanks.
func lineOf(src []byte, off int) string {
	s := bytes.LastIndexByte(src[:off], '\n') + 1
	e := len(src)
	if i := bytes.IndexByte(src[off:], '\n'); i >= 0 {
		e = off + i
	}
	return strings.Join(strings.Fields(string(src[s:e])), "")
}

// cutFrom tells that short is long with bytes cut out, as shrinking cuts a line.
func cutFrom(short, long string) bool {
	i := 0
	for j := 0; i < len(short) && j < len(long); j++ {
		if short[i] == long[j] {
			i++
		}
	}
	return i == len(short)
}
