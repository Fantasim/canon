package canon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The overlay case: a reads b's JSON file through a link and checks its first number; the
// user's buffer for the link holds a number that passes.
const (
	overlayLinkA   = "/// A.\npackage a\n\n/// B's numbers.\nlet ys: [Int] = load(\"data.json\")\n\ncheck ys[0] < 100 else \"too big\"\n"
	overlayLinkBuf = "[5]\n"
	overlayLinkSet = 500
)

// API.md S12, V13 (log-2026-09-29 M4 P14-r4): an overlay covers the real path of every name it
// is read by, so an edit, or a draft, writing b's JSON file is refused while a's link to it holds
// an unsaved buffer, and the checks never see planned content over the user's.
func TestEditRefusesOverlaidLink(t *testing.T) {
	dir := t.TempDir()
	linksWrite(t, dir, map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"a/a.canon":     overlayLinkA,
		"b/b.canon":     "/// B.\npackage b\n\n/// Numbers.\nlet xs: [Int] = load(\"b.json\")\n",
		"b/b.json":      sweepJSON0,
	})
	if err := os.Symlink("../b/b.json", filepath.Join(dir, "a", "data.json")); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	p, err := Open(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	ctx := context.Background()
	if err := p.SetOverlay("a/data.json", []byte(overlayLinkBuf)); err != nil {
		t.Fatal(err)
	}
	if res, err := p.Check(ctx); err != nil || len(res.Findings) != 0 {
		t.Fatalf("Check with the buffer: %v, %+v", err, res)
	}
	set := []Op{Set(readersSet, Int(overlayLinkSet))}
	if _, err := p.Edit(ctx, Edit{Ops: set, AllowErrors: true, Normalize: true}); !errors.Is(err, ErrOverlay) {
		t.Errorf("API.md S12: Edit through an overlaid link: %v, want ErrOverlay", err)
	}
	if _, err := p.Evaluate(ctx, EvalRequest{Path: "b:xs", Draft: set}); !errors.Is(err, ErrOverlay) {
		t.Errorf("API.md V13, S12: a draft through an overlaid link: %v, want ErrOverlay", err)
	}
}
