package canon_test

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// The shared-verification scale case: the layer amending its leaf, the builds timed per depth,
// and the growth in check time allowed from the smallest depth to the largest. Each level doubles
// the paths to the leaf, so a walk that re-visits a shared value grows by 2^10 over the depths.
const (
	sharedVerifyLayer  = "deep"
	sharedVerifyTries  = 3
	sharedVerifyGrowth = 16
)

// sharedVerifyDepths are the depths the scale case builds, smallest first.
var sharedVerifyDepths = []int{10, 15, 20}

// sharedVerifySrc is package p: a chain of lists, each level holding the level below twice, over
// a leaf, an Int when rec is false, a record a layer amends when it is true; n reads the leaf.
func sharedVerifySrc(depth int, rec bool) string {
	var src strings.Builder
	src.WriteString("/// P.\npackage p\n\n")
	elem, leaf := "Int", "local let w0: [Int] = [1]\n"
	if rec {
		elem = "Leaf"
		src.WriteString("/// Leaf.\nrecord Leaf {\n  /// N.\n  n: Int = 1\n}\n\n/// Leaf.\nlet leaf: Leaf = {}\n\n")
		leaf = "local let w0: [Leaf] = [leaf]\n"
	}
	src.WriteString(leaf)
	typ := "[" + elem + "]"
	for i := 1; i <= depth; i++ {
		typ = "[" + typ + "]"
		fmt.Fprintf(&src, "local let w%d: %s = [w%d, w%d]\n", i, typ, i-1, i-1)
	}
	read := fmt.Sprintf("w%d", depth) + strings.Repeat("[1]", depth) + "[0]"
	if rec {
		read += ".n"
	}
	fmt.Fprintf(&src, "\n/// N.\nlet n: Int = %s\n", read)
	return src.String()
}

// sharedTreeSrc is package p: a chain of records T, each level's a and b holding the level below,
// in a list when list is true; n reads the leaf through b.
func sharedTreeSrc(depth int, list bool) string {
	var src strings.Builder
	part, none, down, wrap, last := "T?", "none", "?.b", "%s", "?.n"
	if list {
		part, none, down, wrap, last = "[T]", "[]", ".b[0]", "[%s]", ".n"
	}
	fmt.Fprintf(&src, "/// P.\npackage p\n\n/// T.\nrecord T {\n  /// A.\n  a: %s = %s\n  /// B.\n  b: %s = %s\n  /// N.\n  n: Int = 1\n}\n\n", part, none, part, none)
	src.WriteString("local let w0: T = {}\n")
	for i := 1; i <= depth; i++ {
		below := fmt.Sprintf(wrap, fmt.Sprintf("w%d", i-1))
		fmt.Fprintf(&src, "local let w%d: T = { a: %s, b: %s }\n", i, below, below)
	}
	fmt.Fprintf(&src, "\n/// N.\nlet n: Int? = w%d%s%s\n", depth, strings.Repeat(down, depth), last)
	return src.String()
}

// sharedVerifyCase is a variant of the scale case: its source at a depth, whether it has the leaf
// a layer amends, the layers active and n's value.
type sharedVerifyCase struct {
	name   string
	src    func(int) string
	rec    bool
	layers []string
	want   string
}

// sharedVerifyCases are the scale case's variants.
var sharedVerifyCases = []sharedVerifyCase{
	{"int", func(d int) string { return sharedVerifySrc(d, false) }, false, nil, "1"},
	{"record", func(d int) string { return sharedVerifySrc(d, true) }, true, nil, "1"},
	{"record/layer", func(d int) string { return sharedVerifySrc(d, true) }, true, []string{sharedVerifyLayer}, "2"},
	{"tree/field", func(d int) string { return sharedTreeSrc(d, false) }, false, nil, "1"},
	{"tree/list", func(d int) string { return sharedTreeSrc(d, true) }, false, nil, "1"},
}

// openShared opens src as package p, with a layer amending leaf when rec, active under layers.
func openShared(t testing.TB, src string, rec bool, layers []string) *canon.Project {
	t.Helper()
	law := maps.Clone(editLaw)
	law[srcMain] = src
	if rec {
		law["p/"+sharedVerifyLayer+".layer.canon"] = "package p\nlayer " + sharedVerifyLayer + "\n\namend leaf {\n  n: 2\n}\n"
	}
	opts := project(law)
	opts.Layers = layers
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// sharedCheck checks c at depth and is the time it took, checking n's value.
func sharedCheck(t testing.TB, c sharedVerifyCase, depth int) time.Duration {
	t.Helper()
	p := openShared(t, c.src(depth), c.rec, c.layers)
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
	v, err := p.Value(context.Background(), "p:n")
	if err != nil || v.Text != c.want {
		t.Fatalf("depth %d, layers %v: n = %v (%v), want %s", depth, c.layers, v, err, c.want)
	}
	return took
}

// fastestCheck is the least of sharedVerifyTries checks' times, the one a loaded machine disturbs least.
func fastestCheck(t *testing.T, c sharedVerifyCase, depth int) time.Duration {
	best := sharedCheck(t, c, depth)
	for range sharedVerifyTries - 1 {
		best = min(best, sharedCheck(t, c, depth))
	}
	return best
}

// EVALUATION.md §5, §12.1; DECISIONS 507, 509: shared list chains and record trees check in linear time.
func TestSharedVerifyScale(t *testing.T) {
	for _, c := range sharedVerifyCases {
		t.Run(c.name, func(t *testing.T) {
			small := fastestCheck(t, c, sharedVerifyDepths[0])
			for _, d := range sharedVerifyDepths[1:] {
				if took := fastestCheck(t, c, d); took > sharedVerifyGrowth*max(small, time.Millisecond) {
					t.Errorf("depth %d: %v, depth %d: %v, more than %dx", sharedVerifyDepths[0], small, d, took, sharedVerifyGrowth)
				}
			}
		})
	}
}

// BenchmarkSharedVerifyScale times each variant's check at each depth.
func BenchmarkSharedVerifyScale(b *testing.B) {
	for _, c := range sharedVerifyCases {
		for _, d := range sharedVerifyDepths {
			b.Run(fmt.Sprintf("%s/%d", c.name, d), func(b *testing.B) {
				for b.Loop() {
					sharedCheck(b, c, d)
				}
			})
		}
	}
}
