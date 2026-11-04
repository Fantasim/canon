package canon_test

import (
	"context"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// layerLineSrc is a String-keyed list whose keys no code reads.
const layerLineSrc = "/// P.\npackage p\n\n/// An S.\nrecord S {\n  /// Id.\n  id: String\n  /// W.\n  w: Int = 0\n}\n\n/// Ss.\nlet ss: [S] keyed by id = [{ id: \"x\" }, { id: \"y\" }]\n"

// layerLineOpen opens layerLineSrc with layer files (path, text pairs), dev active or not.
func layerLineOpen(t *testing.T, active bool, files ...string) (*canon.Project, *memFS) {
	t.Helper()
	opts := project(srcLaw(layerLineSrc, append([]string{"project.canon", editLaw["project.canon"]}, files...)...))
	if active {
		opts.Layers = []string{"dev"}
	}
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	m, _ := opts.FS.(*memFS)
	return p, m
}

// API.md E11, W5 (DECISIONS 305, 310): a Rename whose new key no amend line can start with is
// ErrNotEditable `layer`; its text names the layer, the file and the line, and the way out, to remove
// that line first, when it is the one line starting with the key; dev active or not, nothing written.
func TestEditRenameNamesLayerLine(t *testing.T) {
	layer := "package p\nlayer dev\n\namend ss {\n  y.w: 1\n  x.w: 3\n}\n"
	qa := "package p\nlayer qa\n\namend ss {\n  x.w: 5\n}\n"
	cases := []struct {
		files []string
		want  string
	}{
		{[]string{"p/dev.layer.canon", layer}, "op 0: p:ss[x]: value is not editable: layer: dev, p/dev.layer.canon, p:ss.x.w, remove that line first"},
		{[]string{"p/dev.layer.canon", layer, "p/qa.layer.canon", qa}, "op 0: p:ss[x]: value is not editable: layer: dev, p/dev.layer.canon, p:ss.x.w"},
	}
	for _, c := range cases {
		for _, active := range []bool{false, true} {
			p, m := layerLineOpen(t, active, c.files...)
			_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename("p:ss[x]", canon.Key("z z"))}})
			var ne *canon.NotEditableError
			if !errors.As(err, &ne) || ne.Reason != canon.ReasonLayer || err.Error() != c.want {
				t.Errorf("dev active %v: %v, want %q", active, err, c.want)
			}
			if read(t, m, "p/dev.layer.canon") != layer || read(t, m, srcMain) != layerLineSrc {
				t.Errorf("dev active %v: a refused Rename wrote a file", active)
			}
		}
	}
}

// API.md E11 (DECISIONS 305, 462): the way out followed, the line
// that started with the key removed, the same Rename passes, dev active or not.
func TestEditRenameLayerLineWayFollowed(t *testing.T) {
	layer := "package p\nlayer dev\n\namend ss {\n  y.w: 1\n}\n"
	for _, active := range []bool{false, true} {
		p, _ := layerLineOpen(t, active, "p/dev.layer.canon", layer)
		if _, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename("p:ss[x]", canon.Key("z z"))}}); err != nil {
			t.Errorf("dev active %v: %v, want the Rename to pass", active, err)
		}
	}
}
