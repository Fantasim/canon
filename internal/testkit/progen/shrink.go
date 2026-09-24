package progen

import (
	"bytes"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Region is a byte range of a text that shrinking keeps: a site's text, or where its finding
// is expected. The units it overlaps are pinned, and it moves with the units deleted before it.
type Region struct {
	Start, End int
}

// Shrink reduces n units in the manner of Zeller and Hildebrandt's ddmin, removal side only: it
// tries to drop runs of units that are not pinned, halving the run length when none can go, and
// keeps a drop when fails still holds. It calls fails at most tries times.
func Shrink(n int, pinned []bool, fails func(keep []bool) bool, tries int) []bool {
	keep := make([]bool, n)
	for i := range keep {
		keep[i] = true
	}
	for chunk := n / halves; chunk >= 1 && tries > 0; {
		if !dropChunks(keep, pinned, chunk, fails, &tries) {
			chunk /= halves
		}
	}
	return keep
}

// dropChunks tries to delete each run of chunk units that are kept and free; true when one
// could go.
func dropChunks(keep, pinned []bool, chunk int, fails func([]bool) bool, left *int) bool {
	dropped := false
	for from := 0; from < len(keep) && *left > 0; from += chunk {
		trial := append([]bool(nil), keep...)
		changed := false
		for i := from; i < min(from+chunk, len(keep)); i++ {
			if trial[i] && !pinned[i] {
				trial[i], changed = false, true
			}
		}
		if !changed {
			continue
		}
		*left--
		if fails(trial) {
			copy(keep, trial)
			dropped = true
		}
	}
	return dropped
}

// ShrinkText deletes lines, then tokens, of src while fails still holds, never a unit that
// overlaps one of pins, calling fails at most tries times per pass. It returns the smallest
// failing text found and where the pins now lie, in their order.
func ShrinkText(src []byte, pins []Region, fails func(src []byte, pins []Region) bool, tries int) ([]byte, []Region) {
	for _, split := range []func([]byte) []int{lineCuts, tokenCuts} {
		u := cutUnits(src, split(src), pins)
		keep := Shrink(len(u.parts), u.pinned, func(keep []bool) bool {
			return fails(u.join(keep, pins))
		}, tries)
		src, pins = u.join(keep, pins)
	}
	return src, pins
}

// units is a text cut at the offsets cuts (each unit runs to the next cut), with the pinned
// ones marked.
type units struct {
	parts  [][]byte
	pinned []bool
	offset []int // each part's start in the original text
}

func cutUnits(src []byte, cuts []int, pins []Region) units {
	var u units
	start := 0
	for _, c := range append(cuts, len(src)) {
		if c <= start {
			continue
		}
		u.parts = append(u.parts, src[start:c])
		u.pinned = append(u.pinned, overlaps(start, c, pins))
		u.offset = append(u.offset, start)
		start = c
	}
	return u
}

// overlaps tells a unit [start, end) that overlaps a pin, or holds an empty pin's place.
func overlaps(start, end int, pins []Region) bool {
	for _, at := range pins {
		if start < at.End && end > at.Start || start <= at.Start && at.Start < end {
			return true
		}
	}
	return false
}

// join is the text of the kept units and where the pins lie in it.
func (u units) join(keep []bool, pins []Region) ([]byte, []Region) {
	var b bytes.Buffer
	moved := make([]Region, len(pins))
	copy(moved, pins)
	for i, p := range u.parts {
		if keep[i] {
			b.Write(p)
			continue
		}
		for k, at := range pins {
			if u.offset[i] < at.Start {
				moved[k].Start -= len(p)
				moved[k].End -= len(p)
			}
		}
	}
	return b.Bytes(), moved
}

// lineCuts are the starts of src's lines after the first.
func lineCuts(src []byte) []int {
	var cuts []int
	for i, c := range src {
		if c == '\n' && i+1 < len(src) {
			cuts = append(cuts, i+1)
		}
	}
	return cuts
}

// tokenCuts are the ends of src's tokens as the lexer sees them: each unit is a token with the
// trivia before it.
func tokenCuts(src []byte) []int {
	fs := &source.FileSet{}
	file, err := fs.Add(shrinkFile, rootDir+shrinkFile, src)
	if err != nil {
		return lineCuts(src)
	}
	tree := syntax.Parse(file, syntax.FileSource, diag.NewBag(fs, ""))
	var cuts []int
	for _, t := range tree.Tokens {
		if t.End > t.Start {
			cuts = append(cuts, int(t.End))
		}
	}
	return cuts
}
