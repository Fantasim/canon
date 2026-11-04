package canon_test

import (
	"context"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// layerLineSrc is a String-keyed list whose keys no code reads.
const layerLineSrc = "/// P.\npackage p\n\n/// An S.\nrecord S {\n  /// Id.\n  id: String\n  /// W.\n  w: Int = 0\n}\n\n/// Ss.\nlet ss: [S] keyed by id = [{ id: \"x\" }, { id: \"y\" }]\n"

// API.md E11, W5 (DECISIONS 305, 310): a Rename whose new key no amend line can start with is
// ErrNotEditable `layer`; its text names the layer, the file and the line to rewrite or remove first,
// with dev active or not, and nothing is written.
func TestEditRenameNamesLayerLine(t *testing.T) {
	layer := "package p\nlayer dev\n\namend ss {\n  y.w: 1\n  x.w: 3\n}\n"
	for _, active := range []bool{false, true} {
		opts := project(srcLaw(layerLineSrc, "project.canon", editLaw["project.canon"], "p/dev.layer.canon", layer))
		if active {
			opts.Layers = []string{"dev"}
		}
		p, err := canon.Open("/law", opts)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = p.Close() })
		m, _ := opts.FS.(*memFS)
		_, err = p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename("p:ss[x]", canon.Key("z z"))}})
		var ne *canon.NotEditableError
		want := "op 0: p:ss[x]: value is not editable: layer: dev, p/dev.layer.canon, p:ss.x.w"
		if !errors.As(err, &ne) || ne.Reason != canon.ReasonLayer || err.Error() != want {
			t.Errorf("dev active %v: %v, want %q", active, err, want)
		}
		if read(t, m, "p/dev.layer.canon") != layer || read(t, m, srcMain) != layerLineSrc {
			t.Errorf("dev active %v: a refused Rename wrote a file", active)
		}
	}
}
