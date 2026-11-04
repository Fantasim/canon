package edit_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// API.md W5, E22 (DECISIONS 273, 462): a refusal of the Undo check names its operation as every
// refusal does, "operation N (path): …": the first that edits the line no Source writes back.
func TestUndoRefusalNamesOperation(t *testing.T) {
	collect := edit.Member("collect")
	spreadAfter := tableLayerFS(layerDev("  hunt: { goal: kill, target: \"x\" }\n  camp: { ...tmpl, note: \"c\" }\n"))
	spreadLine := tableLayerFS(layerDev("  slay: { ...tmpl, target: \"boar\" }\n"))
	cases := []struct {
		fsys  mapFS
		ops   []edit.Operation
		index int
	}{
		{spreadAfter, []edit.Operation{{Kind: edit.OpRemove, Path: "quests.hunt"}}, 0},
		{spreadLine, []edit.Operation{setAt("quests.gather.note", edit.Str("n")), setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25))}, 1},
	}
	for _, c := range cases {
		s := open(t, c.fsys, []string{"dev"}, "dev", "d")
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		var oe *edit.OpError
		var ne *edit.NotEditableError
		prefix := "operation " + strconv.Itoa(c.index) + " (" + c.ops[c.index].Path + "): "
		if !errors.As(err, &oe) || !errors.As(err, &ne) || ne.Reason != edit.ReasonComputed || oe.Index != c.index || !strings.HasPrefix(err.Error(), prefix) {
			t.Errorf("%+v: %v, want a computed refusal starting %q", c.ops, err, prefix)
		}
	}
}
