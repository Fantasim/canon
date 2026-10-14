package build

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// TYPES.md §13.4, DECISIONS 376: an outside link in an asset root is no stale listing and is still reported.
func TestBuildOutsideAssetLinkNotStale(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevated privilege on windows")
	}
	base := t.TempDir()
	dir := filepath.Join(base, "proj")
	touchTree(t, dir)
	mustWrite(t, filepath.Join(base, "x.png"), "x")
	mustSymlink(t, filepath.Join(base, "x.png"), filepath.Join(dir, "a", "icons", "x.png"))
	src := strings.Replace(touchedSource(0), `[{ icon: "a.png" }]`, `[{ icon: "a.png" }, { icon: "x.png" }]`, 1)
	mustWrite(t, filepath.Join(dir, "a", "a.canon"), src)
	p, err := Open(OS(), filepath.ToSlash(dir), Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, f := range res.Findings.List {
		if f.Code == diag.E3706.Def().Code {
			return
		}
	}
	t.Errorf("no finding of the outside link among %d", len(res.Findings.List))
}
