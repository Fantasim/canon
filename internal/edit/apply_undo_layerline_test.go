package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// API.md W5, W11, W11a, E22: a layer line not a literal refuses its Reset (computed); a request
// beside it verifies; one replacing it, whose Undo could not write it again, is refused before
// anything is written, as computed (log-2026-10-01 M4.1 rulings).
func TestUndoLayerLineNotLiteral(t *testing.T) {
	src := reviewLayerSrc + "\n/// A tag.\nlet tag: String = \"t\"\n"
	dev := "package d\nlayer dev\n\namend quests {\n  slay.goal: kill\n  slay.target: \"boar\"\n  slay.note: tag\n}\n"
	fs := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(src), "law/d/dev.layer.canon": file(dev)}
	collect := edit.Member("collect")
	for _, layers := range [][]string{{"dev"}, nil} {
		v := undoView{layers, "dev"}
		apply := func(ops []edit.Operation) error {
			s := open(t, fs, layers, "dev", "d")
			_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
			return err
		}
		reset := []edit.Operation{{Kind: edit.OpReset, Path: "quests.slay.note"}, setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25))}
		if err := apply(reset); !errors.Is(err, edit.ErrNotEditable) {
			t.Errorf("%v: Reset of the line: %v, want ErrNotEditable", layers, err)
		}
		undoIn(t, undoSpec{name: "beside", fsys: fs, view: v, views: [][]string{{"dev"}, nil}, files: []string{"law/d/dev.layer.canon"},
			ops: []edit.Operation{setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25))}})
		whole := []edit.Operation{setAt("quests.slay", edit.Source(`{ goal: collect, target: 3 }`)), setAt("quests.slay.target", edit.Int(25))}
		var ne *edit.NotEditableError
		if err := apply(whole); !errors.As(err, &ne) || ne.Reason != edit.ReasonComputed {
			t.Errorf("%v: Set of an ancestor: %v, want a NotEditableError, computed", layers, err)
		}
	}
}
