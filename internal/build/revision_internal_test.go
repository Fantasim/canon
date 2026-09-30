package build

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

// API.md S3 (log-2026-09-29 M4 P18): RevisionOf gives the revision revisionOf gives of the same
// lines, in any order, sorted or not, with displays that tie and lines that cannot be read.
func TestRevisionOfAsListing(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for n := range 64 {
		lines := make([]Listed, n)
		for i := range lines {
			lines[i] = Listed{Display: fmt.Sprintf("d/%03d", rng.IntN(n+1)), Sum: sha256.Sum256([]byte{byte(rng.IntN(4))}), Unreadable: rng.IntN(8) == 0}
		}
		sorted := slices.SortedFunc(slices.Values(lines), func(a, b Listed) int { return cmp.Compare(a.Display, b.Display) })
		for _, in := range [][]Listed{lines, sorted, slices.CompactFunc(slices.Clone(sorted), func(a, b Listed) bool { return a.Display == b.Display })} {
			if got, want := RevisionOf(in), revisionOf(digests(in)); got != want {
				t.Fatalf("%d lines: RevisionOf %s, the listing's %s", len(in), got, want)
			}
		}
	}
}

// digests is lines as revisionOf takes them.
func digests(lines []Listed) []digest {
	out := make([]digest, len(lines))
	for i, l := range lines {
		out[i] = digest{path: l.Display, text: unreadMark}
		if !l.Unreadable {
			out[i].text = hex.EncodeToString(l.Sum[:])
		}
	}
	return out
}
