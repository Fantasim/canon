package canon_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// storeLeaf is a leaf type whose storage is checked, a value of it and that value's text.
type storeLeaf struct {
	name, typ, val, text string
}

// storeLeaves are the leaves the storage scale case nests: each has a storage check (EVALUATION.md §4.3).
var storeLeaves = []storeLeaf{
	{"duration", "Duration", "1s", "1s"},
	{"int8", "Int8", "1", "1"},
	{"float32", "Float32", "1.5", "1.5"},
	{"refined", "Int(0..)", "1", "1"},
}

// storeScaleSrc is package p: lets w_i = [w_{i-1}, w_{i-1}] over a leaf, held by r's fields a and
// b when record; n reads the leaf.
func storeScaleSrc(leaf storeLeaf, depth int, record bool) string {
	var src strings.Builder
	typ := leaf.typ
	fmt.Fprintf(&src, "/// P.\npackage p\n\n/// W0.\nlet w0: %s = %s\n", typ, leaf.val)
	for i := 1; i <= depth; i++ {
		typ = "[" + typ + "]"
		fmt.Fprintf(&src, "/// W%d.\nlet w%d: %s = [w%d, w%d]\n", i, i, typ, i-1, i-1)
	}
	read := fmt.Sprintf("w%d", depth)
	if record {
		fmt.Fprintf(&src, "\n/// W.\ntype W = %s\n\n/// R.\nrecord R {\n  /// A.\n  a: W\n  /// B.\n  b: W\n}\n\n/// R.\nlet r: R = { a: %s, b: %s }\n", typ, read, read)
		read = "r.b"
	}
	fmt.Fprintf(&src, "\n/// N.\nlet n: %s = %s%s\n", leaf.typ, read, strings.Repeat("[1]", depth))
	return src.String()
}

// storeCheck checks leaf's case at depth and is the time it took, checking n's value.
func storeCheck(t testing.TB, leaf storeLeaf, depth int, record bool) time.Duration {
	t.Helper()
	p := openShared(t, storeScaleSrc(leaf, depth, record), false, nil)
	start := time.Now()
	res, err := p.Check(context.Background(), "p")
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Findings {
		if f.Severity == "error" {
			t.Fatalf("depth %d: %s %s %s", depth, f.Code, f.Path, f.Message)
		}
	}
	if v, err := p.Value(context.Background(), "p:n"); err != nil || v.Text != leaf.text {
		t.Fatalf("depth %d: n = %v (%v), want %s", depth, v, err, leaf.text)
	}
	return took
}

// EVALUATION.md §4.3, §12.1; DECISIONS 513: lets and a record's two fields nesting a shared value check in linear time.
func TestSharedStoreScale(t *testing.T) {
	for _, record := range []bool{false, true} {
		for _, leaf := range storeLeaves {
			t.Run(fmt.Sprintf("%s/record=%v", leaf.name, record), func(t *testing.T) {
				small := fastestStore(t, leaf, sharedVerifyDepths[0], record)
				for _, d := range sharedVerifyDepths[1:] {
					if took := fastestStore(t, leaf, d, record); took > sharedVerifyGrowth*max(small, time.Millisecond) {
						t.Errorf("depth %d: %v, depth %d: %v, more than %dx", sharedVerifyDepths[0], small, d, took, sharedVerifyGrowth)
					}
				}
			})
		}
	}
}

// fastestStore is the least of sharedVerifyTries checks' times of leaf's case at depth.
func fastestStore(t *testing.T, leaf storeLeaf, depth int, record bool) time.Duration {
	best := storeCheck(t, leaf, depth, record)
	for range sharedVerifyTries - 1 {
		best = min(best, storeCheck(t, leaf, depth, record))
	}
	return best
}

// The where-leaf budget case: w2 holds w1 twice, r holds w2 twice, so r's conversion runs its
// leaf's predicate at 8 places, 3 steps each, after the 9 steps of the three initializers.
const (
	whereStoreSrc = "/// P.\npackage p\n\nlocal let w1: [Int] = [1, 2]\nlocal let w2: [[Int]] = [w1, w1]\n\n" +
		"/// R.\nlet r: [[[Int where it > 0]]] = [w2, w2]\n"
	whereStoreProject = "project acme {\n  canon: \"0.1\"\n  budget: %d\n}\n"
	whereStoreNeed    = 34
	whereStoreStop    = 25
	whereStorePath    = "r[1][0][1]"
)

// whereStoreCheck checks the where-leaf case at budget and is the paths of its budget stops.
func whereStoreCheck(t *testing.T, budget int) []string {
	t.Helper()
	law := map[string]string{"project.canon": fmt.Sprintf(whereStoreProject, budget), srcMain: whereStoreSrc}
	p, err := canon.Open("/law", project(law))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	res, err := p.Check(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range res.Findings {
		if f.Code == string(diag.E4401.Def().Code) {
			out = append(out, f.Path)
		}
	}
	return out
}

// EVALUATION.md §12.1, §12.2; DECISIONS 513: a where leaf runs at each place, so steps and E4401 stay as they were.
func TestSharedStoreWhereSteps(t *testing.T) {
	if got := whereStoreCheck(t, whereStoreNeed); len(got) != 0 {
		t.Errorf("budget %d: stopped at %v, want no stop", whereStoreNeed, got)
	}
	if got := whereStoreCheck(t, whereStoreNeed-1); len(got) != 1 {
		t.Errorf("budget %d: stopped at %v, want one stop", whereStoreNeed-1, got)
	}
	if got := whereStoreCheck(t, whereStoreStop); !slices.Equal(got, []string{whereStorePath}) {
		t.Errorf("budget %d: stopped at %v, want %s", whereStoreStop, got, whereStorePath)
	}
}

// The invalid-element case: xs's element overflows Int8, ys's does not; r holds xs at r[1] and r[10].
const invalidStoreSrc = "/// P.\npackage p\n\nlocal let xs: [Int] = [300]\nlocal let ys: [Int] = [1]\n\n" +
	"/// R.\nlet r: [[Int8]] = [ys, xs, ys, ys, ys, ys, ys, ys, ys, ys, xs]\n"

// EVALUATION.md §4.3, §14; DECISIONS 513: a shared value whose conversion reports is converted at each place.
func TestSharedStoreFindingsPerPlace(t *testing.T) {
	// Both places report; §14 keeps the least path, r[10][0], which a reused visit would not reach.
	p := openShared(t, invalidStoreSrc, false, nil)
	res, err := p.Check(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range res.Findings {
		paths = append(paths, f.Code+" "+f.Path)
	}
	if want := []string{string(diag.E3201.Def().Code) + " r[10][0]"}; !slices.Equal(paths, want) {
		t.Errorf("findings %v, want %v", paths, want)
	}
}
