package canon_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// depSrc has a dependent list and field whose driver, items, the dev layer turns from false (an
// Int reward) to true (a String one), at a let and in a table entry.
const depSrc = `/// D.
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
  /// Its best reward.
  best: Reward(items)?
}

/// The quest.
let q: Q = { items: false }

/// The quests.
let qs: table Q = {
  a { items: false }
}
`

const (
	depDev  = "package d\nlayer dev\n\namend q {\n  items: true\n}\n\namend qs {\n  a.items: true\n}\n"
	depBase = "/law/d/d.canon"
)

// depOpen opens depSrc, with the dev layer active when layered (with no layer file otherwise).
func depOpen(t *testing.T, layered bool) (*canon.Project, *memFS) {
	t.Helper()
	files := map[string]string{"project.canon": editLaw["project.canon"], "d/d.canon": depSrc}
	opts := project(files)
	if layered {
		files["d/dev.layer.canon"] = depDev
		opts = project(files)
		opts.Layers = []string{"dev"}
	}
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, opts.FS.(*memFS)
}

// API.md V1, E18, E19, W8 (EDIT-MATERIALIZE ruling): typed in the base's frame, an Int reward fits
// and dev's String arm makes the re-check refuse it (E3802); a String one is a *ValueError, under
// dev or no layer, at the let and in a table entry alike.
func TestEditDependentTypedInBase(t *testing.T) {
	for _, path := range []string{"d:q.rewards", "d:qs.a.rewards"} {
		p, fsys := depOpen(t, true)
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Add(path, canon.Int(2))}})
		var re *canon.RejectedError
		got, _ := fsys.ReadFile(depBase)
		if !errors.As(err, &re) || res.Applied || !slices.ContainsFunc(re.Findings, isCode(diag.E3802.Def().Code)) || string(got) != depSrc {
			t.Errorf("Add %s 2 under dev: %v, want it rejected with %s, nothing written", path, err, diag.E3802.Def().Code)
		}
		_, err = p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Add(path, canon.Str("t"))}})
		if !errors.Is(err, canon.ErrBadValue) {
			t.Errorf("Add %s \"t\" under dev: %v, want ErrBadValue against the base's Int arm", path, err)
		}
		p0, _ := depOpen(t, false)
		if _, err := p0.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Add(path, canon.Int(2))}}); err != nil {
			t.Errorf("Add %s 2 with no layer: %v, want it applied", path, err)
		}
		if _, err := p0.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Add(path, canon.Str("t"))}}); !errors.Is(err, canon.ErrBadValue) {
			t.Errorf("Add %s \"t\" with no layer: %v, want ErrBadValue", path, err)
		}
	}
}
