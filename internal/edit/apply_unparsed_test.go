package edit_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

const (
	unparsedHead = "/// D.\npackage d\n\n/// Settings.\nrecord Config {\n  /// A port.\n  port: Int = 8765\n}\n\n/// The settings.\nlet config: Config = {}\n"
	unparsedBad  = "\n/// Broken.\nlet bad: Int =\n"
)

// API.md M9, E21 (the layout check): a .canon file the plan writes that does not parse is listed
// in Plan.Unparsed with the parser's findings in its package, nothing printed into it and no error
// from Apply; a sibling that does not parse, not written, is not in the way.
func TestUnparsedCanon(t *testing.T) {
	op := edit.Operation{Kind: edit.OpSet, Path: "config.port", Value: edit.Source("9000")}
	for _, c := range []struct {
		name, written, sibling string
		listed                 bool
	}{
		{"written file", unparsedHead + unparsedBad, "", true},
		{"sibling", unparsedHead, "/// D.\npackage d\n" + unparsedBad, false},
	} {
		fsys := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(c.written)}
		if c.sibling != "" {
			fsys["law/d/z.canon"] = file(c.sibling)
		}
		s := open(t, fsys, nil, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !c.listed {
			if len(plan.Unparsed) != 0 {
				t.Errorf("%s: Unparsed = %v, want none", c.name, plan.Unparsed)
			}
			continue
		}
		if len(plan.Unparsed) != 1 {
			t.Fatalf("%s: Unparsed = %v, want one", c.name, plan.Unparsed)
		}
		ue := plan.Unparsed[0]
		if !errors.Is(ue, edit.ErrUnparsed) || ue.File != "d/d.canon" || len(ue.Findings) == 0 || ue.Findings[0].Package != "d" {
			t.Errorf("%s: %+v, want d/d.canon with findings in package d", c.name, ue)
		}
		if len(plan.Changes) != 0 || !slices.Equal(plan.Touched, []string{"d"}) {
			t.Errorf("%s: changes %v touched %v, want the file unwritten, package d touched", c.name, plan.Changes, plan.Touched)
		}
	}
}

// API.md E21: the operations after the one writing a file that does not parse are still judged:
// a refusal of theirs comes before the layout one.
func TestUnparsedLaterOp(t *testing.T) {
	fsys := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(unparsedHead + unparsedBad)}
	s := open(t, fsys, nil, "", "d")
	ops := []edit.Operation{
		{Kind: edit.OpSet, Path: "config.port", Value: edit.Source("9000")},
		{Kind: edit.OpSet, Path: "nope", Value: edit.Source("1")},
	}
	_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
	var oe *edit.OpError
	if !errors.As(err, &oe) || oe.Index != 1 || !errors.Is(err, edit.ErrNoPath) {
		t.Errorf("Apply = %v, want op 1 ErrNoPath", err)
	}
}

// API.md M9: a JSON source that does not parse, repeats a key or is not UTF-8 is the same
// refusal, its finding reported.
func TestUnparsedJSON(t *testing.T) {
	s := open(t, aliasProject(false), nil, "", "d")
	for _, raw := range []string{"{\n  \"x\": 1,\n", "{\"x\": 1, \"x\": 2}\n", "{\"x\": \"\xff\"}\n"} {
		_, err := edit.CanonicalJSON(s.snap, "@a/x.json", []byte(raw))
		var ue *edit.UnparsedError
		if !errors.As(err, &ue) || ue.File != "@a/x.json" || len(ue.Findings) != 1 {
			t.Errorf("%q: %v, want an *UnparsedError with one finding", raw, err)
		}
	}
}
