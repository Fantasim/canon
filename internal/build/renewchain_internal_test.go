package build

import (
	"fmt"
	"path"
	"testing"
)

// renewTail is appended to a/a.canon after its last declaration: a parse of it shares them all.
const renewTail = "// The end.\n"

// renewStep is one snapshot of the entries case: a/a.canon with a comment appended, or not, and
// a/items/two.canon's numbers set to n.
type renewStep struct {
	tail bool
	n    int
}

// renewSteps re-check along one lineage. A step changing both files swaps a.canon with every
// declaration shared and checks two's entry again under the table a.canon declares, as a
// snapshot holding another parse of a.canon's content does (TestCacheConcurrentSnapshots).
var renewSteps = []renewStep{{true, 1}, {true, 2}, {false, 0}, {false, 1}, {true, 2}, {true, 0}}

// IMPLEMENTATION-PLAN §7.6 NFR-02: an entry checked again under a shared table's file equals cold.
func TestIncrementalEntryUnderSharedTable(t *testing.T) {
	z := archiveAnalyzer(t, entriesCase)
	a, two := path.Join(archiveRoot, "a/a.canon"), path.Join(archiveRoot, "a/items/two.canon")
	aText, err := z.fs.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	twoText, err := z.fs.ReadFile(two)
	if err != nil {
		t.Fatal(err)
	}
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	for i, st := range renewSteps {
		text := aText
		if st.tail {
			text = append(text[:len(text):len(text)], renewTail...)
		}
		z.fs.set(a, text)
		z.fs.set(two, fieldNumber.ReplaceAll(twoText, fmt.Appendf(nil, ": %d", st.n)))
		prev := warm.r.epoch
		warm, cold = z.pair(t)
		same(t, fmt.Sprintf("step %d (%v)", i, st), warm, cold)
		if warm.r.epoch != prev {
			t.Fatalf("step %d (%v): a full check, not a Recheck along the lineage", i, st)
		}
	}
}
