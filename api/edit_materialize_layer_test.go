package canon_test

import (
	"context"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// materializeSrc is the EVAL-AMENDNOTE repro: url's default reads the port a dev layer amends.
const materializeSrc = `/// A.
package a

/// Server.
record Server {
  /// Port.
  port: Int = 1
  /// Host.
  host: String = "localhost"
  /// URL.
  url: String = "http://{host}:{port}"
}

/// Server.
let server: Server = {}
`

const (
	materializeDev  = "package a\nlayer dev\n\namend server {\n  port: 9000\n}\n"
	materializeBase = "/law/a/a.canon"
	materializeLine = "/law/a/dev.layer.canon"
)

// API.md W8, W10, E22 (EDIT-MATERIALIZE ruling): a base Set while dev is active writes the base's
// values only, never dev's port, the re-check and the commit included; its Undo, applied at the
// result's revision, brings a/a.canon back byte for byte, and dev's file is never written.
func TestEditBaseUnderLayerWritesNoLayerValue(t *testing.T) {
	for _, field := range []string{"url", "host"} {
		opts := project(map[string]string{"project.canon": editLaw["project.canon"], "a/a.canon": materializeSrc, "a/dev.layer.canon": materializeDev})
		opts.Layers = []string{"dev"}
		fsys := opts.FS.(*memFS)
		p, err := canon.Open("/law", opts)
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("a:server."+field, canon.Str("x"))}})
		want := strings.Replace(materializeSrc, "let server: Server = {}", `let server: Server = { `+field+`: "x" }`, 1)
		if got, _ := fsys.ReadFile(materializeBase); err != nil || len(res.Changes) != 1 || string(got) != want {
			t.Errorf("Set %s with dev active: %v, %+v; a/a.canon reads\n%s\nwant it alone written, as\n%s", field, err, res, got, want)
			_ = p.Close()
			continue
		}
		undo, err := p.Edit(context.Background(), canon.Edit{Base: res.Revision, Ops: res.Undo})
		got, _ := fsys.ReadFile(materializeBase)
		if dev, _ := fsys.ReadFile(materializeLine); err != nil || string(got) != materializeSrc || string(dev) != materializeDev {
			t.Errorf("Undo of Set %s with dev active: %v, %+v; a/a.canon reads\n%s\nwant it back byte for byte", field, err, undo, got)
		}
		_ = p.Close()
	}
}
