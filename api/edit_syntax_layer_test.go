package canon_test

import (
	"context"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// syntaxLayerBad is layer dev of layerLineSrc cut short: a file that does not parse.
const syntaxLayerBad = "package p\nlayer dev\n\namend ss {\n  y.w: 1\n"

// syntaxLayerOpen opens layerLineSrc with dev's layer file broken, editLayer the edit layer.
func syntaxLayerOpen(t *testing.T, editLayer string) (*canon.Project, *memFS) {
	t.Helper()
	opts := project(srcLaw(layerLineSrc, "project.canon", editLaw["project.canon"], "p/dev.layer.canon", syntaxLayerBad))
	opts.EditLayer = editLayer
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	m, _ := opts.FS.(*memFS)
	return p, m
}

// API.md W11, E21, M9: an edit that writes its edit layer's file, inactive, when that file does
// not parse is the refusal of any file that does not parse, naming it; nothing is written.
func TestEditBrokenEditLayerFile(t *testing.T) {
	p, m := syntaxLayerOpen(t, "dev")
	_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("p:ss[x].w", canon.Int(5))}})
	se, ok := isErr[*canon.SyntaxError](err, canon.ErrSyntax)
	if !ok || len(se.Findings) == 0 || se.Findings[0].File != "p/dev.layer.canon" || se.Findings[0].Package != "p" {
		t.Fatalf("API.md M9: %T %v", err, err)
	}
	if read(t, m, "p/dev.layer.canon") != syntaxLayerBad {
		t.Error("API.md M9: a refused edit wrote")
	}
}

// API.md E11, E18, E19: an edit that does not write the broken layer file, a Rename of a key the
// layer would amend or a Set under another edit layer, is judged by its re-check: the layer file's
// own finding refuses it, never an internal error.
func TestEditBesideBrokenLayerFile(t *testing.T) {
	for _, c := range []struct {
		name, layer string
		op          canon.Op
	}{
		{"rename", "", canon.Rename("p:ss[x]", canon.Key("z"))},
		{"other layer", "qa", canon.Set("p:ss[x].w", canon.Int(5))},
		{"base set", "", canon.Set("p:ss[x].w", canon.Int(5))},
	} {
		name := c.name
		p, m := syntaxLayerOpen(t, c.layer)
		_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}})
		re, ok := isErr[*canon.RejectedError](err, canon.ErrRejected)
		if !ok || len(re.Findings) != 1 || re.Findings[0].File != "p/dev.layer.canon" {
			t.Errorf("%s: API.md E19: %T %v", name, err, err)
		}
		if read(t, m, "p/dev.layer.canon") != syntaxLayerBad {
			t.Errorf("%s: API.md E19: a refused edit wrote", name)
		}
	}
}
