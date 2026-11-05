package verify_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// producedSrc's dev layer flips items, so verification retypes q's rewards and rebuilds q (TYPES.md §11.6).
const producedSrc = `/// D.
package d

/// A reward: gold, or an item.
type Reward(item: Bool) = match item {
  false => Int(1..)
  true => String(1..)
}

/// A quest.
record Q {
  /// Whether its rewards are items.
  items: Bool = false
  /// What it gives.
  rewards: [Reward(items)] = []
}

/// The quest.
let q: Q = { items: false }
`

const producedDev = "package d\nlayer dev\n\namend q {\n  items: true\n}\n"

// CLI.md §3.7, EVALUATION.md §9.3 step 4: Produced is the base root, though verification rebuilt the amended one.
func TestProducedSurvivesVerificationCopies(t *testing.T) {
	dir := writeArchive(t, &txtar.Archive{Files: []txtar.File{
		{Name: projectFile, Data: []byte(projectOpen + "}\n")},
		{Name: "d/d.canon", Data: []byte(producedSrc)},
		{Name: "d/dev.layer.canon", Data: []byte(producedDev)},
	}})
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{Layers: []string{"dev"}})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"d"})
	if err != nil {
		t.Fatal(err)
	}
	if n := a.Result().Summary.Errors; n != 0 {
		t.Fatalf("%d errors", n)
	}
	q, ok := a.Force(eval.Root{Pkg: "d", Name: "q"})
	if !ok {
		t.Fatal("q did not force")
	}
	rec := q.(*value.Record)
	for _, c := range []struct {
		v    value.Value
		want string
	}{
		{q, "Q{items: false, rewards: []}"},
		{rec.Fields[0], "true"},
		{rec.Fields[1], "[]"},
	} {
		if got := a.Produced(c.v).CanonText(); got != c.want {
			t.Errorf("Produced(%s) = %s, want %s", c.v.CanonText(), got, c.want)
		}
	}
}
