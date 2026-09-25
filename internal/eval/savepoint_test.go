package eval_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"golang.org/x/tools/txtar"
)

// savepointSrc is a table whose hub's waiting fields hold defaults charging steps and applied
// records, then a map needing every other entry's waiting k: hub first, its attempt is undone.
const savepointSrc = `/// A.
package a

/// K.
enum K { num, col }

/// Rec.
record Rec(e: Loop) {
  /// N, charging steps.
  n: Int = [1, 2, 3].map((i) => i * 2).sum()
}

/// Loop.
record Loop {
  /// Like.
  like: ref loops
  /// W.
  w: Int = 1
  /// L.
  l: LP(like)?
  /// K, reading l.
  k: K = if l == none { num } else { col }
  /// Recs.
  recs: [Rec(like)] = []
  /// M.
  m: {x in loops: LP(x)}? = none
}

/// LP.
type LP(e: Loop) = match e.k {
  num => Int
  col => Int
}

/// Loops.
let loops: table Loop = load("@resource/loops.json")
`

// EVALUATION.md §12: an undone, retried decoding attempt charges steps and binds records once.
func TestSavepointChargesOnce(t *testing.T) {
	const n = 20
	var keys, rest strings.Builder
	for i := range n {
		fmt.Fprintf(&keys, `, "e%d": 1`, i)
		fmt.Fprintf(&rest, `, "e%d": {"like": "hub", "l": 1}`, i)
	}
	hub := `"hub": {"like": "hub", "k": "num", "recs": [{}, {}, {}], "m": {` + keys.String()[2:] + `}}`
	spent := map[string][2]int64{}
	for name, doc := range map[string]string{
		"hub first": `{` + hub + rest.String() + `}`,
		"hub last":  `{` + rest.String()[2:] + `, ` + hub + `}`,
	} {
		a := &txtar.Archive{Files: []txtar.File{{Name: "a/a.canon", Data: []byte(savepointSrc)}, {Name: "resource/loops.json", Data: []byte(doc)}}}
		p := fromArchive(t, a)
		b := runBuildWith(t, p, eval.Options{}, jsonLoader(t, p, a))
		if _, ok := b.values[eval.Root{Pkg: "a", Name: "loops"}]; !ok {
			t.Fatalf("%s: loops poisoned:\n%s", name, b.findings(t))
		}
		spent[name] = [2]int64{b.ev.StepsSpent(), int64(b.ev.BoundCount())}
	}
	if spent["hub first"] != spent["hub last"] {
		t.Errorf("steps and bound records: hub first %v, hub last %v, want equal", spent["hub first"], spent["hub last"])
	}
}
