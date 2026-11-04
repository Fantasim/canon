package canon_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// driverSrc is a table whose entries type their rewards by an earlier field, items.
const driverSrc = "/// D.\npackage d\n\n/// A reward: gold, or an item.\ntype Reward(item: Bool) = match item {\n  false => Int(1..)\n  true => String(1..)\n}\n\n" +
	"/// A quest.\nrecord Q {\n  /// Whether its rewards are items.\n  items: Bool = false\n  /// What it gives.\n  rewards: [Reward(items)] = []\n  /// A count.\n  n: Int = 0\n}\n\n" +
	"/// A count.\nlet k0: Int = 4\n\n/// Quests.\nlet qs: table Q = {\n  a { items: true, rewards: [\"s\"] }\n}\n"

// API.md W11, E15, E19 (DECISIONS 462): with the edit layer active, a Set of a driver inside a line
// beside an identifier applies in the edit as in an all-literal line; the re-check judges the
// dependent sibling the driver leaves in error, and the edit is rejected, nothing written.
func TestEditLayerDriverRechecked(t *testing.T) {
	lines := map[string]string{"lz": "a: { n: k0, items: true, rewards: [\"s\"] }", "ly": "a: { items: true, rewards: [\"s\"] }"}
	for _, layer := range []string{"lz", "ly"} {
		opts := project(map[string]string{
			"project.canon": editLaw["project.canon"], "d/d.canon": driverSrc,
			"d/" + layer + ".layer.canon": "package d\nlayer " + layer + "\n\namend qs {\n  " + lines[layer] + "\n}\n",
		})
		opts.Layers, opts.EditLayer = []string{layer}, layer
		p, err := canon.Open("/law", opts)
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("d:qs[a].items", canon.Bool(false))}})
		var re *canon.RejectedError
		switch {
		case !errors.As(err, &re) || res == nil || res.Applied:
			t.Errorf("%s: %v, want the edit rejected by its re-check", layer, err)
		case !slices.ContainsFunc(re.Findings, isCode(diag.E3802.Def().Code)):
			t.Errorf("%s: rejected with %v, want the dependent rewards' mismatch", layer, re.Findings)
		}
		_ = p.Close()
	}
}
