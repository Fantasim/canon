package canon_test

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// placedProject declares a root in the project, two placed elsewhere on this machine, a missing
// optional one and a missing consumer one, out of name order.
const placedProject = `project acme {
  canon: "0.1"
  roots {
    opt: "../opt"
    data: "data"
    near: "../near"
    both: "../both"
    admin: "../admin"
  }
  optional_roots: [opt]
  consumer_roots: [admin]
}
`

// placedLocal places near and both on this machine (DECISIONS 332).
const placedLocal = "project acme {\n  roots {\n    near: \"/placed/near\"\n    both: \"placed/both\"\n  }\n}\n"

// API.md §5.5, O2, DECISIONS 346, 332, 343: Info's roots are sorted, placed, and Absent when lacking.
func TestInfoRoots(t *testing.T) {
	fsys := newMemFS(map[string][]byte{
		"/law/project.canon": []byte(placedProject), "/law/project.local.canon": []byte(placedLocal),
		"/law/data/x.txt": nil, "/placed/near/x.txt": nil, "/law/placed/both/x.txt": nil, "/over/both/x.txt": nil,
	})
	p, err := canon.Open("/law", canon.Options{FS: fsys, Roots: map[string]string{"both": "/over/both"}})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	vol := strings.TrimSuffix(p.Root(), "/law") // the volume Open put on /law ("" but on Windows, API.md §2.2)
	info, err := p.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []canon.RootInfo{
		{Name: "admin", Dir: vol + "/admin", Absent: true},
		{Name: "both", Dir: vol + "/over/both"},
		{Name: "data", Dir: vol + "/law/data"},
		{Name: "near", Dir: vol + "/placed/near"},
		{Name: "opt", Dir: vol + "/opt", Absent: true},
	}
	if info.Name != "acme" || !reflect.DeepEqual(info.Roots, want) {
		t.Errorf("API.md §5.5, DECISIONS 346: Info roots\n got %+v\nwant %+v", info.Roots, want)
	}
	if err := fsys.WriteFile("/opt/x.txt", nil); err != nil {
		t.Fatal(err)
	}
	info, err = p.Info(context.Background())
	if err != nil || len(info.Roots) != len(want) || info.Roots[4].Absent {
		t.Errorf("API.md O3, DECISIONS 346: a root cloned after Open counts from the next snapshot: %+v, %v", info.Roots, err)
	}
}

// API.md §5.5, DECISIONS 346: a project without roots has none.
func TestInfoNoRoots(t *testing.T) {
	p, _ := openLaw(t, map[string]string{"project.canon": "project plain {\n  canon: \"0.1\"\n}\n"})
	info, err := p.Info(context.Background())
	if err != nil || len(info.Roots) != 0 {
		t.Errorf("API.md §5.5: Info %+v, %v", info, err)
	}
}

// API.md §5.5, DECISIONS 346: a root's Dir is the FS name it is placed at, never resolved through a link.
func TestInfoRootDirKeepsLinks(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "law"), lockLinkDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "real"), lockLinkDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(tmp, "real"), filepath.Join(tmp, "link")); err != nil {
		t.Skipf("no symbolic link here: %v", err)
	}
	text := "project acme {\n  canon: \"0.1\"\n  roots {\n    ext: \"../link\"\n  }\n}\n"
	if err := os.WriteFile(filepath.Join(tmp, "law", "project.canon"), []byte(text), lockLinkFile); err != nil {
		t.Fatal(err)
	}
	p, err := canon.Open(filepath.Join(tmp, "law"), canon.Options{Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	info, err := p.Info(context.Background())
	want := path.Join(path.Dir(filepath.ToSlash(p.Root())), "link")
	if err != nil || len(info.Roots) != 1 || info.Roots[0].Dir != want || info.Roots[0].Absent {
		t.Errorf("DECISIONS 346: Info roots %+v, %v, want ext at %s", info.Roots, err, want)
	}
}
