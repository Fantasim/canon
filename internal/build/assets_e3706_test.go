package build_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

const e3706Project = `project demo {
  canon: "0.1"
  roots {
    assets: "assets"
  }
}
`

const e3706Source = `/// Items.
package items

/// Item.
record Item {
  /// Icon.
  icon: asset("@assets", ext: [png])
}

/// Items.
let items: stable table Item = {
  sword { icon: "sword.png" }
}
`

// TYPES.md §13.4, DECISIONS 376: canon check reports E3706 for an asset reached through an outside link.
func TestAssetE3706Check(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevated privilege on windows")
	}
	base := t.TempDir()
	proj := filepath.Join(base, "proj")
	outside := filepath.Join(base, "outside", "sword.png")
	files := []struct{ name, data string }{
		{filepath.Join(proj, "project.canon"), e3706Project},
		{filepath.Join(proj, "items", "items.canon"), e3706Source},
		{filepath.Join(proj, "assets", ".keep"), ""},
		{outside, "x"},
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f.name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f.name, []byte(f.data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(proj, "assets", "sword.png")); err != nil {
		t.Fatal(err)
	}
	p, err := build.Open(build.OS(), filepath.ToSlash(proj), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Findings.List {
		if f.Code == diag.E3706.Def().Code {
			return
		}
	}
	t.Errorf("no finding of the outside link among %d", len(res.Findings.List))
}
