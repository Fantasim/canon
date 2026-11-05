package canon_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// syntaxOpen opens package d, its main file holding syntaxBad.
func syntaxOpen(t *testing.T) (*canon.Project, *memFS) {
	t.Helper()
	return openEdit(t, map[string]string{"d/d.canon": syntaxHead + syntaxBad})
}

// syntaxLoose is syntaxHead with a space too many: a file not in canonical layout (M9).
var syntaxLoose = strings.Replace(syntaxHead, "port: Int = 8765", "port: Int =  8765", 1)

// syntaxTexts are what d's file holds under an overlay or on disk: text that does not parse, and a
// file not in canonical layout, which the layout check refuses alike.
var syntaxTexts = []struct{ name, text string }{{"broken", syntaxHead + syntaxBad}, {"loose", syntaxLoose}}

// API.md M9: the findings of a file that does not parse are its package's.
func TestEditBrokenFilePackage(t *testing.T) {
	p, _ := syntaxOpen(t)
	_, err := p.Edit(context.Background(), syntaxSet())
	se, ok := isErr[*canon.SyntaxError](err, canon.ErrSyntax)
	if !ok || len(se.Findings) == 0 || se.Findings[0].Package != "d" {
		t.Fatalf("API.md M9: %v", err)
	}
}

// API.md E21, S12: the overlay check comes before the layout one, whatever the overlay holds: a
// file that does not parse gives the refusal a file not in canonical layout gives.
func TestEditBrokenFileOverlay(t *testing.T) {
	for _, c := range syntaxTexts {
		name, text := c.name, c.text
		p, m := openEdit(t, map[string]string{"d/d.canon": syntaxHead})
		before := read(t, m, "d/d.canon")
		if err := p.SetOverlay(context.Background(), "d/d.canon", []byte(text)); err != nil {
			t.Fatal(err)
		}
		_, err := p.Edit(context.Background(), syntaxSet())
		if pe, ok := isErr[*canon.PathError](err, canon.ErrOverlay); !ok || pe.Path != "d/d.canon" {
			t.Errorf("%s: API.md S12: %v", name, err)
		}
		if read(t, m, "d/d.canon") != before {
			t.Errorf("%s: API.md S12: a refused edit wrote", name)
		}
	}
}

// API.md E21, S5: the staleness check comes before the layout one: a base made stale by the file
// breaking on disk is a *StaleError, as one made stale by the file losing its layout is.
func TestEditBrokenFileStale(t *testing.T) {
	for _, c := range syntaxTexts {
		name, text := c.name, c.text
		p, m := openEdit(t, map[string]string{"d/d.canon": syntaxHead})
		e := syntaxSet()
		e.Base = p.Revision()
		if err := m.WriteFile("/law/d/d.canon", []byte(text)); err != nil {
			t.Fatal(err)
		}
		_, err := p.Edit(context.Background(), e)
		if se, ok := isErr[*canon.StaleError](err, canon.ErrStale); !ok || len(se.Files) == 0 {
			t.Errorf("%s: API.md S5: %T %v", name, err, err)
		}
	}
}

// API.md E21: each op is judged in turn before the layout check: a later op's own refusal comes
// before the syntax error of the file an earlier one writes.
func TestEditBrokenFileLaterOp(t *testing.T) {
	p, m := syntaxOpen(t)
	before := read(t, m, "d/d.canon")
	set := canon.Set("d:config.port", canon.Int(9000))
	for _, c := range []struct {
		name string
		op   canon.Op
		want error
	}{
		{"no path", canon.Set("d:nope", canon.Int(1)), canon.ErrNoPath},
		{"bad value", canon.Set("d:config.port", canon.Str("x")), canon.ErrBadValue},
	} {
		_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{set, c.op}})
		var pe *canon.PathError
		var ve *canon.ValueError
		op := -2
		switch {
		case errors.As(err, &pe):
			op = pe.Op
		case errors.As(err, &ve):
			op = ve.Op
		}
		if !errors.Is(err, c.want) || op != 1 {
			t.Errorf("%s: API.md E21: %T %v, want op 1 %v", c.name, err, err, c.want)
		}
	}
	if read(t, m, "d/d.canon") != before {
		t.Error("API.md E21: a refused edit wrote")
	}
}

// API.md V13, M9: a draft the file it writes cannot take is the same refusal.
func TestEvaluateDraftBrokenFile(t *testing.T) {
	p, _ := syntaxOpen(t)
	_, err := p.Evaluate(context.Background(), canon.EvalRequest{Path: "d:config", Draft: syntaxSet().Ops})
	if se, ok := isErr[*canon.SyntaxError](err, canon.ErrSyntax); !ok || len(se.Findings) == 0 || se.Findings[0].File != "d/d.canon" {
		t.Fatalf("API.md V13: %T %v", err, err)
	}
}

// API.md E32, M9: a RenameName whose packages hold a file that does not parse, the target's own
// or an importer's, keeps E32's refusal, `broken`.
func TestRenameNameBrokenFile(t *testing.T) {
	for _, c := range []struct {
		name  string
		files map[string]string
	}{
		{"sibling", map[string]string{"d/d.canon": syntaxHead, "d/z.canon": "/// D.\npackage d\n" + syntaxBad}},
		{"same", map[string]string{"d/d.canon": syntaxHead + syntaxBad}},
		{"importer", map[string]string{"d/d.canon": syntaxHead, "e/e.canon": "/// E.\npackage e\n\nimport d\n" + syntaxBad}},
	} {
		name := c.name
		p, _ := openEdit(t, c.files)
		_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.RenameName("d:Config.port", "door")}})
		if ne, ok := isErr[*canon.NotEditableError](err, canon.ErrNotEditable); !ok || ne.Reason != canon.ReasonBroken {
			t.Errorf("%s: API.md E32: %T %v", name, err, err)
		}
	}
}
