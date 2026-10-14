package build

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

const (
	assetLinkProject = "proj"
	linkOutside      = "outside"
	linkOther        = "other"
	linkFrom         = "items"
	linkPNG          = "f.png"
	linkNested       = "dir/x.png"
	linkRootName     = "@assets"
)

// assetTree is a project directory holding the root assets (".") beside the directories outside and other.
type assetTree struct {
	t                   *testing.T
	base, proj, outside string
}

func newAssetTree(t *testing.T) assetTree {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevated privilege on windows")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lt := assetTree{t: t, base: base, proj: filepath.Join(base, assetLinkProject), outside: filepath.Join(base, linkOutside)}
	mustMkdir(t, lt.proj)
	mustMkdir(t, filepath.Join(lt.outside, "dir"))
	mustWrite(t, filepath.Join(lt.outside, linkPNG), "x")
	mustWrite(t, filepath.Join(lt.outside, linkNested), "x")
	mustMkdir(t, filepath.Join(base, linkOther))
	mustWrite(t, filepath.Join(base, linkOther, linkPNG), "x")
	return lt
}

// assets is the asset lookup of a layout whose roots are assets (the project directory) and other.
func (lt assetTree) assets() *assets {
	lt.t.Helper()
	p := &project.Project{Roots: []project.Root{
		{Name: "assets", Path: "."},
		{Name: linkOther, Path: "../" + linkOther},
	}}
	layout, ok := project.NewLayout(p, filepath.ToSlash(lt.proj), nil, diag.NewBag(nil, ""))
	if !ok {
		lt.t.Fatal("layout")
	}
	return &assets{fs: project.OS(), layout: layout, host: &evalHost{}, dirs: map[string]dirListing{}}
}

// TYPES.md §13.4, DECISIONS 376: a file, a folder and a root reached through an outside link name it.
func TestAssetLinkOutsideRoots(t *testing.T) {
	lt := newAssetTree(t)
	mustSymlink(t, filepath.Join(lt.outside, linkPNG), filepath.Join(lt.proj, linkPNG))
	mustSymlink(t, filepath.Join(lt.outside, "dir"), filepath.Join(lt.proj, "dir"))
	mustSymlink(t, lt.outside, filepath.Join(lt.proj, "Icon"))
	for _, c := range []struct{ name, root, path, link string }{
		{"file link", linkRootName, linkPNG, linkRootName + "/" + linkPNG},
		{"folder link", linkRootName, linkNested, linkRootName + "/dir"},
		{"folder link, target without the file", linkRootName, "dir/missing.png", linkRootName + "/dir"},
		{"root reached through a link", linkRootName + "/Icon", linkPNG, linkRootName + "/Icon"},
	} {
		_, found, link := lt.assets().Exists(c.root, linkFrom, c.path)
		if found || link != c.link {
			t.Errorf("%s: Exists(%q, %q) = found %v, link %q; want false, %q", c.name, c.root, c.path, found, link, c.link)
		}
	}
}

// WIRE.md §10, TYPES.md §13.4, DECISIONS 376: the manifest's listing leaves out an outside link.
func TestAssetListingOmitsOutsideLink(t *testing.T) {
	lt := newAssetTree(t)
	mustWrite(t, filepath.Join(lt.proj, "keep.png"), "x")
	mustSymlink(t, filepath.Join(lt.outside, linkPNG), filepath.Join(lt.proj, linkPNG))
	a := lt.assets()
	a.Exists(linkRootName, linkFrom, "keep.png")
	var names []string
	for _, l := range listings([]*evalHost{{assets: a}}) {
		names = append(names, listedNames(l.names)...)
	}
	if !slices.Contains(names, "keep.png") || slices.Contains(names, linkPNG) {
		t.Errorf("listed %v, want keep.png and not %s", names, linkPNG)
	}
}

// TYPES.md §13.4, DECISIONS 376: a link into another declared root is followed.
func TestAssetLinkIntoAnotherRoot(t *testing.T) {
	lt := newAssetTree(t)
	mustSymlink(t, filepath.Join(lt.base, linkOther, linkPNG), filepath.Join(lt.proj, linkPNG))
	if _, found, link := lt.assets().Exists(linkRootName, linkFrom, linkPNG); !found || link != "" {
		t.Errorf("Exists = found %v, link %q; want true, none", found, link)
	}
}

// TYPES.md §13.4, DECISIONS 376: a dangling link, a loop and a looping root hold nothing (E3701).
func TestAssetLinkDanglingLooping(t *testing.T) {
	lt := newAssetTree(t)
	mustSymlink(t, filepath.Join(lt.proj, "gone.png"), filepath.Join(lt.proj, "dangling.png"))
	mustSymlink(t, filepath.Join(lt.proj, "b.png"), filepath.Join(lt.proj, "a.png"))
	mustSymlink(t, filepath.Join(lt.proj, "a.png"), filepath.Join(lt.proj, "b.png"))
	mustSymlink(t, filepath.Join(lt.proj, "loop"), filepath.Join(lt.proj, "loop"))
	for _, c := range []struct{ name, root, path string }{
		{"dangling", linkRootName, "dangling.png"},
		{"loop", linkRootName, "a.png"},
		{"looping root", linkRootName + "/loop", linkPNG},
	} {
		if _, found, link := lt.assets().Exists(c.root, linkFrom, c.path); found || link != "" {
			t.Errorf("%s: Exists = found %v, link %q; want false, none", c.name, found, link)
		}
	}
}
